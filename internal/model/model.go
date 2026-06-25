package model

import "time"

const (
	TaskRunPending  = "pending"
	TaskRunRunning  = "running"
	TaskRunSuccess  = "success"
	TaskRunFailed   = "failed"
	TaskRunCanceled = "canceled"
	TaskRunTimeout  = "timeout"

	NodeRunPending  = "pending"
	NodeRunRunning  = "running"
	NodeRunSuccess  = "success"
	NodeRunFailed   = "failed"
	NodeRunSkipped  = "skipped"
	NodeRunCanceled = "canceled"
	NodeRunTimeout  = "timeout"
)

type Project struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"not null"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Task struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	ProjectID       uint      `json:"projectId" gorm:"index;not null"`
	Name            string    `json:"name" gorm:"not null"`
	Description     string    `json:"description"`
	PipelineJSON    string    `json:"pipelineJson" gorm:"type:text;not null"`
	AllowConcurrent bool      `json:"allowConcurrent"`
	TimeoutSeconds  int       `json:"timeoutSeconds"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type TaskRun struct {
	ID                   uint       `json:"id" gorm:"primaryKey"`
	ProjectID            uint       `json:"projectId" gorm:"index;not null"`
	TaskID               uint       `json:"taskId" gorm:"index;not null"`
	Status               string     `json:"status" gorm:"index;not null"`
	TriggerType          string     `json:"triggerType"`
	TriggeredBy          string     `json:"triggeredBy"`
	InputJSON            string     `json:"inputJson" gorm:"type:text"`
	PipelineSnapshotJSON string     `json:"pipelineSnapshotJson" gorm:"type:text;not null"`
	StartedAt            *time.Time `json:"startedAt"`
	FinishedAt           *time.Time `json:"finishedAt"`
	DurationMS           int64      `json:"durationMs"`
	ErrorMessage         string     `json:"errorMessage"`
	CreatedAt            time.Time  `json:"createdAt"`
}

type NodeRun struct {
	ID                 uint       `json:"id" gorm:"primaryKey"`
	TaskRunID          uint       `json:"taskRunId" gorm:"index;not null"`
	AgentID            uint       `json:"agentId" gorm:"index"`
	NodeID             string     `json:"nodeId" gorm:"index;not null"`
	NodeName           string     `json:"nodeName"`
	NodeType           string     `json:"nodeType" gorm:"index;not null"`
	Status             string     `json:"status" gorm:"index;not null"`
	NodeIndex          int        `json:"nodeIndex"`
	ParamsSnapshotJSON string     `json:"paramsSnapshotJson" gorm:"type:text"`
	OutputJSON         string     `json:"outputJson" gorm:"type:text"`
	StartedAt          *time.Time `json:"startedAt"`
	FinishedAt         *time.Time `json:"finishedAt"`
	DurationMS         int64      `json:"durationMs"`
	ErrorMessage       string     `json:"errorMessage"`
	RetryCount         int        `json:"retryCount"`
	AssignedAt         *time.Time `json:"assignedAt"`
	CreatedAt          time.Time  `json:"createdAt"`
}

type RunLog struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	TaskRunID uint      `json:"taskRunId" gorm:"index;not null"`
	NodeRunID uint      `json:"nodeRunId" gorm:"index"`
	Sequence  int       `json:"sequence" gorm:"index;not null"`
	Stream    string    `json:"stream"`
	Content   string    `json:"content" gorm:"type:text;not null"`
	CreatedAt time.Time `json:"createdAt"`
}

type Agent struct {
	ID              uint       `json:"id" gorm:"primaryKey"`
	Name            string     `json:"name" gorm:"not null;uniqueIndex"`
	TokenHash       string     `json:"-" gorm:"index"`
	TokenSecret     string     `json:"-" gorm:"type:text"`
	EndpointURL     string     `json:"endpointUrl"`
	OS              string     `json:"os"`
	Arch            string     `json:"arch"`
	Hostname        string     `json:"hostname"`
	LabelsJSON      string     `json:"labelsJson" gorm:"type:text"`
	Status          string     `json:"status" gorm:"index;not null"`
	LastHeartbeatAt *time.Time `json:"lastHeartbeatAt"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

type User struct {
	ID           uint       `json:"id" gorm:"primaryKey"`
	Username     string     `json:"username" gorm:"not null;uniqueIndex"`
	DisplayName  string     `json:"displayName"`
	Role         string     `json:"role" gorm:"index;not null"`
	PasswordHash string     `json:"-"`
	Status       string     `json:"status" gorm:"index;not null"`
	LastLoginAt  *time.Time `json:"lastLoginAt"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

type Session struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    uint      `json:"userId" gorm:"index;not null"`
	TokenHash string    `json:"-" gorm:"uniqueIndex;not null"`
	ExpiresAt time.Time `json:"expiresAt" gorm:"index;not null"`
	CreatedAt time.Time `json:"createdAt"`
}

type Credential struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"not null;uniqueIndex"`
	Type        string    `json:"type" gorm:"index;not null"`
	Description string    `json:"description"`
	Username    string    `json:"username"`
	SecretJSON  string    `json:"-" gorm:"type:text"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}
