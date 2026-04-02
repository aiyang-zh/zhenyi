package zroute

import (
	"testing"

	"github.com/aiyang-zh/zhenyi/zmodel"
	"github.com/aiyang-zh/zhenyi/zmsg"
)

func FuzzHRWScore_NoPanic(f *testing.F) {
	f.Add(uint64(1), uint64(2), uint64(3))
	f.Add(uint64(0), uint64(0), uint64(0))
	f.Add(uint64(0xffffffffffffffff), uint64(123), uint64(456))

	f.Fuzz(func(t *testing.T, key uint64, actorID uint64, process uint64) {
		_ = hrwScore(key, actorID, process)
	})
}

func FuzzRendezvousHashStrategy_PickOne_RealScore(f *testing.F) {
	s := &RendezvousHashStrategy{}

	f.Add(uint64(1), int32(1), int32(3))
	f.Add(uint64(0), int32(1), int32(0))
	f.Add(uint64(100), int32(0), int32(8))

	f.Fuzz(func(t *testing.T, sessionID uint64, rpcID int32, candCount int32) {
		if candCount < 0 {
			return
		}
		if candCount > 32 {
			candCount = 32
		}

		candidates := make([]zmodel.ActorConfig, 0, candCount)
		for i := int32(0); i < candCount; i++ {
			candidates = append(candidates, zmodel.ActorConfig{
				Id:      uint64(i + 1),
				Process: uint32((i % 3) + 1),
			})
		}

		msg := &zmsg.Message{SessionId: sessionID, RpcId: uint64(rpcID)}
		got := s.PickOne(msg, candidates)

		if len(candidates) == 0 {
			if got != -1 {
				t.Fatalf("expected -1 for empty candidates, got=%d", got)
			}
			return
		}
		if got < 0 || got >= len(candidates) {
			t.Fatalf("index out of range: got=%d len=%d", got, len(candidates))
		}

		// Verify returned index is the max-score candidate when key != 0.
		key := DefaultRemoteRouteKey(msg)
		if key == 0 {
			if got != 0 {
				t.Fatalf("expected fallback idx=0 for key==0, got=%d", got)
			}
			return
		}

		bestIdx := -1
		var bestScore uint64
		for i, c := range candidates {
			score := hrwScore(key, uint64(c.Id), uint64(c.Process))
			if bestIdx < 0 || score > bestScore {
				bestIdx = i
				bestScore = score
			}
		}
		if bestIdx != got {
			t.Fatalf("score mismatch: got=%d expected=%d", got, bestIdx)
		}
	})
}
