package mas

import (
	"context"
	"testing"
	"time"
)

func TestMessageBusPublishDirectAndBroadcast(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	bus := NewMessageBus(2)
	defer bus.Close()

	alice, err := bus.Subscribe("alice")
	if err != nil {
		t.Fatal(err)
	}
	bob, err := bus.Subscribe("bob")
	if err != nil {
		t.Fatal(err)
	}

	if err := bus.Publish(ctx, Message{From: "alice", To: "bob", Content: "hello"}); err != nil {
		t.Fatal(err)
	}
	assertMessage(t, bob, "alice", "hello")

	if err := bus.Publish(ctx, Message{From: "alice", To: "*", Content: "broadcast"}); err != nil {
		t.Fatal(err)
	}
	assertMessage(t, bob, "alice", "broadcast")

	select {
	case msg := <-alice:
		t.Fatalf("sender should not receive own broadcast, got %+v", msg)
	case <-time.After(20 * time.Millisecond):
	}
}

func TestRunBusAgent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	bus := NewMessageBus(4)
	defer bus.Close()

	client, err := bus.Subscribe("client")
	if err != nil {
		t.Fatal(err)
	}
	err = RunBusAgent(ctx, bus, "echo", func(ctx context.Context, msg Message) (*Message, error) {
		return &Message{To: msg.From, Content: "echo:" + msg.Content}, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := bus.Publish(ctx, Message{From: "client", To: "echo", Content: "ping"}); err != nil {
		t.Fatal(err)
	}
	assertMessage(t, client, "echo", "echo:ping")
}

func assertMessage(t *testing.T, ch <-chan Message, from, content string) {
	t.Helper()
	select {
	case msg := <-ch:
		if msg.From != from || msg.Content != content {
			t.Fatalf("message=%+v, want from=%q content=%q", msg, from, content)
		}
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for message")
	}
}
