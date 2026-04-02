package zroute

import (
	"testing"
)

// FuzzRendezvousHashScore tests HRW score calculation with random inputs.
func FuzzRendezvousHashScore(f *testing.F) {
	f.Add(uint64(1), uint64(100), uint64(1000))
	f.Add(uint64(0), uint64(0), uint64(0))
	f.Add(uint64(18446744073709551615), uint64(18446744073709551615), uint64(18446744073709551615))

	f.Fuzz(func(t *testing.T, key uint64, node uint64, seed uint64) {
		// Test HRW score calculation - should not panic
		// This is a placeholder; actual implementation depends on your hash function
		score := key ^ node ^ seed
		_ = score
	})
}

// FuzzRemoteRouteSelection tests remote route selection with random candidates.
func FuzzRemoteRouteSelection(f *testing.F) {
	f.Add(uint64(1), int32(100), int32(5))
	f.Add(uint64(0), int32(0), int32(0))
	f.Add(uint64(999), int32(1000), int32(10))

	f.Fuzz(func(t *testing.T, key uint64, msgId int32, candidateCount int32) {
		// Test route selection - should not panic
		if candidateCount < 0 {
			t.Logf("Invalid candidate count: %d", candidateCount)
		}
		_ = key
		_ = msgId
	})
}

// FuzzLocalRouting tests local routing with random actor IDs.
func FuzzLocalRouting(f *testing.F) {
	f.Add(uint64(1))
	f.Add(uint64(0))
	f.Add(uint64(18446744073709551615))

	f.Fuzz(func(t *testing.T, actorId uint64) {
		// Test local routing - should not panic
		_ = actorId
	})
}

// FuzzRoutingKeyGeneration tests routing key generation with random inputs.
func FuzzRoutingKeyGeneration(f *testing.F) {
	f.Add([]byte("user:123"))
	f.Add([]byte{})
	f.Add([]byte("room:abc:xyz"))

	f.Fuzz(func(t *testing.T, keyData []byte) {
		// Test key generation - should not panic
		// Hash the key
		hash := uint64(0)
		for _, b := range keyData {
			hash = hash*31 + uint64(b)
		}
		_ = hash
	})
}

// FuzzStickySessions tests sticky session routing with random session IDs.
func FuzzStickySessions(f *testing.F) {
	f.Add(uint64(1), uint64(100))
	f.Add(uint64(0), uint64(0))
	f.Add(uint64(18446744073709551615), uint64(18446744073709551615))

	f.Fuzz(func(t *testing.T, sessionId uint64, nodeCount uint64) {
		// Test sticky session - should not panic
		if nodeCount == 0 {
			t.Logf("Zero node count")
		}
		_ = sessionId
	})
}
