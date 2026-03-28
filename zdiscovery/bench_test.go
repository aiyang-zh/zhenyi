package zdiscovery

import (
	"testing"

	"github.com/aiyang-zh/zhenyi/zmodel"
)

func BenchmarkNoop_FindPoll(b *testing.B) {
	d := NewNoopDiscovery()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		d.FindPoll("/servers/1")
	}
}

func BenchmarkNoop_FindRandom(b *testing.B) {
	d := NewNoopDiscovery()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		d.FindRandom("/servers/1")
	}
}

func BenchmarkNoop_FindMod(b *testing.B) {
	d := NewNoopDiscovery()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		d.FindMod(1, uint64(i))
	}
}

func BenchmarkCloneCache(b *testing.B) {
	m := map[uint32][]zmodel.ActorConfig{
		1: make([]zmodel.ActorConfig, 64),
		2: make([]zmodel.ActorConfig, 64),
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = cloneCache(m)
	}
}
