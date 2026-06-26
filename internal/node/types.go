package node

import "context"

type NodeExecutor interface {
	Type() string
	Metadata() NodeMetadata
	Validate(params map[string]any) error
	Execute(ctx *NodeContext, params map[string]any) (*NodeResult, error)
}

type NodeMetadata struct {
	Type        string      `json:"type"`
	Name        string      `json:"name"`
	Category    string      `json:"category"`
	Description string      `json:"description"`
	SupportedOS []string    `json:"supportedOS"`
	Fields      []NodeField `json:"fields"`
}

type NodeField struct {
	Name     string              `json:"name"`
	Label    string              `json:"label"`
	Type     string              `json:"type"`
	Required bool                `json:"required"`
	Default  any                 `json:"default,omitempty"`
	Options  []string            `json:"options,omitempty"`
	Secret   bool                `json:"secret,omitempty"`
	ShowWhen *NodeFieldCondition `json:"showWhen,omitempty"`
}

type NodeFieldCondition struct {
	Field  string `json:"field"`
	Equals any    `json:"equals"`
}

type NodeContext struct {
	Context           context.Context
	TaskRunID         uint
	NodeRunID         uint
	Workspace         string
	Log               func(stream string, content string)
	ResolveCredential func(id uint) (*Credential, error)
}

type NodeResult struct {
	Output map[string]any `json:"output"`
}

type Credential struct {
	ID          uint
	Name        string
	Type        string
	Description string
	Username    string
	Secrets     map[string]string
}

// InputSource describes where a select input's options come from dynamically.
type InputSource struct {
	Type   string         `json:"type"`
	Params map[string]any `json:"params"`
}

type PipelineDefinition struct {
	Name          string          `json:"name"`
	StartNodeID   string          `json:"startNodeId,omitempty"`
	AgentSelector AgentSelector   `json:"agentSelector"`
	Inputs        []PipelineInput `json:"inputs"`
	Nodes         []PipelineNode  `json:"nodes"`
}

type AgentSelector struct {
	Labels []string `json:"labels"`
}

// PipelineInput defines a user-facing parameter shown before a task run.
// Type is one of: "string", "select", "boolean", "number".
// For select inputs: Options holds static choices; Source (if set) fetches
// choices dynamically and takes precedence over Options.
type PipelineInput struct {
	Name     string       `json:"name"`
	Label    string       `json:"label"`
	Type     string       `json:"type"`
	Required bool         `json:"required"`
	Default  any          `json:"default,omitempty"`
	Options  []string     `json:"options,omitempty"`
	Source   *InputSource `json:"source,omitempty"`
}

type PipelineNode struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	Type            string         `json:"type"`
	Params          map[string]any `json:"params"`
	TimeoutSeconds  int            `json:"timeoutSeconds"`
	RetryTimes      int            `json:"retryTimes"`
	NextNodeID      string         `json:"nextNodeId,omitempty"`
	FallbackNodeID  string         `json:"fallbackNodeId,omitempty"`
	ContinueOnError bool           `json:"continueOnError"`
}
