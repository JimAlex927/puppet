package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"puppet/internal/agent"
	"puppet/internal/agentproto"
	"puppet/internal/config"
	"puppet/internal/credential"
	"puppet/internal/logstream"
	"puppet/internal/model"
	"puppet/internal/node"

	"gorm.io/gorm"
)

type Engine struct {
	db       *gorm.DB
	registry *node.Registry
	hub      *logstream.Hub
	cfg      config.Config
	creds    *credential.Service
	agents   *agent.Service

	logMu sync.Mutex
	next  map[uint]int
}

func New(db *gorm.DB, registry *node.Registry, hub *logstream.Hub, cfg config.Config) *Engine {
	return &Engine{
		db:       db,
		registry: registry,
		hub:      hub,
		cfg:      cfg,
		creds:    credential.NewService(db),
		agents:   agent.NewService(db),
		next:     map[uint]int{},
	}
}

func (e *Engine) StartTask(ctx context.Context, taskID uint, triggerType string, triggeredBy string, input map[string]any) (model.TaskRun, error) {
	var task model.Task
	var run model.TaskRun
	var pipeline node.PipelineDefinition

	inputJSON, _ := json.Marshal(input)
	err := e.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&task, taskID).Error; err != nil {
			return err
		}
		if !task.AllowConcurrent {
			var count int64
			if err := tx.Model(&model.TaskRun{}).
				Where("task_id = ? AND status IN ?", task.ID, []string{model.TaskRunPending, model.TaskRunRunning}).
				Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return fmt.Errorf("task already has a running task run")
			}
		}
		if err := json.Unmarshal([]byte(task.PipelineJSON), &pipeline); err != nil {
			return err
		}
		run = model.TaskRun{
			ProjectID:            task.ProjectID,
			TaskID:               task.ID,
			Status:               model.TaskRunPending,
			TriggerType:          triggerType,
			TriggeredBy:          triggeredBy,
			InputJSON:            string(inputJSON),
			PipelineSnapshotJSON: task.PipelineJSON,
		}
		return tx.Create(&run).Error
	})
	if err != nil {
		return model.TaskRun{}, err
	}

	go e.execute(context.Background(), task, run, pipeline)
	return run, nil
}

