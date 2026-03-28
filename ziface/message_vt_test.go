package ziface

import (
	"errors"
	"reflect"
	"testing"

	"github.com/aiyang-zh/zhenyi/zmsg"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/runtime/protoimpl"
)

// dummyVTMessage is a minimal proto.Message + vt codec implementation for testing
// MarshalVTToMsg / UnmarshalVTFromMsg without depending on generated protobufs.
type dummyVTMessage struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	msgID int32
	data  []byte
}

func (x *dummyVTMessage) Reset()         { *x = dummyVTMessage{} }
func (x *dummyVTMessage) String() string { return "dummyVTMessage" }
func (*dummyVTMessage) ProtoMessage()    {}
func (x *dummyVTMessage) ProtoReflect() protoreflect.Message {
	mi := &dummyVTMessageInfo
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

var dummyVTMessageInfo = protoimpl.MessageInfo{
	GoReflectType: reflect.TypeOf((*dummyVTMessage)(nil)).Elem(),
}

func (x *dummyVTMessage) UnmarshalVT(b []byte) error {
	x.data = append(x.data[:0], b...)
	return nil
}
func (x *dummyVTMessage) MarshalVT() ([]byte, error) { return append([]byte(nil), x.data...), nil }
func (x *dummyVTMessage) MarshalToVT(dst []byte) (int, error) {
	n := copy(dst, x.data)
	return n, nil
}
func (x *dummyVTMessage) SizeVT() int     { return len(x.data) }
func (x *dummyVTMessage) GetMsgId() int32 { return x.msgID }

type errVTMessage struct{ dummyVTMessage }

func (e *errVTMessage) MarshalToVT([]byte) (int, error) { return 0, errors.New("boom") }

func TestMarshalVTToMsg_ReusesData(t *testing.T) {
	m := zmsg.GetMessage()
	defer m.Release()

	p := &dummyVTMessage{msgID: 1, data: []byte("hello")}
	if err := MarshalVTToMsg(p, m); err != nil {
		t.Fatalf("MarshalVTToMsg err: %v", err)
	}
	if string(m.Data) != "hello" {
		t.Fatalf("unexpected m.Data: %q", string(m.Data))
	}
}

func TestMarshalVTToMsg_SizeZeroClearsData(t *testing.T) {
	m := zmsg.GetMessage()
	defer m.Release()
	m.Data = append(m.Data[:0], []byte("x")...)
	p := &dummyVTMessage{msgID: 1, data: nil}
	if err := MarshalVTToMsg(p, m); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(m.Data) != 0 {
		t.Fatalf("expected cleared data")
	}
}

func TestMarshalVTToMsg_GrowsDataWhenCapInsufficient(t *testing.T) {
	m := zmsg.GetMessage()
	defer m.Release()
	// force small cap
	m.Data = make([]byte, 0, 1)
	p := &dummyVTMessage{msgID: 1, data: []byte("0123456789")}
	if err := MarshalVTToMsg(p, m); err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(m.Data) != "0123456789" {
		t.Fatalf("unexpected: %q", string(m.Data))
	}
}

func TestMarshalVTToMsg_PropagatesMarshalError(t *testing.T) {
	m := zmsg.GetMessage()
	defer m.Release()
	p := &errVTMessage{dummyVTMessage{msgID: 1, data: []byte("x")}}
	if err := MarshalVTToMsg(p, m); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUnmarshalVTFromMsg(t *testing.T) {
	m := zmsg.GetMessage()
	defer m.Release()
	m.Data = append(m.Data[:0], []byte("world")...)

	p := &dummyVTMessage{msgID: 1}
	if err := UnmarshalVTFromMsg(m, p); err != nil {
		t.Fatalf("UnmarshalVTFromMsg err: %v", err)
	}
	if string(p.data) != "world" {
		t.Fatalf("unexpected decoded data: %q", string(p.data))
	}
}

func TestUnmarshalVTFromMsg_NilAndEmptyNoop(t *testing.T) {
	p := &dummyVTMessage{msgID: 1}
	_ = UnmarshalVTFromMsg(nil, p)
	m := zmsg.GetMessage()
	defer m.Release()
	m.Data = m.Data[:0]
	_ = UnmarshalVTFromMsg(m, p)
	_ = UnmarshalVTFromMsg(m, nil)
}

func BenchmarkMarshalVTToMsg(b *testing.B) {
	m := zmsg.GetMessage()
	defer m.Release()
	p := &dummyVTMessage{msgID: 1, data: make([]byte, 128)}
	for i := range p.data {
		p.data[i] = byte(i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = MarshalVTToMsg(p, m)
	}
}
