package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"puppet/internal/agent"
	"puppet/internal/auth"
	"puppet/internal/confignode"
	"puppet/internal/credential"
	"puppet/internal/engine"
	"puppet/internal/logstream"
	"puppet/internal/model"
	"puppet/internal/node"
	"puppet/internal/project"
	"puppet/internal/task"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct {
	db             *gorm.DB
	registry       *node.Registry
	configRegistry *confignode.Registry
	engine         *engine.Engine
	hub            *logstream.Hub
	projects       *project.Service
	tasks          *task.Service
	agents         *agent.Service
	creds          *credential.Service
	auths          *auth.Service
}

type resolvedRunInput struct {
	Name     string   `json:"name"`
	Label    string   `json:"label"`
	Type     string   `json:"type"`
	Required bool     `json:"required"`
	Default  any      `json:"default,omitempty"`
	Options  []string `json:"options"`
	Error    string   `json:"error,omitempty"`
}

func NewRouter(db *gorm.DB, registry *node.Registry, configRegistry *confignode.Registry, runner *engine.Engine, hub *logstream.Hub) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(cors())

	h := &Handler{
		db:             db,
		registry:       registry,
		configRegistry: configRegistry,
		engine:         runner,
		hub:            hub,
		projects:       project.NewService(db),
		tasks:          task.NewService(db),
		agents:         agent.NewService(db),
		creds:          credential.NewService(db),
		auths:          auth.NewService(db),
	}

	api := r.Group("/api")
	{
		api.POST("/auth/login", h.login)
		api.POST("/auth/logout", h.authMiddleware(), h.logout)
		api.GET("/auth/me", h.authMiddleware(), h.me)

		api.POST("/agent-callback/heartbeat", h.agentHeartbeat)
		api.POST("/agent-callback/node-runs/:id/logs", h.agentAppendLog)

		protected := api.Group("")
		protected.Use(h.authMiddleware())
		{
			protected.GET("/dashboard/summary", h.dashboardSummary)

			protected.GET("/projects", h.listProjects)
			protected.POST("/projects", h.createProject)
			protected.GET("/projects/:id", h.getProject)
			protected.PUT("/projects/:id", h.updateProject)
			protected.DELETE("/projects/:id", h.deleteProject)

			protected.GET("/projects/:id/tasks", h.listTasks)
			protected.POST("/projects/:id/tasks", h.createTask)
			protected.GET("/tasks/:id", h.getTask)
			protected.PUT("/tasks/:id", h.updateTask)
			protected.DELETE("/tasks/:id", h.deleteTask)

			protected.GET("/tasks/:id/pipeline", h.getPipeline)
			protected.PUT("/tasks/:id/pipeline", h.updatePipeline)
			protected.GET("/node-types", h.nodeTypes)
			protected.GET("/config-node-types", h.configNodeTypes)
			protected.GET("/tasks/:id/run-config", h.getRunConfig)

			protected.POST("/tasks/:id/run", h.runTask)
			protected.GET("/tasks/:id/runs", h.listTaskRuns)
			protected.GET("/task-runs/:id", h.getTaskRun)
			protected.GET("/task-runs/:id/node-runs", h.listNodeRuns)
			protected.GET("/task-runs/:id/logs", h.listRunLogs)
			protected.GET("/task-runs/:id/events", h.taskRunEvents)

			protected.GET("/agents", h.listAgents)
			protected.POST("/agents", h.createAgent)
			protected.GET("/agents/:id", h.getAgent)
			protected.PUT("/agents/:id", h.updateAgent)
			protected.DELETE("/agents/:id", h.deleteAgent)

			protected.GET("/credentials", h.listCredentials)
			protected.POST("/credentials", h.createCredential)
			protected.GET("/credentials/:id", h.getCredential)
			protected.PUT("/credentials/:id", h.updateCredential)
			protected.DELETE("/credentials/:id", h.deleteCredential)

			protected.GET("/users", h.adminOnly(), h.listUsers)
			protected.POST("/users", h.adminOnly(), h.createUser)
			protected.PUT("/users/:id", h.adminOnly(), h.updateUser)
			protected.DELETE("/users/:id", h.adminOnly(), h.deleteUser)
		}
	}

	return r
}

func cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func (h *Handler) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := bearerToken(c)
		if token == "" {
			token = c.Query("token")
		}
		if token == "" {
			fail(c, http.StatusUnauthorized, errors.New("missing authorization token"))
			c.Abort()
			return
		}
		user, err := h.auths.Authenticate(token)
		if err != nil {
			fail(c, http.StatusUnauthorized, errors.New("invalid authorization token"))
			c.Abort()
			return
		}
		c.Set("user", user)
		c.Set("token", token)
		c.Next()
	}
}

func (h *Handler) adminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, _ := c.Get("user")
		if u, ok := user.(model.User); ok && u.Role == "admin" {
			c.Next()
			return
		}
		fail(c, http.StatusForbidden, errors.New("admin role required"))
		c.Abort()
	}
}

func bearerToken(c *gin.Context) string {
	header := c.GetHeader("Authorization")
	if header == "" {
		return ""
	}
	const prefix = "Bearer "
	if len(header) <= len(prefix) || header[:len(prefix)] != prefix {
		return ""
	}
	return header[len(prefix):]
}

func (h *Handler) login(c *gin.Context) {
	var req auth.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	resp, err := h.auths.Login(req)
	respond(c, resp, err)
}

func (h *Handler) logout(c *gin.Context) {
	token := c.GetString("token")
	respond(c, gin.H{"loggedOut": true}, h.auths.Logout(token))
}

func (h *Handler) me(c *gin.Context) {
	user, _ := c.Get("user")
	ok(c, user)
}

func (h *Handler) listUsers(c *gin.Context) {
	users, err := h.auths.ListUsers()
	respond(c, users, err)
}

func (h *Handler) createUser(c *gin.Context) {
	var req auth.UserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	user, err := h.auths.CreateUser(req)
	respond(c, user, err)
}

func (h *Handler) updateUser(c *gin.Context) {
	var req auth.UserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	user, err := h.auths.UpdateUser(paramID(c, "id"), req)
	respond(c, user, err)
}

func (h *Handler) deleteUser(c *gin.Context) {
	respond(c, gin.H{"deleted": true}, h.auths.DeleteUser(paramID(c, "id")))
}

func (h *Handler) dashboardSummary(c *gin.Context) {
	var projectCount, taskCount, todayRunCount, runningCount, successCount, failedCount, agentOnlineCount int64
	today := time.Now().Truncate(24 * time.Hour)
	h.db.Model(&model.Project{}).Count(&projectCount)
	h.db.Model(&model.Task{}).Count(&taskCount)
	h.db.Model(&model.TaskRun{}).Where("created_at >= ?", today).Count(&todayRunCount)
	h.db.Model(&model.TaskRun{}).Where("status IN ?", []string{model.TaskRunPending, model.TaskRunRunning}).Count(&runningCount)
	h.db.Model(&model.TaskRun{}).Where("status = ?", model.TaskRunSuccess).Count(&successCount)
	h.db.Model(&model.TaskRun{}).Where("status = ?", model.TaskRunFailed).Count(&failedCount)
	h.db.Model(&model.Agent{}).Where("status IN ?", []string{"online", "running", "idle"}).Count(&agentOnlineCount)

	var recentRuns []model.TaskRun
	h.db.Order("id desc").Limit(8).Find(&recentRuns)
	ok(c, gin.H{
		"projectCount":     projectCount,
		"taskCount":        taskCount,
		"todayRunCount":    todayRunCount,
		"runningCount":     runningCount,
		"successCount":     successCount,
		"failedCount":      failedCount,
		"agentOnlineCount": agentOnlineCount,
		"recentRuns":       recentRuns,
	})
}

func (h *Handler) listProjects(c *gin.Context) {
	projects, err := h.projects.List()
	respond(c, projects, err)
}

func (h *Handler) createProject(c *gin.Context) {
	var req model.Project
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	if req.Name == "" {
		fail(c, http.StatusBadRequest, errors.New("name is required"))
		return
	}
	project, err := h.projects.Create(model.Project{Name: req.Name, Description: req.Description})
	respond(c, project, err)
}

func (h *Handler) getProject(c *gin.Context) {
	project, err := h.projects.Get(paramID(c, "id"))
	respond(c, project, err)
}

func (h *Handler) updateProject(c *gin.Context) {
	var req model.Project
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	project, err := h.projects.Update(paramID(c, "id"), req)
	respond(c, project, err)
}