func (e *Engine) execute(parent context.Context, task model.Task, run model.TaskRun, pipeline node.PipelineDefinition) {
	ctx := parent
	cancel := func() {}
	if task.TimeoutSeconds > 0 {
		ctx, cancel = context.WithTimeout(parent, time.Duration(task.TimeoutSeconds)*time.Second)
	}
	defer cancel()

	e.setAgentStatus("running")
	defer e.setAgentStatus("online")

	startedAt := time.Now()
	run.Status = model.TaskRunRunning
	run.StartedAt = &startedAt
	if err := e.db.Save(&run).Error; err != nil {
		log.Printf("engine: save run error: %v", err)
	}
	e.publishTaskStatus(run)
	e.appendLog(run.ID, 0, "system", fmt.Sprintf("[task:start] task run #%d\n", run.ID))

	workspace := filepath.Join(e.cfg.WorkspaceDir, fmt.Sprintf("taskrun-%d", run.ID))
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		e.finishTask(&run, model.TaskRunFailed, startedAt, err.Error())
		return
	}

	var taskErr error
	runInput := map[string]any{}
	_ = json.Unmarshal([]byte(run.InputJSON), &runInput)
	nodeByID, nodeIndex, usesEdges, err := indexPipeline(pipeline)
	if err != nil {
		e.finishTask(&run, model.TaskRunFailed, startedAt, err.Error())
		return
	}
	currentID := pipeline.StartNodeID
	if currentID == "" && len(pipeline.Nodes) > 0 {
		currentID = pipeline.Nodes[0].ID
	}
	visited := map[string]bool{}
	nodeOutputs := map[string]map[string]any{}
	executionIndex := 0

	for currentID != "" {
		select {
		case <-ctx.Done():
			taskErr = ctx.Err()
		default:
		}
		if taskErr != nil {
			break
		}

		if visited[currentID] {
			taskErr = fmt.Errorf("pipeline cycle detected at node %q", currentID)
			break
		}
		visited[currentID] = true

		pipelineNode, ok := nodeByID[currentID]
		if !ok {
			taskErr = fmt.Errorf("node %q not found", currentID)
			break
		}
		renderedNode := renderPipelineNode(pipelineNode, runInput, nodeOutputs)

		selectedAgent, err := e.selectAgent(pipeline.AgentSelector.Labels)
		if err != nil {
			taskErr = err
			break
		}
		nodeRun, err := e.createNodeRun(run.ID, executionIndex, renderedNode, selectedAgent.ID)
		if err != nil {
			taskErr = err
			break
		}
		var status string
		var nodeOutput map[string]any
		if selectedAgent.EndpointURL != "" {
			status, nodeOutput, err = e.executeRemoteNode(ctx, run.ID, nodeRun, renderedNode, selectedAgent)
		} else {
			status, nodeOutput, err = e.executeNode(ctx, run.ID, nodeRun, renderedNode, workspace)
		}
		if nodeOutput != nil {
			nodeOutputs[pipelineNode.ID] = nodeOutput
		}
		if err != nil {
			nextID := pipelineNode.FallbackNodeID
			if nextID != "" {
				e.appendLog(run.ID, nodeRun.ID, "system", fmt.Sprintf("fallback to node %s\n", nextID))
				currentID = nextID
				executionIndex++
				continue
			}
			if pipelineNode.ContinueOnError {
				nextID = nextSuccessID(pipeline.Nodes, nodeIndex[currentID], pipelineNode, usesEdges)
				e.appendLog(run.ID, nodeRun.ID, "system", fmt.Sprintf("node failed but continueOnError=true: %v\n", err))
				currentID = nextID
				executionIndex++
				continue
			}
			taskErr = err
			break
		}
		if status == model.NodeRunTimeout && pipelineNode.FallbackNodeID == "" && !pipelineNode.ContinueOnError {
			taskErr = context.DeadlineExceeded
			break
		}
		currentID = nextSuccessID(pipeline.Nodes, nodeIndex[currentID], pipelineNode, usesEdges)
		executionIndex++
	}

	if taskErr != nil {
		s := model.TaskRunFailed
		if errors.Is(taskErr, context.DeadlineExceeded) {
			s = model.TaskRunTimeout
		}
		e.finishTask(&run, s, startedAt, taskErr.Error())
		return
	}
	e.finishTask(&run, model.TaskRunSuccess, startedAt, "")
}

func indexPipeline(pipeline node.PipelineDefinition) (map[string]node.PipelineNode, map[string]int, bool, error) {
	nodeByID := map[string]node.PipelineNode{}
	nodeIndex := map[string]int{}
	usesEdges := pipeline.StartNodeID != ""
	for index, pipelineNode := range pipeline.Nodes {
		if pipelineNode.ID == "" {
			return nil, nil, false, fmt.Errorf("node at index %d has empty id", index)
		}
		if _, exists := nodeByID[pipelineNode.ID]; exists {
			return nil, nil, false, fmt.Errorf("duplicate node id %q", pipelineNode.ID)
		}
		if pipelineNode.NextNodeID != "" || pipelineNode.FallbackNodeID != "" {
			usesEdges = true
		}
		nodeByID[pipelineNode.ID] = pipelineNode
		nodeIndex[pipelineNode.ID] = index
	}
	if len(pipeline.Nodes) == 0 {
		return nodeByID, nodeIndex, usesEdges, nil
	}
	if pipeline.StartNodeID != "" {
		if _, ok := nodeByID[pipeline.StartNodeID]; !ok {
			return nil, nil, false, fmt.Errorf("startNodeId %q not found", pipeline.StartNodeID)
		}
	}
	for _, pipelineNode := range pipeline.Nodes {
		if pipelineNode.NextNodeID != "" {
			if _, ok := nodeByID[pipelineNode.NextNodeID]; !ok {
				return nil, nil, false, fmt.Errorf("nextNodeId %q of node %q not found", pipelineNode.NextNodeID, pipelineNode.ID)
			}
		}
		if pipelineNode.FallbackNodeID != "" {
			if _, ok := nodeByID[pipelineNode.FallbackNodeID]; !ok {
				return nil, nil, false, fmt.Errorf("fallbackNodeId %q of node %q not found", pipelineNode.FallbackNodeID, pipelineNode.ID)
			}
		}
	}
	return nodeByID, nodeIndex, usesEdges, nil
}

