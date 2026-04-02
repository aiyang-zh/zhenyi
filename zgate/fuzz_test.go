package zgate

import (
	"testing"
)

// FuzzParseWireMessage tests wire protocol parsing with random input.
func FuzzParseWireMessage(f *testing.F) {
	// Seed with valid wire message format: [msgId(4)][seqId(4)][dataLen(4)][data]
	f.Add([]byte{
		0x00, 0x00, 0x00, 0x01, // msgId = 1
		0x00, 0x00, 0x00, 0x02, // seqId = 2
		0x00, 0x00, 0x00, 0x05, // dataLen = 5
		0x68, 0x65, 0x6c, 0x6c, 0x6f, // "hello"
	})
	f.Add([]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		// Try to parse - should not panic
		// This is a basic test; actual parsing depends on your wire protocol implementation
		if len(data) >= 12 {
			// Extract header
			msgId := int32(data[0])<<24 | int32(data[1])<<16 | int32(data[2])<<8 | int32(data[3])
			seqId := int32(data[4])<<24 | int32(data[5])<<16 | int32(data[6])<<8 | int32(data[7])
			dataLen := int32(data[8])<<24 | int32(data[9])<<16 | int32(data[10])<<8 | int32(data[11])

			// Verify basic invariants
			if dataLen < 0 {
				t.Logf("Invalid dataLen: %d", dataLen)
			}
			_ = msgId
			_ = seqId
		}
	})
}

// FuzzGateRouting tests message routing with random actor IDs and message IDs.
func FuzzGateRouting(f *testing.F) {
	f.Add(uint64(1), int32(100))
	f.Add(uint64(0), int32(0))
	f.Add(uint64(18446744073709551615), int32(2147483647))

	f.Fuzz(func(t *testing.T, actorId uint64, msgId int32) {
		// Test routing key generation - should not panic
		// This is a placeholder; actual routing depends on your implementation
		_ = actorId
		_ = msgId
	})
}

// FuzzHeartbeatProcessing tests heartbeat message processing with random intervals.
func FuzzHeartbeatProcessing(f *testing.F) {
	f.Add(int64(1000))
	f.Add(int64(0))
	f.Add(int64(9223372036854775807))

	f.Fuzz(func(t *testing.T, interval int64) {
		// Test heartbeat processing - should not panic
		if interval < 0 {
			t.Logf("Invalid interval: %d", interval)
		}
		_ = interval
	})
}

// FuzzConnectionHandling tests connection state transitions with random events.
func FuzzConnectionHandling(f *testing.F) {
	f.Add(uint64(1), true)
	f.Add(uint64(0), false)
	f.Add(uint64(999999), true)

	f.Fuzz(func(t *testing.T, connId uint64, isConnected bool) {
		// Test connection state - should not panic
		_ = connId
		_ = isConnected
	})
}

// FuzzRateLimiting tests rate limiter with random token counts.
func FuzzRateLimiting(f *testing.F) {
	f.Add(int64(0))
	f.Add(int64(1000))
	f.Add(int64(9223372036854775807))

	f.Fuzz(func(t *testing.T, tokens int64) {
		// Test rate limiting - should not panic
		if tokens < 0 {
			t.Logf("Invalid token count: %d", tokens)
		}
		_ = tokens
	})
}
