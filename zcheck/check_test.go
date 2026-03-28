package zcheck

import (
	"testing"

	"github.com/aiyang-zh/zhenyi/zbus"
	"github.com/aiyang-zh/zhenyi/znats"
)

type noopBus struct{}

func (noopBus) Broadcast(string, []byte) error                            { return nil }
func (noopBus) Subscribe(string, zbus.Handler) (zbus.Subscription, error) { return nil, nil }

func TestValidate_RequireRemoteBus(t *testing.T) {
	prev := zbus.DefaultBus
	defer func() { zbus.DefaultBus = prev }()
	zbus.DefaultBus = nil

	if err := Validate(Config{RequireRemoteBus: true}); err == nil {
		t.Fatal("expected error")
	}
	zbus.DefaultBus = noopBus{}
	if err := Validate(Config{RequireRemoteBus: true}); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestValidate_RequireNatsPool(t *testing.T) {
	prevBus, prevPool := zbus.DefaultBus, znats.DefaultNatsClient
	defer func() {
		zbus.DefaultBus = prevBus
		znats.DefaultNatsClient = prevPool
	}()
	zbus.DefaultBus = nil
	znats.DefaultNatsClient = nil

	if err := Validate(Config{RequireNatsPool: true}); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidate_TouchMetrics(t *testing.T) {
	if err := Validate(Config{TouchMetricsRegistry: true}); err != nil {
		t.Fatal(err)
	}
}
