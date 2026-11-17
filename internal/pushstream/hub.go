package pushstream

import (
	"sync"
	"sync/atomic"

	"github.com/YN1753/vesper/bothub/internal/store"
)

type StreamEvent struct {
	Event string `json:"event"`
	Data  any    `json:"data"`
}

// Hub manages active SSE subscribers and broadcasts newly created notifications and bot events in real time.
type Hub struct {
	mu            sync.RWMutex
	clients       map[chan store.Notification]func()
	eventClients  map[chan StreamEvent]func()
	done          chan struct{}
	closed        bool
	droppedEvents atomic.Uint64
}

// DroppedCount returns the total number of SSE events dropped due to slow consumers.
func (h *Hub) DroppedCount() uint64 {
	return h.droppedEvents.Load()
}

func NewHub() *Hub {
	return &Hub{
		clients:      make(map[chan store.Notification]func()),
		eventClients: make(map[chan StreamEvent]func()),
		done:         make(chan struct{}),
	}
}

// Done returns a channel that is closed when the Hub is shutting down.
func (h *Hub) Done() <-chan struct{} {
	return h.done
}

// KickSubscribers disconnects all currently active SSE subscribers (forcing re-authentication)
// while keeping the Hub open for subsequent authenticated connections.
func (h *Hub) KickSubscribers() {
	h.mu.Lock()
	cancels := make([]func(), 0, len(h.clients)+len(h.eventClients))
	for _, cancel := range h.clients {
		cancels = append(cancels, cancel)
	}
	for _, cancel := range h.eventClients {
		cancels = append(cancels, cancel)
	}
	h.mu.Unlock()

	for _, cancel := range cancels {
		cancel()
	}
}

// CloseAll signals all active SSE subscribers to terminate cleanly during server shutdown.
func (h *Hub) CloseAll() {
	h.mu.Lock()
	if !h.closed {
		h.closed = true
		close(h.done)
	}
	h.mu.Unlock()
	h.KickSubscribers()
}

// Subscribe registers a new notification subscriber channel and returns an unsubscribe closure.
func (h *Hub) Subscribe() (<-chan store.Notification, func()) {
	ch := make(chan store.Notification, 64)
	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			h.mu.Lock()
			delete(h.clients, ch)
			close(ch)
			h.mu.Unlock()
		})
	}

	h.mu.Lock()
	if h.closed {
		close(ch)
		h.mu.Unlock()
		return ch, func() {}
	}
	h.clients[ch] = unsubscribe
	h.mu.Unlock()
	return ch, unsubscribe
}

// SubscribeEvents registers a general typed SSE event subscriber (notifications, bot_state, bot_message).
func (h *Hub) SubscribeEvents() (<-chan StreamEvent, func()) {
	ch := make(chan StreamEvent, 64)
	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			h.mu.Lock()
			delete(h.eventClients, ch)
			close(ch)
			h.mu.Unlock()
		})
	}

	h.mu.Lock()
	if h.closed {
		close(ch)
		h.mu.Unlock()
		return ch, func() {}
	}
	h.eventClients[ch] = unsubscribe
	h.mu.Unlock()
	return ch, unsubscribe
}

// PublishEvent fans out any typed SSE event to all connected SubscribeEvents listeners.
// When a slow subscriber's buffer is full, it evicts the oldest queued event so the latest state wins.
func (h *Hub) PublishEvent(event string, data any) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.closed {
		return
	}
	ev := StreamEvent{Event: event, Data: data}
	for ch := range h.eventClients {
		select {
		case ch <- ev:
		default:
			h.droppedEvents.Add(1)
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- ev:
			default:
			}
		}
	}
}

// Publish fans out a notification to all connected subscribers without blocking.
func (h *Hub) Publish(n store.Notification) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.closed {
		return
	}
	for ch := range h.clients {
		select {
		case ch <- n:
		default:
			h.droppedEvents.Add(1)
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- n:
			default:
			}
		}
	}
	ev := StreamEvent{Event: "notification", Data: n}
	for ch := range h.eventClients {
		select {
		case ch <- ev:
		default:
			h.droppedEvents.Add(1)
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- ev:
			default:
			}
		}
	}
}

// SubscriberCount returns the number of currently connected SSE listeners.
func (h *Hub) SubscriberCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients) + len(h.eventClients)
}
