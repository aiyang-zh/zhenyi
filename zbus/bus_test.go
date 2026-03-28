package zbus

import "testing"

type memSub struct{ unsubscribed bool }

func (s *memSub) Unsubscribe() error {
	s.unsubscribed = true
	return nil
}

type memBus struct {
	subs map[string][]Handler
}

func newMemBus() *memBus { return &memBus{subs: make(map[string][]Handler)} }

func (b *memBus) Broadcast(topic string, data []byte) error {
	for _, h := range b.subs[topic] {
		h(topic, data)
	}
	return nil
}

func (b *memBus) Subscribe(topic string, handler Handler) (Subscription, error) {
	b.subs[topic] = append(b.subs[topic], handler)
	return &memSub{}, nil
}

func TestDefaultBusCanBeInjected(t *testing.T) {
	b := newMemBus()
	DefaultBus = b
	if DefaultBus == nil {
		t.Fatalf("DefaultBus should not be nil after injection")
	}
}

func BenchmarkMemBusBroadcast(b *testing.B) {
	bus := newMemBus()
	_, _ = bus.Subscribe("t", func(_ string, _ []byte) {})
	payload := []byte("hello")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = bus.Broadcast("t", payload)
	}
}
