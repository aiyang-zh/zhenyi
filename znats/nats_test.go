package znats

import (
	"context"
	"testing"
	"time"
)

func TestNatsPoolBroadcast_NoClients(t *testing.T) {
	p := &NatsPool{clients: nil}
	if err := p.Broadcast("t", []byte("x")); err == nil {
		t.Fatalf("expected error when no clients")
	}
}

func TestNewNatsBus_Validation(t *testing.T) {
	if NewNatsBus(nil) != nil {
		t.Fatalf("expected nil bus when pool is nil")
	}
}

func TestNatsBus_Subscribe_RejectsNilHandler(t *testing.T) {
	bus := NewNatsBus(&NatsPool{clients: []*Nats{NewNats(DefaultURL)}})
	if _, err := bus.Subscribe("t", nil); err == nil {
		t.Fatalf("expected error for nil handler")
	}
}

func TestNatsBroadcast_NoConn(t *testing.T) {
	nc := NewNats(DefaultURL)
	if err := nc.Broadcast("t", []byte("x")); err == nil {
		t.Fatalf("expected error when conn is nil")
	}
}

func TestNatsRequest_NoConn(t *testing.T) {
	nc := NewNats(DefaultURL)
	if _, err := nc.Request("t", []byte("x")); err == nil {
		t.Fatalf("expected error when conn is nil")
	}
}

func TestNatsConnect_Cancelled(t *testing.T) {
	nc := NewNats("nats://127.0.0.1:1")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := nc.Connect(ctx); err == nil {
		t.Fatalf("expected error on cancelled context")
	}
}

func BenchmarkHrwLikeTopicTimeoutLookup(b *testing.B) {
	nc := NewNats(DefaultURL)
	nc.SetGlobalTimeout(123 * time.Millisecond)
	nc.SetTopicTimeout("t", 456*time.Millisecond) // no-op until sub exists
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = nc.getTimeout("t")
	}
}
