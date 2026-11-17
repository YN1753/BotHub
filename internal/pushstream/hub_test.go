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

	// Fill channel buffer so slow-consumer drop counter increments and latest event is retained
	for i := 0; i < 200; i++ {
		hub.PublishEvent("burst", i)
	}
	if hub.DroppedCount() == 0 {
		t.Fatal("expected non-zero dropped events on full subscriber buffer")
	}
	var lastVal int
	for len(ch) > 0 {
		ev := <-ch
		lastVal, _ = ev.Data.(int)
	}
	if lastVal != 199 {
		t.Fatalf("expected oldest-eviction to retain latest event 199, got %d", lastVal)
	}

	// KickSubscribers disconnects active subscribers without permanently closing Hub
	hub.KickSubscribers()
	if _, ok := <-ch; ok {
		t.Fatal("expected subscriber channel closed after KickSubscribers")
	}
	unsub() // idempotent after kick
	ch2, unsub2 := hub.SubscribeEvents()
	hub.PublishEvent("post_kick", "alive")
	if ev := <-ch2; ev.Event != "post_kick" {
		t.Fatalf("expected Hub to remain usable after KickSubscribers, got %+v", ev)
	}
	unsub2()
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
