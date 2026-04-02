package zcodec

import "testing"

func clampBytes(b []byte, max int) []byte {
	if len(b) <= max {
		return b
	}
	return b[:max]
}

func FuzzJSONMessageDecode_NoPanic(f *testing.F) {
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"a":1}`))
	f.Add([]byte("not-json"))

	f.Fuzz(func(t *testing.T, payload []byte) {
		payload = clampBytes(payload, 16*1024)
		msg := NewJSONMessageFromBytes(1, payload)
		var out map[string]any
		_ = msg.Decode(&out) // errors are ok; only panic is forbidden
	})
}

func FuzzMsgpackMessageDecode_NoPanic(f *testing.F) {
	f.Add([]byte{0x80})       // empty map
	f.Add([]byte{0x91, 0x01}) // array with 1 elem
	f.Add([]byte("not-msgpack"))

	f.Fuzz(func(t *testing.T, payload []byte) {
		payload = clampBytes(payload, 16*1024)
		msg := NewMsgpackMessageFromBytes(1, payload)
		var out map[string]any
		_ = msg.Decode(&out)
	})
}

func FuzzBytesMessageMarshalTo_NoPanic(f *testing.F) {
	f.Add([]byte("hello"), uint8(1))
	f.Add([]byte("hello"), uint8(8))
	f.Add([]byte{}, uint8(0))

	f.Fuzz(func(t *testing.T, payload []byte, dstLen uint8) {
		payload = clampBytes(payload, 8*1024)
		msg := NewBytesMessage(1, payload)

		// dst can be smaller than payload; copy should just truncate without panic.
		dst := make([]byte, int(dstLen))
		_, _ = msg.MarshalToVT(dst)
		_, _ = msg.MarshalVT()
	})
}
