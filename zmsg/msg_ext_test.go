package zmsg

import "testing"

func TestWireMessageAccessors(t *testing.T) {
	m := &Message{}
	m.SetMsgId(123)
	if got := m.GetMsgId(); got != 123 {
		t.Fatalf("GetMsgId=%d", got)
	}
	m.SetSeqId(456)
	if got := m.GetSeqId(); got != 456 {
		t.Fatalf("GetSeqId=%d", got)
	}
	m.SetMessageData([]byte{1, 2, 3})
	if got := m.GetMessageData(); len(got) != 3 || got[0] != 1 {
		t.Fatalf("GetMessageData=%v", got)
	}
}

func TestResetCallsSmartReset(t *testing.T) {
	m := &Message{
		MsgId:     1,
		Data:      []byte{1},
		ToClient:  true,
		SessionId: 3,
		RefCount:  4,
	}
	m.Reset()
	if m.MsgId != 0 || m.Data != nil || m.ToClient || m.SessionId != 0 || m.RefCount != 0 {
		t.Fatalf("expected SmartReset state, got %+v", *m)
	}
}
