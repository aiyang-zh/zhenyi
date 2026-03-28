package znats

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	natssrv "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"

	"github.com/aiyang-zh/zhenyi/zbus"
)

func runEmbeddedNATSServer(t *testing.T) (*natssrv.Server, string) {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen err: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()

	opts := &natssrv.Options{
		Host: "127.0.0.1",
		Port: port,
	}
	s, err := natssrv.NewServer(opts)
	if err != nil {
		t.Fatalf("new server err: %v", err)
	}
	go s.Start()
	if !s.ReadyForConnections(3 * time.Second) {
		s.Shutdown()
		t.Fatalf("nats server not ready")
	}
	return s, fmt.Sprintf("nats://127.0.0.1:%d", port)
}

func TestNats_Connect_Subscribe_Broadcast_Request_Unsubscribe_Close(t *testing.T) {
	srv, url := runEmbeddedNATSServer(t)
	defer srv.Shutdown()

	nc := NewNats(url)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := nc.Connect(ctx); err != nil {
		t.Fatalf("connect err: %v", err)
	}

	// SubscribeSync + Broadcast
	sub := nc.Subscribe("t.sync")
	if sub == nil {
		t.Fatalf("expected sync subscription")
	}
	if err := nc.Broadcast("t.sync", []byte("x")); err != nil {
		t.Fatalf("broadcast err: %v", err)
	}
	msg, err := sub.NextMsg(2 * time.Second)
	if err != nil {
		t.Fatalf("next msg err: %v", err)
	}
	if string(msg.Data) != "x" {
		t.Fatalf("data=%q", string(msg.Data))
	}

	// SubscribeChan
	ch := make(chan *nats.Msg, 1)
	sub2 := nc.SubscribeChan("t.chan", ch)
	if sub2 == nil {
		t.Fatalf("expected chan subscription")
	}
	if err := nc.Broadcast("t.chan", []byte("y")); err != nil {
		t.Fatalf("broadcast err: %v", err)
	}
	select {
	case m := <-ch:
		if string(m.Data) != "y" {
			t.Fatalf("data=%q", string(m.Data))
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting chan msg")
	}

	// SubscribeQueue
	subQ := nc.SubscribeQueue("t.queue", "q")
	if subQ == nil {
		t.Fatalf("expected queue subscription")
	}

	// SubscribeCall + Request/Reply
	var gotCb int
	subCb := nc.SubscribeCall("t.req", func(m *nats.Msg) {
		gotCb++
		_ = m.Respond([]byte("pong"))
	})
	if subCb == nil {
		t.Fatalf("expected callback subscription")
	}
	// Topic timeout overrides global timeout only after subscription exists
	nc.SetGlobalTimeout(5 * time.Second)
	nc.SetTopicTimeout("t.req", 500*time.Millisecond)
	if nc.getTimeout("t.req") != 500*time.Millisecond {
		t.Fatalf("timeout=%v", nc.getTimeout("t.req"))
	}
	resp, err := nc.Request("t.req", []byte("ping"))
	if err != nil {
		t.Fatalf("request err: %v", err)
	}
	if string(resp.Data) != "pong" {
		t.Fatalf("resp=%q", string(resp.Data))
	}
	if gotCb == 0 {
		t.Fatalf("expected callback invoked")
	}

	// GetSub
	if nc.GetSub("t.sync") == nil {
		t.Fatalf("expected GetSub returns sub")
	}
	if nc.GetSub("nope") != nil {
		t.Fatalf("expected nil for missing sub")
	}

	// UnSubscribe (existing + non-existing)
	if errTopics := nc.UnSubscribe("t.sync", "t.chan", "t.queue", "t.req", "does.not.exist"); len(errTopics) != 0 {
		t.Fatalf("unexpected unsubscribe errors: %v", errTopics)
	}
	if nc.GetSub("t.sync") != nil {
		t.Fatalf("expected removed from map")
	}

	// Close is safe
	nc.Close()
	nc.Close()
}

func TestNatsBus_SubscribeAndBroadcast_Integration(t *testing.T) {
	srv, url := runEmbeddedNATSServer(t)
	defer srv.Shutdown()

	nc := NewNats(url)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := nc.Connect(ctx); err != nil {
		t.Fatalf("connect err: %v", err)
	}

	pool := &NatsPool{clients: []*Nats{nc}}
	bus := NewNatsBus(pool)
	if bus == nil {
		t.Fatalf("expected bus")
	}

	var (
		mu   sync.Mutex
		gotT string
		gotD []byte
	)
	ready := make(chan struct{})
	sub, err := bus.Subscribe("t.bus", func(topic string, data []byte) {
		mu.Lock()
		gotT = topic
		gotD = append([]byte(nil), data...)
		mu.Unlock()
		close(ready)
	})
	if err != nil || sub == nil {
		t.Fatalf("subscribe err=%v sub=%v", err, sub)
	}
	defer func() { _ = sub.Unsubscribe() }()

	if err := bus.Broadcast("t.bus", []byte("hello")); err != nil {
		t.Fatalf("bus broadcast err: %v", err)
	}
	select {
	case <-ready:
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting handler")
	}
	mu.Lock()
	defer mu.Unlock()
	if gotT != "t.bus" || string(gotD) != "hello" {
		t.Fatalf("got topic=%q data=%q", gotT, string(gotD))
	}
}

func TestNatsSubscription_Unsubscribe_NilSafe(t *testing.T) {
	var s *natsSubscription
	if err := s.Unsubscribe(); err != nil {
		t.Fatalf("expected nil err")
	}
	s = &natsSubscription{sub: nil}
	if err := s.Unsubscribe(); err != nil {
		t.Fatalf("expected nil err")
	}
}

func TestNatsBus_NotInitializedBranches(t *testing.T) {
	var b *NatsBus
	if err := b.Broadcast("t", []byte("x")); err == nil {
		t.Fatalf("expected error")
	}
	if _, err := b.Subscribe("t", func(string, []byte) {}); err == nil {
		t.Fatalf("expected error")
	}

	// pool nil
	b = &NatsBus{pool: nil}
	if err := b.Broadcast("t", []byte("x")); err == nil {
		t.Fatalf("expected error")
	}
	if _, err := b.Subscribe("t", func(string, []byte) {}); err == nil {
		t.Fatalf("expected error")
	}

	// handler nil already covered elsewhere, but keep here for completeness
	b = NewNatsBus(&NatsPool{clients: []*Nats{NewNats(DefaultURL)}})
	if _, err := b.Subscribe("t", nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestTopicBusInterfaceCompliance(t *testing.T) {
	var _ zbus.TopicBus = (*NatsBus)(nil)
}
