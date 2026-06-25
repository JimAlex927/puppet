package agentproto

import "puppet/internal/node"

type ExecuteNodeRequest struct {
	TaskRunID   uint              `json:"taskRunId"`
	NodeRunID   uint              `json:"nodeRunId"`
	Workspace   string            `json:"workspace"`
	ServerURL   string            `json:"serverUrl"`
	Node        node.PipelineNode `json:"node"`
	Credentials []node.Credential `json:"credentials"`
}

type ExecuteNodeResponse struct {
	Status       string         `json:"status"`
	Output       map[string]any `json:"output"`
	ErrorMessage string         `json:"errorMessage"`
	DurationMS   int64          `json:"durationMs"`
}

type LogRequest struct {
	TaskRunID uint   `json:"taskRunId"`
	Stream    string `json:"stream"`
	Content   string `json:"content"`
}
