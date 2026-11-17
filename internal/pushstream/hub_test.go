package pushstream

import "testing"

func TestHubPubSubDropAndClose(t *testing.T) {
	hub := NewHub()
	ch, unsub := hub.SubscribeEvents()
	if hub.SubscriberCount() != 1 {
		t.Fatalf("expected 1 subscriber, got %d", hub.SubscriberCount())
	}

	hub.PublishEvent("assistant_update", map[string]any{"kind": "todos"})
	select {
	case ev := <-ch:
		if ev.Event != "assistant_update" {
			t.Fatalf("unexpected event name: %q", ev.Event)
		}
	default:
		t.Fatal("expected published event in subscriber channel")
	}

	// Fill channel buffer so slow-consumer drop counter increments deterministically
	for i := 0; i < 200; i++ {
		hub.PublishEvent("burst", i)
	}
	if hub.DroppedCount() == 0 {
		t.Fatal("expected non-zero dropped events on full subscriber buffer")
	}

	unsub()
	if hub.SubscriberCount() != 0 {
		t.Fatalf("expected 0 subscribers after unsubscribe, got %d", hub.SubscriberCount())
	}

	hub.CloseAll()
	select {
	case <-hub.Done():
	default:
		t.Fatal("expected Done channel closed after CloseAll")
	}
	// Publishing after CloseAll must safely no-op without incrementing drops
	beforeDrops := hub.DroppedCount()
	hub.PublishEvent("after_close", true)
	if hub.DroppedCount() != beforeDrops {
		t.Fatal("expected no-op publish after CloseAll")
	}
}
