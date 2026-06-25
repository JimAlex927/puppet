package confignode

import (
	"context"
	"fmt"
	"sort"

	"puppet/internal/node"
)

type Executor interface {
	Type() string
	Metadata() node.NodeMetadata
	Validate(params map[string]any) error
	Execute(ctx Context, params map[string]any) (Result, error)
}

type Context struct {
	Context           context.Context
	ResolveCredential func(id uint) (*node.Credential, error)
}

type Result struct {
	Output map[string]any `json:"output"`
}

type Registry struct {
	executors map[string]Executor
}

func NewRegistry() *Registry {
	return &Registry{executors: map[string]Executor{}}
}

func (r *Registry) Register(executor Executor) {
	r.executors[executor.Type()] = executor
}

func (r *Registry) MustGet(nodeType string) (Executor, error) {
	executor, ok := r.executors[nodeType]
	if !ok {
		return nil, fmt.Errorf("unknown config node type %q", nodeType)
	}
	return executor, nil
}

func (r *Registry) Metadata() []node.NodeMetadata {
	items := make([]node.NodeMetadata, 0, len(r.executors))
	for _, executor := range r.executors {
		items = append(items, executor.Metadata())
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Type < items[j].Type })
	return items
}
