package pushstream

import (
	"sync"

	"github.com/YN1753/vesper/bothub/internal/store"
)

// Hub manages active SSE subscribers and broadcasts newly created notifications in real time.
type Hub struct {
	mu      sync.RWMutex
	clients map[chan store.Notification]struct{}
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[chan store.Notification]struct{}),
	}
}

// Subscribe registers a new subscriber channel and returns an unsubscribe closure.
func (h *Hub) Subscribe() (<-chan store.Notification, func()) {
	ch := make(chan store.Notification, 64)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			h.mu.Lock()
			delete(h.clients, ch)
			close(ch)
			h.mu.Unlock()
		})
	}
	return ch, unsubscribe
}

// Publish fans out a notification to all connected subscribers without blocking.
func (h *Hub) Publish(n store.Notification) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.clients {
		select {
		case ch <- n:
		default:
			// Drop if subscriber buffer is full; client can catch up via after_id
		}
	}
}

// SubscriberCount returns the number of currently connected SSE listeners.
func (h *Hub) SubscriberCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