func nextSuccessID(nodes []node.PipelineNode, currentIndex int, pipelineNode node.PipelineNode, usesEdges bool) string {
	if pipelineNode.NextNodeID != "" {
		return pipelineNode.NextNodeID
	}
	if usesEdges {
		return ""
	}
	nextIndex := currentIndex + 1
	if nextIndex >= len(nodes) {
		return ""
	}
	return nodes[nextIndex].ID
}

func renderPipelineNode(pipelineNode node.PipelineNode, input map[string]any, nodeOutputs map[string]map[string]any) node.PipelineNode {
	rendered, ok := renderParams(pipelineNode.Params, input, nodeOutputs).(map[string]any)
	if !ok {
		rendered = map[string]any{}
	}
	pipelineNode.Params = rendered
	return pipelineNode
}

func renderParams(value any, input map[string]any, nodeOutputs map[string]map[string]any) any {
	switch typed := value.(type) {
	case map[string]any:
		next := map[string]any{}
		for key, item := range typed {
			next[key] = renderParams(item, input, nodeOutputs)
		}
		return next
	case []any:
		next := make([]any, 0, len(typed))
		for _, item := range typed {
			next = append(next, renderParams(item, input, nodeOutputs))
		}
		return next
	case string:
		return renderInputString(typed, input, nodeOutputs)
	default:
		return value
	}
}

// renderInputString replaces ${input.key} / ${key} with run inputs
// and ${node.nodeId.key} with the output of a previously executed node.
func renderInputString(value string, input map[string]any, nodeOutputs map[string]map[string]any) string {
	rendered := value
	for key, item := range input {
		replacement := fmt.Sprint(item)
		rendered = strings.ReplaceAll(rendered, "${input."+key+"}", replacement)
		rendered = strings.ReplaceAll(rendered, "${"+key+"}", replacement)
	}
	for nodeID, outputs := range nodeOutputs {
		for key, item := range outputs {
			rendered = strings.ReplaceAll(rendered, "${node."+nodeID+"."+key+"}", fmt.Sprint(item))
		}
	}
	return rendered
}

func (e *Engine) createNodeRun(taskRunID uint, index int, pipelineNode node.PipelineNode, agentID uint) (model.NodeRun, error) {
	params, _ := json.Marshal(pipelineNode.Params)
	nodeRun := model.NodeRun{
		TaskRunID:          taskRunID,
		AgentID:            agentID,
		NodeID:             pipelineNode.ID,
		NodeName:           pipelineNode.Name,
		NodeType:           pipelineNode.Type,
		Status:             model.NodeRunPending,
		NodeIndex:          index,
		ParamsSnapshotJSON: string(params),
	}
	if err := e.db.Create(&nodeRun).Error; err != nil {
		return nodeRun, err
	}
	e.publishNodeStatus(taskRunID, nodeRun)
	return nodeRun, nil
}

func (e *Engine) selectAgent(labels []string) (model.Agent, error) {
	var agents []model.Agent
	if err := e.db.Where("status IN ?", []string{"online", "idle", "running"}).Order("id asc").Find(&agents).Error; err != nil {
		return model.Agent{}, err
	}
	if len(labels) == 0 {
		labels = []string{"local"}
	}
	for _, item := range agents {
		if labelsMatch(item.LabelsJSON, labels) {
			return item, nil
		}
	}
	return model.Agent{}, fmt.Errorf("no online agent matches labels %v", labels)
}

func labelsMatch(labelsJSON string, required []string) bool {
	var labels []string
	_ = json.Unmarshal([]byte(labelsJSON), &labels)
	set := map[string]bool{}
	for _, label := range labels {
		set[label] = true
	}
	for _, label := range required {
		if !set[label] {
			return false
		}
	}
	return true
}

