package logstream

import "sync"

type Event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

type Hub struct {
	mu          sync.RWMutex
	subscribers map[uint]map[chan Event]struct{}
}

func NewHub() *Hub {
	return &Hub{subscribers: map[uint]map[chan Event]struct{}{}}
}

func (h *Hub) Subscribe(taskRunID uint) chan Event {
	ch := make(chan Event, 64)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subscribers[taskRunID] == nil {
		h.subscribers[taskRunID] = map[chan Event]struct{}{}
	}
	h.subscribers[taskRunID][ch] = struct{}{}
	return ch
}

func (h *Hub) Unsubscribe(taskRunID uint, ch chan Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if subscribers := h.subscribers[taskRunID]; subscribers != nil {
		delete(subscribers, ch)
		close(ch)
		if len(subscribers) == 0 {
			delete(h.subscribers, taskRunID)
		}
	}
}

func (h *Hub) Publish(taskRunID uint, event Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.subscribers[taskRunID] {
		select {
		case ch <- event:
		default:
		}
	}
}
