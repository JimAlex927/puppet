package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"puppet/internal/agent"
	"puppet/internal/auth"
	"puppet/internal/config"
	"puppet/internal/confignode"
	"puppet/internal/credential"
	"puppet/internal/engine"
	"puppet/internal/logstream"
	"puppet/internal/model"
	"puppet/internal/node"
	"puppet/internal/project"
	"puppet/internal/runfiles"
	"puppet/internal/schedule"
	"puppet/internal/sharedfiles"
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
	cfg            config.Config
	projects       *project.Service
	tasks          *task.Service
	agents         *agent.Service
	creds          *credential.Service
	auths          *auth.Service
	sharedFiles    *sharedfiles.Service
	runFiles       *runfiles.Service
	schedules      *schedule.Service
}

type resolvedRunInput struct {
	Name     string   `json:"name"`
	Label    string   `json:"label"`
	Type     string   `json:"type"`
	Required bool     `json:"required"`
	Default  any      `json:"default,omitempty"`
	Options  []string `json:"options"`
	Multiple bool     `json:"multiple,omitempty"`
	Error    string   `json:"error,omitempty"`
}

type pageResult[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}

func NewRouter(db *gorm.DB, registry *node.Registry, configRegistry *confignode.Registry, runner *engine.Engine, hub *logstream.Hub, cfg config.Config) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(cors())

	sharedFiles, err := sharedfiles.NewService(db, cfg.SharedFilesDir, "/api/shared-file-uploads/")
	if err != nil {
		panic(fmt.Sprintf("initialize shared files: %v", err))
	}
	runFiles, err := runfiles.NewService(db, cfg.WorkspaceDir, "/api/task-run-file-uploads/")
	if err != nil {
		panic(fmt.Sprintf("initialize run files: %v", err))
	}

	agentSvc := agent.NewService(db)
	agentSvc.StartHeartbeatWatcher(context.Background())

	h := &Handler{
		db:             db,
		registry:       registry,
		configRegistry: configRegistry,
		engine:         runner,
		hub:            hub,
		cfg:            cfg,
		projects:       project.NewService(db),
		tasks:          task.NewService(db),
		agents:         agentSvc,
		creds:          credential.NewService(db),
		auths:          auth.NewService(db),
		sharedFiles:    sharedFiles,
		runFiles:       runFiles,
		schedules:      schedule.NewService(db, runner),
	}

	// Public webhook endpoint — no auth required, validated by per-task token.
	r.POST("/webhook/:token", h.webhookTrigger)

	r.GET("/health", func(c *gin.Context) {
		var count int64
		if err := db.Model(&model.Agent{}).Count(&count).Error; err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "error", "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api := r.Group("/api")
	{
		api.GET("/public/status", h.publicStatus)
		api.GET("/public/shared-files/:token/download", h.downloadSharedFileShare)

		api.POST("/auth/login", h.login)
		api.POST("/auth/logout", h.authMiddleware(), h.logout)
		api.GET("/auth/me", h.authMiddleware(), h.me)

		api.POST("/agent-callback/heartbeat", h.agentHeartbeat)
		api.POST("/agent-callback/node-runs/:id/logs", h.agentAppendLog)

		protected := api.Group("")
		protected.Use(h.authMiddleware())
		{
			protected.GET("/dashboard/summary", h.dashboardSummary)

			protected.GET("/schedules", h.listSchedules)
			protected.POST("/schedules", h.createSchedule)
			protected.GET("/schedules/:id", h.getSchedule)
			protected.PUT("/schedules/:id", h.updateSchedule)
			protected.DELETE("/schedules/:id", h.deleteSchedule)
			protected.POST("/schedules/:id/run", h.runScheduleNow)

			protected.GET("/shared-files", h.listSharedFiles)
			protected.POST("/shared-files/:id/share", h.createSharedFileShare)
			protected.DELETE("/shared-files/:id", h.deleteSharedFile)
			protected.GET("/shared-files/:id/download", h.downloadSharedFile)
			protected.Any("/shared-file-uploads", h.handleSharedFileUpload)
			protected.Any("/shared-file-uploads/", h.handleSharedFileUpload)
			protected.Any("/shared-file-uploads/:uploadID", h.handleSharedFileUpload)

			protected.GET("/projects", h.listProjects)
			protected.POST("/projects", h.createProject)
			protected.POST("/projects/import", h.importProject)
			protected.GET("/projects/:id", h.getProject)
			protected.PUT("/projects/:id", h.updateProject)
			protected.DELETE("/projects/:id", h.deleteProject)
			protected.GET("/projects/:id/export", h.exportProject)

			protected.GET("/projects/:id/tasks", h.listTasks)
			protected.POST("/projects/:id/tasks", h.createTask)
			protected.GET("/tasks/:id", h.getTask)
			protected.PUT("/tasks/:id", h.updateTask)
			protected.DELETE("/tasks/:id", h.deleteTask)

			protected.POST("/tasks/:id/webhook-token", h.generateWebhookToken)
			protected.DELETE("/tasks/:id/webhook-token", h.revokeWebhookToken)

			protected.GET("/tasks/:id/pipeline", h.getPipeline)
			protected.PUT("/tasks/:id/pipeline", h.updatePipeline)
			protected.GET("/node-types", h.nodeTypes)
			protected.GET("/config-node-types", h.configNodeTypes)
			protected.GET("/tasks/:id/run-config", h.getRunConfig)

			protected.POST("/tasks/:id/run", h.runTask)
			protected.POST("/tasks/:id/runs/prepare", h.prepareTaskRun)
			protected.GET("/tasks/:id/runs", h.listTaskRuns)
			protected.GET("/task-runs/:id", h.getTaskRun)
			protected.POST("/task-runs/:id/start", h.startTaskRun)
			protected.POST("/task-runs/:id/cancel", h.cancelTaskRun)
			protected.GET("/task-runs/:id/node-runs", h.listNodeRuns)
			protected.GET("/task-runs/:id/logs", h.listRunLogs)
			protected.GET("/task-runs/:id/events", h.taskRunEvents)
			protected.GET("/task-runs/:id/files", h.listTaskRunFiles)
			protected.GET("/task-runs/:id/files/download", h.downloadTaskRunFile)
			protected.POST("/task-runs/:id/file-bundles", h.createTaskRunFileBundle)
			protected.GET("/task-runs/:id/file-bundles/:bundle/download", h.downloadTaskRunFileBundle)
			protected.Any("/task-run-file-uploads", h.handleTaskRunFileUpload)
			protected.Any("/task-run-file-uploads/", h.handleTaskRunFileUpload)
			protected.Any("/task-run-file-uploads/:uploadID", h.handleTaskRunFileUpload)

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
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Tus-Resumable, Upload-Length, Upload-Metadata, Upload-Offset, Upload-Defer-Length, Upload-Concat")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS")
		c.Writer.Header().Set("Access-Control-Expose-Headers", "Location, Tus-Resumable, Upload-Offset, Upload-Length")
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

func pagination(c *gin.Context) (int, int, bool) {
	pageText := strings.TrimSpace(c.Query("page"))
	pageSizeText := strings.TrimSpace(c.Query("pageSize"))
	if pageText == "" && pageSizeText == "" {
		return 0, 0, false
	}
	page, err := strconv.Atoi(pageText)
	if err != nil || page < 1 {
		page = 1
	}
	pageSize, err := strconv.Atoi(pageSizeText)
	if err != nil || pageSize < 1 {
		pageSize = 12
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize, true
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

func (h *Handler) publicStatus(c *gin.Context) {
	var runningCount int64
	if err := h.db.Model(&model.TaskRun{}).
		Where("status IN ?", []string{model.TaskRunPending, model.TaskRunRunning}).
		Count(&runningCount).Error; err != nil {
		respond(c, nil, err)
		return
	}

	var latestSuccess model.TaskRun
	err := h.db.
		Where("status = ?", model.TaskRunSuccess).
		Order("finished_at desc, id desc").
		First(&latestSuccess).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		ok(c, gin.H{
			"healthy":         runningCount == 0,
			"runningCount":    runningCount,
			"latestSuccessAt": nil,
		})
		return
	}
	if err != nil {
		respond(c, nil, err)
		return
	}
	latestSuccessAt := latestSuccess.FinishedAt
	if latestSuccessAt == nil {
		latestSuccessAt = &latestSuccess.CreatedAt
	}

	ok(c, gin.H{
		"healthy":         runningCount == 0,
		"runningCount":    runningCount,
		"latestSuccessAt": latestSuccessAt,
	})
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

func (h *Handler) listSharedFiles(c *gin.Context) {
	files, err := h.sharedFiles.List()
	respond(c, files, err)
}

func (h *Handler) createSharedFileShare(c *gin.Context) {
	var req struct {
		ExpiresInMinutes int `json:"expiresInMinutes"`
	}
	_ = c.ShouldBindJSON(&req)
	var expiresAt *time.Time
	if req.ExpiresInMinutes > 0 {
		t := time.Now().Add(time.Duration(req.ExpiresInMinutes) * time.Minute)
		expiresAt = &t
	}
	createdBy := ""
	if current, exists := c.Get("user"); exists {
		if user, ok := current.(model.User); ok {
			createdBy = user.Username
		}
	}
	share, err := h.sharedFiles.CreateShare(paramID(c, "id"), expiresAt, createdBy)
	if err != nil {
		respond(c, nil, err)
		return
	}
	ok(c, gin.H{
		"token":     share.Token,
		"expiresAt": share.ExpiresAt,
		"url":       "/api/public/shared-files/" + share.Token + "/download",
	})
}

func (h *Handler) downloadSharedFile(c *gin.Context) {
	file, err := h.sharedFiles.Get(paramID(c, "id"))
	if err != nil {
		respond(c, nil, err)
		return
	}
	if _, err := os.Stat(file.StoragePath); err != nil {
		respond(c, nil, err)
		return
	}
	c.Header("Content-Type", file.ContentType)
	c.Header("Content-Disposition", "attachment; filename="+strconv.Quote(file.Name))
	c.File(file.StoragePath)
}

func (h *Handler) downloadSharedFileShare(c *gin.Context) {
	file, _, err := h.sharedFiles.FileByShareToken(c.Param("token"))
	if err != nil {
		fail(c, http.StatusNotFound, errors.New("share link is invalid or expired"))
		return
	}
	if _, err := os.Stat(file.StoragePath); err != nil {
		fail(c, http.StatusNotFound, errors.New("file not found"))
		return
	}
	c.Header("Content-Type", file.ContentType)
	c.Header("Content-Disposition", "attachment; filename="+strconv.Quote(file.Name))
	c.File(file.StoragePath)
}

func (h *Handler) deleteSharedFile(c *gin.Context) {
	respond(c, gin.H{"deleted": true}, h.sharedFiles.Delete(paramID(c, "id")))
}

func (h *Handler) handleSharedFileUpload(c *gin.Context) {
	username := ""
	if current, exists := c.Get("user"); exists {
		if user, ok := current.(model.User); ok {
			username = user.Username
		}
	}
	req := c.Request.WithContext(sharedfiles.WithUploadedBy(c.Request.Context(), username))
	h.sharedFiles.ServeUpload(c.Writer, req)
}

func (h *Handler) listProjects(c *gin.Context) {
	if page, pageSize, ok := pagination(c); ok {
		var total int64
		var projects []model.Project
		query := h.db.Model(&model.Project{})
		if err := query.Count(&total).Error; err != nil {
			respond(c, nil, err)
			return
		}
		err := query.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&projects).Error
		respond(c, pageResult[model.Project]{Items: projects, Total: total, Page: page, PageSize: pageSize}, err)
		return
	}
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

func (h *Handler) exportProject(c *gin.Context) {
	content, filename, err := h.projects.ExportArchive(paramID(c, "id"))
	if err != nil {
		respond(c, nil, err)
		return
	}
	c.Header("Content-Disposition", "attachment; filename="+strconv.Quote(filename))
	c.Data(http.StatusOK, "application/zip", content)
}

func (h *Handler) importProject(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		fail(c, http.StatusBadRequest, errors.New("file is required"))
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	defer file.Close()

	const maxArchiveSize = 25 * 1024 * 1024
	content, err := io.ReadAll(io.LimitReader(file, maxArchiveSize+1))
	if err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	if len(content) > maxArchiveSize {
		fail(c, http.StatusBadRequest, errors.New("project archive is too large"))
		return
	}

	project, err := h.projects.ImportArchive(content)
	respond(c, project, err)
}

func (h *Handler) listTasks(c *gin.Context) {
	if page, pageSize, ok := pagination(c); ok {
		projectID := paramID(c, "id")
		var total int64
		var tasks []model.Task
		query := h.db.Model(&model.Task{}).Where("project_id = ?", projectID)
		if err := query.Count(&total).Error; err != nil {
			respond(c, nil, err)
			return
		}
		err := query.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&tasks).Error
		respond(c, pageResult[model.Task]{Items: tasks, Total: total, Page: page, PageSize: pageSize}, err)
		return
	}
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

func (h *Handler) listSchedules(c *gin.Context) {
	schedules, err := h.schedules.List()
	respond(c, schedules, err)
}

func (h *Handler) createSchedule(c *gin.Context) {
	var req model.TaskSchedule
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	created, err := h.schedules.Create(req)
	respond(c, created, err)
}

func (h *Handler) getSchedule(c *gin.Context) {
	item, err := h.schedules.Get(paramID(c, "id"))
	respond(c, item, err)
}

func (h *Handler) updateSchedule(c *gin.Context) {
	var req model.TaskSchedule
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	updated, err := h.schedules.Update(paramID(c, "id"), req)
	respond(c, updated, err)
}

func (h *Handler) deleteSchedule(c *gin.Context) {
	respond(c, gin.H{"deleted": true}, h.schedules.Delete(paramID(c, "id")))
}

func (h *Handler) runScheduleNow(c *gin.Context) {
	run, err := h.schedules.RunNow(c.Request.Context(), paramID(c, "id"))
	respond(c, run, err)
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

func (h *Handler) generateWebhookToken(c *gin.Context) {
	var task model.Task
	if err := h.db.First(&task, paramID(c, "id")).Error; err != nil {
		respond(c, nil, err)
		return
	}
	token, err := generateToken()
	if err != nil {
		fail(c, http.StatusInternalServerError, err)
		return
	}
	task.WebhookToken = token
	if err := h.db.Save(&task).Error; err != nil {
		respond(c, nil, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token})
}

func (h *Handler) revokeWebhookToken(c *gin.Context) {
	var task model.Task
	if err := h.db.First(&task, paramID(c, "id")).Error; err != nil {
		respond(c, nil, err)
		return
	}
	task.WebhookToken = ""
	err := h.db.Save(&task).Error
	respond(c, gin.H{"ok": true}, err)
}

func (h *Handler) webhookTrigger(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		fail(c, http.StatusBadRequest, errors.New("missing token"))
		return
	}
	var task model.Task
	if err := h.db.Where("webhook_token = ?", token).First(&task).Error; err != nil {
		fail(c, http.StatusUnauthorized, errors.New("invalid webhook token"))
		return
	}
	var req struct {
		Input map[string]any `json:"input"`
	}
	_ = c.ShouldBindJSON(&req)
	pipeline, err := h.tasks.Pipeline(task.ID)
	if err != nil {
		respond(c, nil, err)
		return
	}
	input, err := h.normalizeRunInput(c.Request.Context(), pipeline, req.Input, false)
	if err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	run, err := h.engine.StartTask(c.Request.Context(), task.ID, "webhook", "webhook", input)
	respond(c, run, err)
}

func generateToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
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
	input, err := h.normalizeRunInput(c.Request.Context(), pipeline, req.Input, false)
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

func (h *Handler) prepareTaskRun(c *gin.Context) {
	var req struct {
		Input map[string]any `json:"input"`
	}
	_ = c.ShouldBindJSON(&req)
	taskID := paramID(c, "id")
	pipeline, err := h.tasks.Pipeline(taskID)
	if err != nil {
		respond(c, nil, err)
		return
	}
	input, err := h.normalizeRunInput(c.Request.Context(), pipeline, req.Input, true)
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
	run, err := h.engine.PrepareTaskRun(c.Request.Context(), taskID, "manual", triggeredBy, input)
	respond(c, run, err)
}

func (h *Handler) startTaskRun(c *gin.Context) {
	runID := paramID(c, "id")
	if err := h.validatePreparedFileInputs(runID); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	run, err := h.engine.StartPreparedTaskRun(c.Request.Context(), runID)
	respond(c, run, err)
}

func (h *Handler) handleTaskRunFileUpload(c *gin.Context) {
	h.runFiles.ServeUpload(c.Writer, c.Request)
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
			Multiple: input.Multiple,
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

func (h *Handler) normalizeRunInput(ctx context.Context, pipeline node.PipelineDefinition, input map[string]any, allowPendingFiles bool) (map[string]any, error) {
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
			if allowPendingFiles && item.Type == "file" {
				if item.Multiple {
					normalized[item.Name] = []string{}
				} else {
					normalized[item.Name] = ""
				}
				continue
			}
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
	if inputType == "file" {
		switch typed := value.(type) {
		case []any:
			return len(typed) == 0
		case []string:
			return len(typed) == 0
		}
	}
	if inputType == "boolean" || inputType == "number" {
		return false
	}
	return strings.TrimSpace(fmt.Sprint(value)) == ""
}

func (h *Handler) validatePreparedFileInputs(runID uint) error {
	var run model.TaskRun
	if err := h.db.First(&run, runID).Error; err != nil {
		return err
	}
	var pipeline node.PipelineDefinition
	if err := json.Unmarshal([]byte(run.PipelineSnapshotJSON), &pipeline); err != nil {
		return err
	}
	input := map[string]any{}
	if strings.TrimSpace(run.InputJSON) != "" {
		_ = json.Unmarshal([]byte(run.InputJSON), &input)
	}
	for _, item := range pipeline.Inputs {
		if item.Type != "file" || !item.Required {
			continue
		}
		value, exists := input[item.Name]
		if inputIsEmpty(value, item.Type, exists) {
			return fmt.Errorf("input %q is required", item.Name)
		}
	}
	return nil
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

func (h *Handler) cancelTaskRun(c *gin.Context) {
	id := paramID(c, "id")
	var run model.TaskRun
	if err := h.db.First(&run, id).Error; err != nil {
		respond(c, nil, err)
		return
	}
	if taskRunDone(run.Status) {
		ok(c, run)
		return
	}

	h.engine.CancelTaskRun(run.ID)

	now := time.Now()
	var sequence int
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		run.Status = model.TaskRunCanceled
		run.FinishedAt = &now
		if run.StartedAt != nil {
			run.DurationMS = now.Sub(*run.StartedAt).Milliseconds()
		}
		run.ErrorMessage = "canceled by user"
		if err := tx.Save(&run).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.NodeRun{}).
			Where("task_run_id = ? AND status IN ?", run.ID, []string{model.NodeRunPending, model.NodeRunRunning}).
			Updates(map[string]any{
				"status":        model.NodeRunCanceled,
				"finished_at":   now,
				"error_message": "canceled by user",
			}).Error; err != nil {
			return err
		}
		sequence = nextLogSequence(tx, run.ID)
		return tx.Create(&model.RunLog{
			TaskRunID: run.ID,
			Sequence:  sequence,
			Stream:    "system",
			Content:   "[task:canceled] canceled by user\n",
		}).Error
	}); err != nil {
		respond(c, nil, err)
		return
	}

	h.hub.Publish(run.ID, logstream.Event{
		Type: "log",
		Data: map[string]any{
			"taskRunId": run.ID,
			"nodeRunId": 0,
			"stream":    "system",
			"content":   "[task:canceled] canceled by user\n",
			"sequence":  sequence,
		},
	})
	h.hub.Publish(run.ID, logstream.Event{
		Type: "task_status",
		Data: map[string]any{
			"taskRunId": run.ID,
			"status":    run.Status,
			"run":       run,
		},
	})
	ok(c, run)
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
		} else if errors.Is(err, schedule.ErrInvalidCronSchedule) {
			status = http.StatusBadRequest
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

func taskRunDone(status string) bool {
	switch status {
	case model.TaskRunSuccess, model.TaskRunFailed, model.TaskRunCanceled, model.TaskRunTimeout:
		return true
	default:
		return false
	}
}

func nextLogSequence(db *gorm.DB, taskRunID uint) int {
	var max int
	_ = db.Model(&model.RunLog{}).
		Where("task_run_id = ?", taskRunID).
		Select("COALESCE(MAX(sequence), 0)").
		Scan(&max).Error
	return max + 1
}