func (e *Engine) executeNode(ctx context.Context, taskRunID uint, nodeRun model.NodeRun, pipelineNode node.PipelineNode, workspace string) (string, map[string]any, error) {
	executor, err := e.registry.MustGet(pipelineNode.Type)
	if err != nil {
		e.failNode(taskRunID, &nodeRun, model.NodeRunFailed, time.Now(), err.Error())
		return model.NodeRunFailed, nil, err
	}
	if err := executor.Validate(pipelineNode.Params); err != nil {
		e.failNode(taskRunID, &nodeRun, model.NodeRunFailed, time.Now(), err.Error())
		return model.NodeRunFailed, nil, err
	}

	startedAt := time.Now()
	nodeRun.StartedAt = &startedAt
	nodeRun.Status = model.NodeRunRunning
	if err := e.db.Save(&nodeRun).Error; err != nil {
		log.Printf("engine: save nodeRun error: %v", err)
	}
	e.publishNodeStatus(taskRunID, nodeRun)
	e.appendLog(taskRunID, nodeRun.ID, "system", fmt.Sprintf("[node:start] %s (%s)\n", pipelineNode.Name, pipelineNode.ID))

	attempts := pipelineNode.RetryTimes + 1
	if attempts < 1 {
		attempts = 1
	}

	var result *node.NodeResult
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		nodeCtx := ctx
		cancel := func() {}
		if pipelineNode.TimeoutSeconds > 0 {
			nodeCtx, cancel = context.WithTimeout(ctx, time.Duration(pipelineNode.TimeoutSeconds)*time.Second)
		}
		result, lastErr = executor.Execute(&node.NodeContext{
			Context:           nodeCtx,
			TaskRunID:         taskRunID,
			NodeRunID:         nodeRun.ID,
			Workspace:         workspace,
			ResolveCredential: e.creds.Resolve,
			Log: func(stream, content string) {
				e.appendLog(taskRunID, nodeRun.ID, stream, content)
			},
		}, pipelineNode.Params)
		cancel()
		nodeRun.RetryCount = attempt - 1
		if lastErr == nil {
			break
		}
		if attempt < attempts {
			e.appendLog(taskRunID, nodeRun.ID, "system", fmt.Sprintf("attempt %d failed: %v, retrying\n", attempt, lastErr))
		}
	}

	finishedAt := time.Now()
	nodeRun.FinishedAt = &finishedAt
	nodeRun.DurationMS = finishedAt.Sub(startedAt).Milliseconds()

	var output map[string]any
	if result != nil {
		output = result.Output
		outputJSON, _ := json.Marshal(output)
		nodeRun.OutputJSON = string(outputJSON)
	}

	if lastErr != nil {
		status := model.NodeRunFailed
		if errors.Is(lastErr, context.DeadlineExceeded) {
			status = model.NodeRunTimeout
		}
		nodeRun.Status = status
		nodeRun.ErrorMessage = lastErr.Error()
		if err := e.db.Save(&nodeRun).Error; err != nil {
			log.Printf("engine: save nodeRun error: %v", err)
		}
		e.publishNodeStatus(taskRunID, nodeRun)
		e.appendLog(taskRunID, nodeRun.ID, "system", fmt.Sprintf("[node:failed] %s (%s) duration=%dms error=%v\n", pipelineNode.Name, pipelineNode.ID, nodeRun.DurationMS, lastErr))
		return status, nil, lastErr
	}

	nodeRun.Status = model.NodeRunSuccess
	if err := e.db.Save(&nodeRun).Error; err != nil {
		log.Printf("engine: save nodeRun error: %v", err)
	}
	e.publishNodeStatus(taskRunID, nodeRun)
	e.appendLog(taskRunID, nodeRun.ID, "system", fmt.Sprintf("[node:end] %s (%s) status=success duration=%dms\n", pipelineNode.Name, pipelineNode.ID, nodeRun.DurationMS))
	return model.NodeRunSuccess, output, nil
}

