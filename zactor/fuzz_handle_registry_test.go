package zactor

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/aiyang-zh/zhenyi/zmsg"
)

func clampFuzzBytes(b []byte, max int) []byte {
	if len(b) <= max {
		return b
	}
	return b[:max]
}

func FuzzHandleRegistry_HandleClientMessage_NoPanic(f *testing.F) {
	f.Add(int32(1), uint32(1), []byte("hello"))
	f.Add(int32(0), uint32(0), []byte{})
	f.Add(int32(-1), uint32(123), []byte{0xff, 0xee, 0xdd})

	f.Fuzz(func(t *testing.T, msgId int32, seqId uint32, data []byte) {
		data = clampFuzzBytes(data, 16*1024)

		h := newTestHandle()

		var called atomic.Bool
		var gotSeq uint32
		var gotLen int

		h.RegisterHandle(msgId, func(ctx context.Context, m *zmsg.Message) {
			_ = ctx
			called.Store(true)
			gotSeq = m.SeqId
			gotLen = len(m.Data)
		})

		m := zmsg.GetMessage()
		m.MsgId = msgId
		m.SeqId = seqId
		m.Data = append(m.Data[:0], data...)
		defer m.Release()

		h.HandleClientMessage(context.Background(), m)

		if !called.Load() {
			t.Fatalf("handler not called, msgId=%d seq=%d", msgId, seqId)
		}
		if gotSeq != seqId {
			t.Fatalf("SeqId mismatch: got=%d want=%d", gotSeq, seqId)
		}
		if gotLen != len(data) {
			t.Fatalf("Data length mismatch: got=%d want=%d", gotLen, len(data))
		}

		// Unregistered msgId path should not panic.
		m2 := zmsg.GetMessage()
		m2.MsgId = msgId + 1
		m2.SeqId = seqId
		m2.Data = append(m2.Data[:0], data...)
		defer m2.Release()
		h.HandleClientMessage(context.Background(), m2)
	})
}
