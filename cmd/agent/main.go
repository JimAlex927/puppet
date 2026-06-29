package main

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"puppet/internal/agentproto"
	"puppet/internal/model"
	"puppet/internal/node"
	archivenode "puppet/internal/nodes/archive"
	"puppet/internal/nodes/git"
	httpnode "puppet/internal/nodes/http"
	processnode "puppet/internal/nodes/process"
	"puppet/internal/nodes/shell"
	"puppet/internal/nodes/sleep"

	"github.com/gin-gonic/gin"
)

type runtimeConfig struct {
	listen       string
	serverURL    string
	token        string
	workspaceDir string
	registry     *node.Registry
}

var postLogClient = &http.Client{Timeout: 10 * time.Second}

func main() {
	cfg := runtimeConfig{}
	flag.StringVar(&cfg.listen, "listen", getenv("PUPPET_AGENT_LISTEN", ":9090"), "agent listen address")
	flag.StringVar(&cfg.serverURL, "server", getenv("PUPPET_SERVER_URL", "http://localhost:8080"), "server base URL")
	flag.StringVar(&cfg.token, "token", os.Getenv("PUPPET_AGENT_TOKEN"), "agent token")
	flag.StringVar(&cfg.workspaceDir, "workspace", getenv("PUPPET_AGENT_WORKSPACE_DIR", "agent-workspaces"), "agent workspace dir")
	flag.Parse()
	if cfg.token == "" {
		log.Fatal("agent token is required")
	}
	if err := os.MkdirAll(cfg.workspaceDir, 0o755); err != nil {
		log.Fatal(err)
	}
	cfg.registry = node.NewRegistry()
	cfg.registry.Register(shell.New())
	cfg.registry.Register(sleep.New())
	cfg.registry.Register(httpnode.New())
	cfg.registry.Register(git.New())
	cfg.registry.Register(processnode.NewStart())
	cfg.registry.Register(processnode.NewStop())
	cfg.registry.Register(archivenode.NewCompress())
	cfg.registry.Register(archivenode.NewExtract())

	go heartbeatLoop(cfg)

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	r.POST("/api/agent/execute-node", cfg.executeNode)
	log.Printf("puppet agent listening on %s", cfg.listen)
	if err := r.Run(cfg.listen); err != nil {
		log.Fatal(err)
	}
}

func (cfg runtimeConfig) executeNode(c *gin.Context) {
	if subtle.ConstantTimeCompare([]byte(bearerToken(c)), []byte(cfg.token)) != 1 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "invalid agent token", "data": nil})
		return
	}
	var req agentproto.ExecuteNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error(), "data": nil})
		return
	}
	executor, ok := cfg.registry.Get(req.Node.Type)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": agentproto.ExecuteNodeResponse{
			Status:       model.NodeRunFailed,
			ErrorMessage: fmt.Sprintf("unknown node type %q", req.Node.Type),
		}})
		return
	}
	if err := executor.Validate(req.Node.Params); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": agentproto.ExecuteNodeResponse{
			Status:       model.NodeRunFailed,
			ErrorMessage: fmt.Sprintf("validation failed: %v", err),
		}})
		return
	}
	workspace := req.Workspace
	if !isAbs(workspace) {
		workspace = joinPath(cfg.workspaceDir, workspace)
	}
	_ = os.MkdirAll(workspace, 0o755)
	credentialMap := map[uint]node.Credential{}
	for _, item := range req.Credentials {
		credentialMap[item.ID] = item
	}
	started := time.Now()
	result, err := executor.Execute(&node.NodeContext{
		Context:   c.Request.Context(),
		TaskRunID: req.TaskRunID,
		NodeRunID: req.NodeRunID,
		Workspace: workspace,
		ResolveCredential: func(id uint) (*node.Credential, error) {
			if item, ok := credentialMap[id]; ok {
				return &item, nil
			}
			return nil, fmt.Errorf("credential %d was not included in agent job", id)
		},
		Log: func(stream string, content string) {
			postLog(cfg, req, stream, content)
		},
	}, req.Node.Params)
	resp := agentproto.ExecuteNodeResponse{
		Status:     model.NodeRunSuccess,
		DurationMS: time.Since(started).Milliseconds(),
	}
	if result != nil {
		resp.Output = result.Output
	}
	if err != nil {
		resp.Status = model.NodeRunFailed
		resp.ErrorMessage = err.Error()
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": resp})
}

func postLog(cfg runtimeConfig, req agentproto.ExecuteNodeRequest, stream string, content string) {
	body, _ := json.Marshal(agentproto.LogRequest{TaskRunID: req.TaskRunID, Stream: stream, Content: content})
	httpReq, err := http.NewRequest(http.MethodPost, strings.TrimRight(req.ServerURL, "/")+"/api/agent-callback/node-runs/"+fmt.Sprint(req.NodeRunID)+"/logs", bytes.NewReader(body))
	if err != nil {
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+cfg.token)
	resp, err := postLogClient.Do(httpReq)
	if err == nil {
		_ = resp.Body.Close()
	}
}

func heartbeatLoop(cfg runtimeConfig) {
	for {
		body, _ := json.Marshal(gin.H{"os": runtime.GOOS, "arch": runtime.GOARCH, "hostname": hostname()})
		req, err := http.NewRequest(http.MethodPost, strings.TrimRight(cfg.serverURL, "/")+"/api/agent-callback/heartbeat", bytes.NewReader(body))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+cfg.token)
			resp, err := http.DefaultClient.Do(req)
			if err == nil {
				_ = resp.Body.Close()
			}
		}
		time.Sleep(30 * time.Second)
	}
}

func bearerToken(c *gin.Context) string {
	header := c.GetHeader("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(header, "Bearer ")
}

func hostname() string {
	value, _ := os.Hostname()
	return value
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func isAbs(path string) bool {
	if runtime.GOOS == "windows" {
		return len(path) > 2 && path[1] == ':'
	}
	return strings.HasPrefix(path, "/")
}

func joinPath(base string, child string) string {
	if strings.HasSuffix(base, "/") || strings.HasSuffix(base, "\\") {
		return base + child
	}
	return base + string(os.PathSeparator) + child
}
