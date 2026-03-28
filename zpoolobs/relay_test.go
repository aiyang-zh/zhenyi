package zpoolobs

import (
	"sync/atomic"
	"testing"
)

type countRelayObs struct {
	get atomic.Int32
}

func (c *countRelayObs) OnPoolCreate(string) {}
func (c *countRelayObs) OnNew(string)        {}
func (c *countRelayObs) OnGet(string)        { c.get.Add(1) }
func (c *countRelayObs) OnPut(string)        {}
func (c *countRelayObs) OnPutNil(string)     {}

func TestPoolRelay_LateSetObserver(t *testing.T) {
	t.Cleanup(func() { SetObserver(nil) })
	SetObserver(nil)

	p := NewObservedPool("relay.late.test", func() int { return 0 })
	var obs countRelayObs
	SetObserver(&obs)

	_ = p.Get()
	if obs.get.Load() != 1 {
		t.Fatalf("expected OnGet after late SetObserver, got %d", obs.get.Load())
	}
}