func (e *Engine) executeRemoteNode(ctx context.Context, taskRunID uint, nodeRun model.NodeRun, pipelineNode node.PipelineNode, selectedAgent model.Agent) (string, map[string]any, error) {
	startedAt := time.Now()
	nodeRun.StartedAt = &startedAt
	nodeRun.AssignedAt = &startedAt
	nodeRun.Status = model.NodeRunRunning
	if err := e.db.Save(&nodeRun).Error; err != nil {
		log.Printf("engine: save nodeRun error: %v", err)
	}
	e.publishNodeStatus(taskRunID, nodeRun)
	e.appendLog(taskRunID, nodeRun.ID, "system", fmt.Sprintf("[node:start] %s (%s) agent=%s\n", pipelineNode.Name, pipelineNode.ID, selectedAgent.Name))

	attempts := pipelineNode.RetryTimes + 1
	if attempts < 1 {
		attempts = 1
	}

	var resp agentproto.ExecuteNodeResponse
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		resp, lastErr = e.dispatchRemoteNode(ctx, taskRunID, nodeRun.ID, pipelineNode, selectedAgent)
		nodeRun.RetryCount = attempt - 1
		if lastErr == nil && resp.Status == model.NodeRunSuccess {
			break
		}
		if lastErr == nil && resp.ErrorMessage != "" {
			lastErr = errors.New(resp.ErrorMessage)
		}
		if lastErr == nil {
			lastErr = fmt.Errorf("agent returned status %s", resp.Status)
		}
		if attempt < attempts {
			e.appendLog(taskRunID, nodeRun.ID, "system", fmt.Sprintf("attempt %d failed: %v, retrying\n", attempt, lastErr))
		}
	}

	finishedAt := time.Now()
	nodeRun.FinishedAt = &finishedAt
	if resp.DurationMS > 0 {
		nodeRun.DurationMS = resp.DurationMS
	} else {
		nodeRun.DurationMS = finishedAt.Sub(startedAt).Milliseconds()
	}

	var output map[string]any
	if resp.Output != nil {
		output = resp.Output
		outputJSON, _ := json.Marshal(output)
		nodeRun.OutputJSON = string(outputJSON)
	}

	if lastErr != nil {
		nodeRun.Status = model.NodeRunFailed
		nodeRun.ErrorMessage = lastErr.Error()
		if err := e.db.Save(&nodeRun).Error; err != nil {
			log.Printf("engine: save nodeRun error: %v", err)
		}
		e.publishNodeStatus(taskRunID, nodeRun)
		e.appendLog(taskRunID, nodeRun.ID, "system", fmt.Sprintf("[node:failed] %s (%s) duration=%dms error=%v\n", pipelineNode.Name, pipelineNode.ID, nodeRun.DurationMS, lastErr))
		return model.NodeRunFailed, nil, lastErr
	}

	nodeRun.Status = model.NodeRunSuccess
	if err := e.db.Save(&nodeRun).Error; err != nil {
		log.Printf("engine: save nodeRun error: %v", err)
	}
	e.publishNodeStatus(taskRunID, nodeRun)
	e.appendLog(taskRunID, nodeRun.ID, "system", fmt.Sprintf("[node:end] %s (%s) status=success duration=%dms\n", pipelineNode.Name, pipelineNode.ID, nodeRun.DurationMS))
	return model.NodeRunSuccess, output, nil
}

