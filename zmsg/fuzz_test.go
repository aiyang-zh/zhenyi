package zmsg

import (
	"testing"

	"github.com/aiyang-zh/zhenyi-base/zserialize"
)

// FuzzMessageMarshalUnmarshal tests Message serialization/deserialization with random input.
func FuzzMessageMarshalUnmarshal(f *testing.F) {
	// Seed with some basic test cases
	f.Add(int32(1), uint32(100), []byte("hello"))
	f.Add(int32(0), uint32(0), []byte{})
	f.Add(int32(65535), uint32(999), []byte("x"))

	f.Fuzz(func(t *testing.T, msgId int32, seqId uint32, data []byte) {
		// Create a message
		msg := GetMessage()
		defer msg.MustRelease()

		msg.MsgId = msgId
		msg.SeqId = seqId
		msg.Data = append(msg.Data[:0], data...)

		// Marshal
		buf := make([]byte, msg.Size())
		n, err := msg.MarshalTo(buf)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}
		if n != len(buf) {
			t.Fatalf("Marshal size mismatch: got %d, want %d", n, len(buf))
		}

		// Unmarshal
		msg2 := GetMessage()
		defer msg2.MustRelease()

		err = msg2.Unmarshal(buf)
		if err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		// Verify
		if msg2.MsgId != msgId {
			t.Fatalf("MsgId mismatch: got %d, want %d", msg2.MsgId, msgId)
		}
		if msg2.SeqId != seqId {
			t.Fatalf("SeqId mismatch: got %d, want %d", msg2.SeqId, seqId)
		}
		if len(msg2.Data) != len(data) {
			t.Fatalf("Data length mismatch: got %d, want %d", len(msg2.Data), len(data))
		}
		if string(msg2.Data) != string(data) {
			t.Fatalf("Data mismatch: got %q, want %q", msg2.Data, data)
		}
	})
}

// FuzzMessageRefCount tests reference counting with random operations.
func FuzzMessageRefCount(f *testing.F) {
	f.Add([]byte{1, 2, 3})
	f.Add([]byte{})
	f.Add([]byte{255})

	f.Fuzz(func(t *testing.T, data []byte) {
		msg := GetMessage()
		defer msg.MustRelease()

		msg.Data = append(msg.Data[:0], data...)

		// Retain multiple times
		for i := 0; i < 10; i++ {
			msg.Retain()
		}

		// Release multiple times
		for i := 0; i < 10; i++ {
			msg.Release()
		}

		// Should still be valid
		if msg.LoadRefCount() != 1 {
			t.Fatalf("RefCount should be 1, got %d", msg.LoadRefCount())
		}
	})
}

// FuzzMessagePoolReset tests pool reset with random data.
func FuzzMessagePoolReset(f *testing.F) {
	f.Add([]byte("test"))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		msg := GetMessage()
		msg.Data = append(msg.Data[:0], data...)
		msg.MsgId = 12345
		msg.SeqId = 67890

		// Reset
		msg.PoolReset()

		// Verify reset
		if len(msg.Data) != 0 {
			t.Fatalf("Data should be empty after reset, got %d bytes", len(msg.Data))
		}
		if msg.MsgId != 0 {
			t.Fatalf("MsgId should be 0 after reset, got %d", msg.MsgId)
		}
		if msg.SeqId != 0 {
			t.Fatalf("SeqId should be 0 after reset, got %d", msg.SeqId)
		}

		msg.MustRelease()
	})
}

// FuzzJSONMessageUnmarshal tests JSON message deserialization with random input.
func FuzzJSONMessageUnmarshal(f *testing.F) {
	f.Add([]byte(`{"key":"value"}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"a":1,"b":2}`))

	f.Fuzz(func(t *testing.T, jsonData []byte) {
		// Try to unmarshal - should not panic (errors are fine).
		// Note: fuzz only validates "no panic", not semantic correctness.
		if len(jsonData) > 32*1024 {
			jsonData = jsonData[:32*1024]
		}
		var out map[string]any
		_ = zserialize.UnmarshalJson(jsonData, &out)
	})
}
