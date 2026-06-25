package node

import (
	"fmt"
	"sort"
)

type Registry struct {
	executors map[string]NodeExecutor
}

func NewRegistry() *Registry {
	return &Registry{executors: map[string]NodeExecutor{}}
}

func (r *Registry) Register(executor NodeExecutor) {
	r.executors[executor.Type()] = executor
}

func (r *Registry) Get(nodeType string) (NodeExecutor, bool) {
	executor, ok := r.executors[nodeType]
	return executor, ok
}

func (r *Registry) MustGet(nodeType string) (NodeExecutor, error) {
	executor, ok := r.Get(nodeType)
	if !ok {
		return nil, fmt.Errorf("unknown node type %q", nodeType)
	}
	return executor, nil
}

func (r *Registry) Metadata() []NodeMetadata {
	items := make([]NodeMetadata, 0, len(r.executors))
	for _, executor := range r.executors {
		items = append(items, executor.Metadata())
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Category == items[j].Category {
			return items[i].Type < items[j].Type
		}
		return items[i].Category < items[j].Category
	})
	return items
}
