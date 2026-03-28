package zcodec

import (
	"testing"

	"github.com/aiyang-zh/zhenyi/ziface"
	"github.com/aiyang-zh/zhenyi/zmsg"
)

func TestStandardAdapters_ImplementIMessage(t *testing.T) {
	var _ ziface.IMessage = (*JSONMessage)(nil)
	var _ ziface.IMessage = (*BytesMessage)(nil)
	var _ ziface.IMessage = (*MsgpackMessage)(nil)
}

func TestJSONMessage_RoundTrip(t *testing.T) {
	type Payload struct {
		Foo string
		Bar int
	}
	in := Payload{Foo: "x", Bar: 42}

	jm, err := NewJSONMessage(1, in)
	if err != nil {
		t.Fatalf("NewJSONMessage failed: %v", err)
	}

	m := &zmsg.Message{}
	if err := ziface.MarshalVTToMsg(jm, m); err != nil {
		t.Fatalf("MarshalVTToMsg failed: %v", err)
	}

	outMsg := &JSONMessage{msgID: 1}
	if err := ziface.UnmarshalVTFromMsg(m, outMsg); err != nil {
		t.Fatalf("UnmarshalVTFromMsg failed: %v", err)
	}

	var out Payload
	if err := outMsg.Decode(&out); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if out != in {
		t.Fatalf("unexpected roundtrip result: in=%+v out=%+v", in, out)
	}
}

func TestJSONMessage_NilReceiver_NoPanic(t *testing.T) {
	var msg *JSONMessage
	if err := msg.UnmarshalVT(nil); err != nil {
		t.Fatalf("expected nil err for nil receiver UnmarshalVT, got %v", err)
	}
	if _, err := msg.MarshalVT(); err != nil {
		t.Fatalf("expected nil err for nil receiver MarshalVT, got %v", err)
	}
	if err := msg.Decode(&struct{}{}); err != nil {
		t.Fatalf("expected nil err for nil receiver Decode, got %v", err)
	}
}

func TestBytesMessage_RoundTrip(t *testing.T) {
	in := []byte("hello")
	bm := NewBytesMessage(2, in)

	m := &zmsg.Message{}
	if err := ziface.MarshalVTToMsg(bm, m); err != nil {
		t.Fatalf("MarshalVTToMsg failed: %v", err)
	}

	outMsg := &BytesMessage{msgID: 2}
	if err := ziface.UnmarshalVTFromMsg(m, outMsg); err != nil {
		t.Fatalf("UnmarshalVTFromMsg failed: %v", err)
	}

	if string(outMsg.data) != string(in) {
		t.Fatalf("unexpected bytes: %q", string(outMsg.data))
	}
}

func TestBytesMessage_NilReceiver_NoPanic(t *testing.T) {
	var msg *BytesMessage
	if err := msg.UnmarshalVT(nil); err != nil {
		t.Fatalf("expected nil err for nil receiver UnmarshalVT, got %v", err)
	}
	if _, err := msg.MarshalVT(); err != nil {
		t.Fatalf("expected nil err for nil receiver MarshalVT, got %v", err)
	}
}

func TestMsgpackMessage_RoundTrip(t *testing.T) {
	type Payload struct {
		Foo string
		Bar int
	}
	in := Payload{Foo: "x", Bar: 42}

	mm, err := NewMsgpackMessage(3, in)
	if err != nil {
		t.Fatalf("NewMsgpackMessage failed: %v", err)
	}

	m := &zmsg.Message{}
	if err := ziface.MarshalVTToMsg(mm, m); err != nil {
		t.Fatalf("MarshalVTToMsg failed: %v", err)
	}

	outMsg := &MsgpackMessage{msgID: 3}
	if err := ziface.UnmarshalVTFromMsg(m, outMsg); err != nil {
		t.Fatalf("UnmarshalVTFromMsg failed: %v", err)
	}

	var out Payload
	if err := outMsg.Decode(&out); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if out != in {
		t.Fatalf("unexpected roundtrip result: in=%+v out=%+v", in, out)
	}
}
