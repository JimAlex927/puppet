package sleep

import (
	"fmt"
	"time"

	"puppet/internal/node"
)

type Executor struct{}

func New() *Executor {
	return &Executor{}
}

func (e *Executor) Type() string {
	return "sleep"
}

func (e *Executor) Metadata() node.NodeMetadata {
	return node.NodeMetadata{
		Type:        e.Type(),
		Name:        "Sleep",
		Category:    "control",
		Description: "等待指定秒数",
		SupportedOS: []string{"linux", "darwin", "windows"},
		Fields: []node.NodeField{
			{Name: "seconds", Label: "Seconds", Type: "number", Required: true, Default: 2},
		},
	}
}

func (e *Executor) Validate(params map[string]any) error {
	if secondsFrom(params["seconds"]) <= 0 {
		return fmt.Errorf("seconds must be greater than 0")
	}
	return nil
}

func (e *Executor) Execute(ctx *node.NodeContext, params map[string]any) (*node.NodeResult, error) {
	seconds := secondsFrom(params["seconds"])
	ctx.Log("stdout", fmt.Sprintf("sleep %d seconds\n", seconds))
	timer := time.NewTimer(time.Duration(seconds) * time.Second)
	defer timer.Stop()

	select {
	case <-ctx.Context.Done():
		return nil, ctx.Context.Err()
	case <-timer.C:
		ctx.Log("stdout", "sleep finished\n")
		return &node.NodeResult{Output: map[string]any{"seconds": seconds}}, nil
	}
}

func secondsFrom(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case string:
		var seconds int
		_, _ = fmt.Sscanf(typed, "%d", &seconds)
		return seconds
	default:
		return 0
	}
}