func (h *Handler) deleteProject(c *gin.Context) {
	respond(c, gin.H{"deleted": true}, h.projects.Delete(paramID(c, "id")))
}

func (h *Handler) listTasks(c *gin.Context) {
	tasks, err := h.tasks.ListByProject(paramID(c, "id"))
	respond(c, tasks, err)
}

func (h *Handler) createTask(c *gin.Context) {
	var req model.Task
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	if req.Name == "" {
		fail(c, http.StatusBadRequest, errors.New("name is required"))
		return
	}
	if req.TimeoutSeconds == 0 {
		req.TimeoutSeconds = 600
	}
	req.ProjectID = paramID(c, "id")
	task, err := h.tasks.Create(req)
	respond(c, task, err)
}

func (h *Handler) getTask(c *gin.Context) {
	task, err := h.tasks.Get(paramID(c, "id"))
	respond(c, task, err)
}

func (h *Handler) updateTask(c *gin.Context) {
	var req model.Task
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	task, err := h.tasks.Update(paramID(c, "id"), req)
	respond(c, task, err)
}

func (h *Handler) deleteTask(c *gin.Context) {
	respond(c, gin.H{"deleted": true}, h.tasks.Delete(paramID(c, "id")))
}

func (h *Handler) getPipeline(c *gin.Context) {
	pipeline, err := h.tasks.Pipeline(paramID(c, "id"))
	respond(c, pipeline, err)
}

func (h *Handler) updatePipeline(c *gin.Context) {
	var pipeline node.PipelineDefinition
	if err := c.ShouldBindJSON(&pipeline); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	if pipeline.Name == "" {
		pipeline.Name = "Pipeline"
	}
	if pipeline.AgentSelector.Labels == nil {
		pipeline.AgentSelector.Labels = []string{"local"}
	}
	for index := range pipeline.Nodes {
		if pipeline.Nodes[index].Params == nil {
			pipeline.Nodes[index].Params = map[string]any{}
		}
		if pipeline.Nodes[index].ID == "" {
			pipeline.Nodes[index].ID = fmt.Sprintf("node-%d", index+1)
		}
	}
	if pipeline.StartNodeID == "" && len(pipeline.Nodes) > 0 {
		pipeline.StartNodeID = pipeline.Nodes[0].ID
	}
	if err := validatePipelineRefs(pipeline); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	saved, err := h.tasks.UpdatePipeline(paramID(c, "id"), pipeline)
	respond(c, saved, err)
}

func (h *Handler) nodeTypes(c *gin.Context) {
	ok(c, h.registry.Metadata())
}

func (h *Handler) configNodeTypes(c *gin.Context) {
	ok(c, h.configRegistry.Metadata())
}

func (h *Handler) getRunConfig(c *gin.Context) {
	pipeline, err := h.tasks.Pipeline(paramID(c, "id"))
	if err != nil {
		respond(c, nil, err)
		return
	}
	resolved, err := h.resolveRunInputs(c.Request.Context(), pipeline)
	if err != nil {
		respond(c, nil, err)
		return
	}
	ok(c, gin.H{"inputs": resolved})
}

func (h *Handler) runTask(c *gin.Context) {
	var req struct {
		Input map[string]any `json:"input"`
	}
	_ = c.ShouldBindJSON(&req)
	pipeline, err := h.tasks.Pipeline(paramID(c, "id"))
	if err != nil {
		respond(c, nil, err)
		return
	}
	input, err := h.normalizeRunInput(c.Request.Context(), pipeline, req.Input)
	if err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	triggeredBy := "local-user"
	if current, exists := c.Get("user"); exists {
		if user, ok := current.(model.User); ok {
			triggeredBy = user.Username
		}
	}
	run, err := h.engine.StartTask(c.Request.Context(), paramID(c, "id"), "manual", triggeredBy, input)
	respond(c, run, err)
}