func (e *Engine) dispatchRemoteNode(ctx context.Context, taskRunID uint, nodeRunID uint, pipelineNode node.PipelineNode, selectedAgent model.Agent) (agentproto.ExecuteNodeResponse, error) {
	token, err := e.agents.Token(selectedAgent)
	if err != nil {
		return agentproto.ExecuteNodeResponse{}, err
	}
	credentials, err := e.credentialsForNode(pipelineNode)
	if err != nil {
		return agentproto.ExecuteNodeResponse{}, err
	}
	req := agentproto.ExecuteNodeRequest{
		TaskRunID:   taskRunID,
		NodeRunID:   nodeRunID,
		Workspace:   fmt.Sprintf("taskrun-%d", taskRunID),
		ServerURL:   strings.TrimRight(e.cfg.ServerURL, "/"),
		Node:        pipelineNode,
		Credentials: credentials,
	}
	content, _ := json.Marshal(req)
	endpoint := strings.TrimRight(selectedAgent.EndpointURL, "/") + "/api/agent/execute-node"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(content))
	if err != nil {
		return agentproto.ExecuteNodeResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{Timeout: remoteTimeout(pipelineNode.TimeoutSeconds)}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return agentproto.ExecuteNodeResponse{}, err
	}
	defer httpResp.Body.Close()
	body, _ := io.ReadAll(httpResp.Body)
	if httpResp.StatusCode >= 300 {
		return agentproto.ExecuteNodeResponse{}, fmt.Errorf("agent dispatch failed: %s", strings.TrimSpace(string(body)))
	}
	var apiResp struct {
		Code    int                            `json:"code"`
		Message string                         `json:"message"`
		Data    agentproto.ExecuteNodeResponse `json:"data"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return agentproto.ExecuteNodeResponse{}, err
	}
	if apiResp.Code != 0 {
		return agentproto.ExecuteNodeResponse{}, errors.New(apiResp.Message)
	}
	return apiResp.Data, nil
}

func (e *Engine) credentialsForNode(pipelineNode node.PipelineNode) ([]node.Credential, error) {
	credentialID := uintFromAny(pipelineNode.Params["credentialId"])
	if credentialID == 0 {
		return nil, nil
	}
	cred, err := e.creds.Resolve(credentialID)
	if err != nil {
		return nil, err
	}
	if cred == nil {
		return nil, nil
	}
	return []node.Credential{*cred}, nil
}

func remoteTimeout(seconds int) time.Duration {
	if seconds <= 0 {
		return 30 * time.Minute
	}
	return time.Duration(seconds+30) * time.Second
}

func uintFromAny(value any) uint {
	switch typed := value.(type) {
	case float64:
		return uint(typed)
	case int:
		return uint(typed)
	case uint:
		return typed
	case string:
		id, _ := strconv.ParseUint(typed, 10, 64)
		return uint(id)
	default:
		return 0
	}
}

func (e *Engine) failNode(taskRunID uint, nodeRun *model.NodeRun, status string, startedAt time.Time, message string) {
	now := time.Now()
	nodeRun.Status = status
	nodeRun.StartedAt = &startedAt
	nodeRun.FinishedAt = &now
	nodeRun.DurationMS = now.Sub(startedAt).Milliseconds()
	nodeRun.ErrorMessage = message
	if err := e.db.Save(nodeRun).Error; err != nil {
		log.Printf("engine: save nodeRun error: %v", err)
	}
	e.publishNodeStatus(taskRunID, *nodeRun)
	e.appendLog(taskRunID, nodeRun.ID, "system", fmt.Sprintf("[node:failed] %s (%s) duration=%dms error=%s\n", nodeRun.NodeName, nodeRun.NodeID, nodeRun.DurationMS, message))
}

func (e *Engine) finishTask(run *model.TaskRun, status string, startedAt time.Time, message string) {
	now := time.Now()
	run.Status = status
	run.FinishedAt = &now
	run.DurationMS = now.Sub(startedAt).Milliseconds()
	run.ErrorMessage = message
	if err := e.db.Save(run).Error; err != nil {
		log.Printf("engine: save run error: %v", err)
	}
	e.appendLog(run.ID, 0, "system", fmt.Sprintf("[task:end] status=%s duration=%dms\n", status, run.DurationMS))
	e.publishTaskStatus(*run)

	e.logMu.Lock()
	delete(e.next, run.ID)
	e.logMu.Unlock()
}

func (e *Engine) appendLog(taskRunID uint, nodeRunID uint, stream string, content string) {
	e.logMu.Lock()
	next := e.next[taskRunID]
	if next == 0 {
		var max int
		_ = e.db.Model(&model.RunLog{}).
			Where("task_run_id = ?", taskRunID).
			Select("COALESCE(MAX(sequence), 0)").
			Scan(&max).Error
		next = max
	}
	next++
	e.next[taskRunID] = next
	e.logMu.Unlock()

	entry := model.RunLog{
		TaskRunID: taskRunID,
		NodeRunID: nodeRunID,
		Sequence:  next,
		Stream:    stream,
		Content:   content,
	}
	_ = e.db.Create(&entry).Error
	e.hub.Publish(taskRunID, logstream.Event{
		Type: "log",
		Data: map[string]any{
			"taskRunId": taskRunID,
			"nodeRunId": nodeRunID,
			"stream":    stream,
			"content":   content,
			"sequence":  next,
		},
	})
}

func (e *Engine) AppendLog(taskRunID uint, nodeRunID uint, stream string, content string) {
	e.appendLog(taskRunID, nodeRunID, stream, content)
}

func (e *Engine) publishTaskStatus(run model.TaskRun) {
	e.hub.Publish(run.ID, logstream.Event{
		Type: "task_status",
		Data: map[string]any{
			"taskRunId": run.ID,
			"status":    run.Status,
			"run":       run,
		},
	})
}

func (e *Engine) publishNodeStatus(taskRunID uint, nodeRun model.NodeRun) {
	e.hub.Publish(taskRunID, logstream.Event{
		Type: "node_status",
		Data: map[string]any{
			"nodeRunId": nodeRun.ID,
			"status":    nodeRun.Status,
			"nodeRun":   nodeRun,
		},
	})
}

func (e *Engine) setAgentStatus(status string) {
	_ = e.db.Model(&model.Agent{}).Where("name = ?", "local-agent").Updates(map[string]any{
		"status":            status,
		"last_heartbeat_at": time.Now(),
	}).Error
}
