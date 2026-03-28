package zroute

import (
	"testing"

	"github.com/aiyang-zh/zhenyi/zmodel"
	"github.com/aiyang-zh/zhenyi/zmsg"
)

func TestDefaultRemoteRouteKey(t *testing.T) {
	if got := DefaultRemoteRouteKey(nil); got != 0 {
		t.Fatalf("nil msg got %d", got)
	}

	m := &zmsg.Message{SessionId: 9, RpcId: 11}
	if got := DefaultRemoteRouteKey(m); got != 9 {
		t.Fatalf("SessionId priority got %d", got)
	}
	m = &zmsg.Message{SessionId: 0, RpcId: 11}
	if got := DefaultRemoteRouteKey(m); got != 11 {
		t.Fatalf("RpcId priority got %d", got)
	}
}

func TestRoundRobinStrategyPickOne(t *testing.T) {
	s := &RoundRobinStrategy{}
	cands := []zmodel.ActorConfig{{Id: 1}, {Id: 2}, {Id: 3}}

	i1 := s.PickOne(nil, cands)
	i2 := s.PickOne(nil, cands)
	if i1 == i2 {
		t.Fatalf("expected rotating pick, got same idx=%d", i1)
	}
	if i1 < 0 || i1 >= len(cands) || i2 < 0 || i2 >= len(cands) {
		t.Fatalf("index out of range: %d %d", i1, i2)
	}
}

func TestRendezvousHashStrategyDeterministicPick(t *testing.T) {
	s := &RendezvousHashStrategy{}
	msg := &zmsg.Message{SessionId: 1}
	cands := []zmodel.ActorConfig{{Id: 1, Process: 1}, {Id: 2, Process: 1}, {Id: 3, Process: 2}}

	i1 := s.PickOne(msg, cands)
	i2 := s.PickOne(msg, cands)
	if i1 != i2 {
		t.Fatalf("expected deterministic pick, got %d vs %d", i1, i2)
	}
	if i1 < 0 || i1 >= len(cands) {
		t.Fatalf("index out of range: %d", i1)
	}
}

func TestFirstCandidateStrategyPickOne(t *testing.T) {
	s := FirstCandidateStrategy{}
	cands := []zmodel.ActorConfig{{Id: 1}, {Id: 2}}
	idx := s.PickOne(&zmsg.Message{SessionId: 1}, cands)
	if idx != 0 {
		t.Fatalf("expected idx 0, got %d", idx)
	}
}

func BenchmarkHRWScore(b *testing.B) {
	var sink uint64
	for i := 0; i < b.N; i++ {
		sink ^= hrwScore(123, 456, 789)
	}
	_ = sink
}