func (h *Handler) resolveRunInputs(ctx context.Context, pipeline node.PipelineDefinition) ([]resolvedRunInput, error) {
	resolved := make([]resolvedRunInput, 0, len(pipeline.Inputs))
	for _, input := range pipeline.Inputs {
		item := resolvedRunInput{
			Name:     input.Name,
			Label:    input.Label,
			Type:     input.Type,
			Required: input.Required,
			Default:  input.Default,
			Options:  input.Options,
		}
		if item.Options == nil {
			item.Options = []string{}
		}
		if input.Source != nil {
			opts, err := h.fetchSourceOptions(ctx, input.Source)
			if err != nil {
				item.Error = err.Error()
				item.Options = []string{}
			} else {
				item.Options = opts
			}
		}
		resolved = append(resolved, item)
	}
	return resolved, nil
}

func (h *Handler) fetchSourceOptions(ctx context.Context, source *node.InputSource) ([]string, error) {
	executor, err := h.configRegistry.MustGet(source.Type)
	if err != nil {
		return nil, err
	}
	result, err := executor.Execute(confignode.Context{
		Context:           ctx,
		ResolveCredential: h.creds.Resolve,
	}, source.Params)
	if err != nil {
		return nil, err
	}
	return valuesToStrings(result.Output["options"]), nil
}

func (h *Handler) normalizeRunInput(ctx context.Context, pipeline node.PipelineDefinition, input map[string]any) (map[string]any, error) {
	resolved, err := h.resolveRunInputs(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	normalized := map[string]any{}
	for key, value := range input {
		normalized[key] = value
	}
	for _, item := range resolved {
		value, exists := normalized[item.Name]
		empty := inputIsEmpty(value, item.Type, exists)
		if empty && item.Default != nil {
			value = item.Default
			normalized[item.Name] = value
			empty = false
		}
		if item.Required && empty {
			return nil, fmt.Errorf("input %q is required", item.Name)
		}
		// Skip select validation when source failed (options may be empty due to error).
		if item.Type == "select" && !empty && item.Error == "" && len(item.Options) > 0 {
			if !containsString(item.Options, fmt.Sprint(value)) {
				return nil, fmt.Errorf("input %q value %q is not in options", item.Name, fmt.Sprint(value))
			}
		}
	}
	return normalized, nil
}

func inputIsEmpty(value any, inputType string, exists bool) bool {
	if !exists || value == nil {
		return true
	}
	if inputType == "boolean" || inputType == "number" {
		return false
	}
	return strings.TrimSpace(fmt.Sprint(value)) == ""
}

func (h *Handler) listTaskRuns(c *gin.Context) {
	var runs []model.TaskRun
	err := h.db.Where("task_id = ?", paramID(c, "id")).Order("id desc").Find(&runs).Error
	respond(c, runs, err)
}

func (h *Handler) getTaskRun(c *gin.Context) {
	var run model.TaskRun
	err := h.db.First(&run, paramID(c, "id")).Error
	respond(c, run, err)
}

func (h *Handler) listNodeRuns(c *gin.Context) {
	var runs []model.NodeRun
	err := h.db.Where("task_run_id = ?", paramID(c, "id")).Order("node_index asc, id asc").Find(&runs).Error
	respond(c, runs, err)
}

func (h *Handler) listRunLogs(c *gin.Context) {
	var logs []model.RunLog
	err := h.db.Where("task_run_id = ?", paramID(c, "id")).Order("sequence asc").Find(&logs).Error
	respond(c, logs, err)
}

func (h *Handler) taskRunEvents(c *gin.Context) {
	taskRunID := paramID(c, "id")
	ch := h.hub.Subscribe(taskRunID)
	defer h.hub.Unsubscribe(taskRunID, ch)

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.WriteHeaderNow()
	flusher, _ := c.Writer.(http.Flusher)
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case event := <-ch:
			content, _ := json.Marshal(event.Data)
			_, _ = fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event.Type, content)
			flusher.Flush()
		case <-ticker.C:
			_, _ = fmt.Fprint(c.Writer, ": ping\n\n")
			flusher.Flush()
		case <-c.Request.Context().Done():
			return
		}
	}
}

func (h *Handler) listAgents(c *gin.Context) {
	agents, err := h.agents.List()
	respond(c, agents, err)
}

func (h *Handler) getAgent(c *gin.Context) {
	agent, err := h.agents.Get(paramID(c, "id"))
	respond(c, agent, err)
}

func (h *Handler) createAgent(c *gin.Context) {
	var req agent.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	created, err := h.agents.Create(req)
	respond(c, created, err)
}

func (h *Handler) updateAgent(c *gin.Context) {
	var req agent.UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	updated, err := h.agents.Update(paramID(c, "id"), req)
	respond(c, updated, err)
}

func (h *Handler) deleteAgent(c *gin.Context) {
	respond(c, gin.H{"deleted": true}, h.agents.Delete(paramID(c, "id")))
}

func (h *Handler) authenticateAgent(c *gin.Context) (model.Agent, bool) {
	token := bearerToken(c)
	if token == "" {
		fail(c, http.StatusUnauthorized, errors.New("missing agent token"))
		return model.Agent{}, false
	}
	agent, err := h.agents.AuthenticateBearer(token)
	if err != nil {
		fail(c, http.StatusUnauthorized, errors.New("invalid agent token"))
		return model.Agent{}, false
	}
	return agent, true
}

func (h *Handler) agentHeartbeat(c *gin.Context) {
	agent, ok := h.authenticateAgent(c)
	if !ok {
		return
	}
	var req struct {
		OS       string `json:"os"`
		Arch     string `json:"arch"`
		Hostname string `json:"hostname"`
	}
	_ = c.ShouldBindJSON(&req)
	updated, err := h.agents.Heartbeat(agent, req.OS, req.Arch, req.Hostname)
	respond(c, updated, err)
}

func (h *Handler) agentAppendLog(c *gin.Context) {
	agent, authenticated := h.authenticateAgent(c)
	if !authenticated {
		return
	}
	var nodeRun model.NodeRun
	if err := h.db.First(&nodeRun, paramID(c, "id")).Error; err != nil {
		respond(c, nil, err)
		return
	}
	if nodeRun.AgentID != agent.ID {
		fail(c, http.StatusForbidden, errors.New("node run is not assigned to this agent"))
		return
	}
	var req struct {
		TaskRunID uint   `json:"taskRunId"`
		Stream    string `json:"stream"`
		Content   string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	taskRunID := req.TaskRunID
	if taskRunID == 0 {
		taskRunID = nodeRun.TaskRunID
	}
	h.engine.AppendLog(taskRunID, nodeRun.ID, req.Stream, req.Content)
	ok(c, gin.H{"logged": true})
}

func (h *Handler) listCredentials(c *gin.Context) {
	credentials, err := h.creds.List()
	respond(c, credentials, err)
}

func (h *Handler) createCredential(c *gin.Context) {
	var req credential.UpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	created, err := h.creds.Create(req)
	respond(c, created, err)
}

func (h *Handler) getCredential(c *gin.Context) {
	credential, err := h.creds.Get(paramID(c, "id"))
	respond(c, credential, err)
}

func (h *Handler) updateCredential(c *gin.Context) {
	var req credential.UpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	updated, err := h.creds.Update(paramID(c, "id"), req)
	respond(c, updated, err)
}

func (h *Handler) deleteCredential(c *gin.Context) {
	respond(c, gin.H{"deleted": true}, h.creds.Delete(paramID(c, "id")))
}

func respond(c *gin.Context, data any, err error) {
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		fail(c, status, err)
		return
	}
	ok(c, data)
}

func paramID(c *gin.Context, key string) uint {
	id, _ := strconv.ParseUint(c.Param(key), 10, 64)
	return uint(id)
}

func validatePipelineRefs(pipeline node.PipelineDefinition) error {
	ids := map[string]bool{}
	for _, item := range pipeline.Nodes {
		if ids[item.ID] {
			return fmt.Errorf("duplicate node id %q", item.ID)
		}
		ids[item.ID] = true
	}
	if pipeline.StartNodeID != "" && !ids[pipeline.StartNodeID] {
		return fmt.Errorf("startNodeId %q not found", pipeline.StartNodeID)
	}
	for _, item := range pipeline.Nodes {
		if item.NextNodeID != "" && !ids[item.NextNodeID] {
			return fmt.Errorf("nextNodeId %q of node %q not found", item.NextNodeID, item.ID)
		}
		if item.FallbackNodeID != "" && !ids[item.FallbackNodeID] {
			return fmt.Errorf("fallbackNodeId %q of node %q not found", item.FallbackNodeID, item.ID)
		}
	}
	return nil
}

func valuesToStrings(value any) []string {
	result := []string{}
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		for _, item := range typed {
			result = append(result, fmt.Sprint(item))
		}
	}
	return result
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
