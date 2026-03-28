package zcodec

import (
	"testing"

	"github.com/aiyang-zh/zhenyi/ziface"
	"github.com/aiyang-zh/zhenyi/zmsg"
)

type benchPayload struct {
	UserID int64             `json:"userId"`
	Room   string            `json:"room"`
	Flags  map[string]bool   `json:"flags"`
	Meta   map[string]string `json:"meta"`
}

var benchData = benchPayload{
	UserID: 10001,
	Room:   "lobby",
	Flags:  map[string]bool{"a": true, "b": false, "c": true},
	Meta:   map[string]string{"k1": "v1", "k2": "v2"},
}

func BenchmarkJSONMessage_EncodeAndMarshalVTToMsg(b *testing.B) {
	jm := &JSONMessage{}

	// 给足 Message.Data 容量，避免 benchmark 中混入 make([]byte, size) 的分配成本。
	// 注意：jm.Encode 后 jm.SizeVT 可能会变化，这里用偏大的固定容量兜底。
	m := &zmsg.Message{Data: make([]byte, 0, 64*1024)}

	var encodedSize int
	if err := jm.Encode(benchData); err != nil {
		b.Fatalf("encode failed: %v", err)
	}
	encodedSize = jm.SizeVT()
	if encodedSize > cap(m.Data) {
		m.Data = make([]byte, 0, encodedSize*2)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := jm.Encode(benchData); err != nil {
			b.Fatalf("encode failed: %v", err)
		}
		if err := ziface.MarshalVTToMsg(jm, m); err != nil {
			b.Fatalf("MarshalVTToMsg failed: %v", err)
		}
	}
}

func BenchmarkJSONMessage_UnmarshalVTAndDecode(b *testing.B) {
	// 先生成一次固定输入。
	req, err := NewJSONMessage(1, benchData)
	if err != nil {
		b.Fatalf("NewJSONMessage failed: %v", err)
	}
	body, err := req.MarshalVT()
	if err != nil {
		b.Fatalf("MarshalVT failed: %v", err)
	}

	outMsg := &JSONMessage{}
	var out benchPayload

	m := &zmsg.Message{Data: body}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 框架路径：先把 msg.Data 反序列化到 IMessage，再按业务使用 Decode。
		if err := ziface.UnmarshalVTFromMsg(m, outMsg); err != nil {
			b.Fatalf("UnmarshalVTFromMsg failed: %v", err)
		}
		if err := outMsg.Decode(&out); err != nil {
			b.Fatalf("Decode failed: %v", err)
		}
	}
}

func BenchmarkBytesMessage_MarshalVTToMsg(b *testing.B) {
	bm := NewBytesMessage(2, []byte(`{"userId":10001,"room":"lobby","flags":{"a":true}}`))
	m := &zmsg.Message{Data: make([]byte, 0, 4*1024)}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := ziface.MarshalVTToMsg(bm, m); err != nil {
			b.Fatalf("MarshalVTToMsg failed: %v", err)
		}
	}
}

func BenchmarkBytesMessage_UnmarshalVTFromMsg(b *testing.B) {
	encoded := []byte(`{"userId":10001,"room":"lobby","flags":{"a":true}}`)
	m := &zmsg.Message{Data: encoded}
	outMsg := &BytesMessage{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := ziface.UnmarshalVTFromMsg(m, outMsg); err != nil {
			b.Fatalf("UnmarshalVTFromMsg failed: %v", err)
		}
	}
}

func BenchmarkMsgpackMessage_EncodeAndMarshalVTToMsg(b *testing.B) {
	mm := &MsgpackMessage{}
	m := &zmsg.Message{Data: make([]byte, 0, 64*1024)}

	if err := mm.Encode(benchData); err != nil {
		b.Fatalf("encode failed: %v", err)
	}
	if size := mm.SizeVT(); size > cap(m.Data) {
		m.Data = make([]byte, 0, size*2)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := mm.Encode(benchData); err != nil {
			b.Fatalf("encode failed: %v", err)
		}
		if err := ziface.MarshalVTToMsg(mm, m); err != nil {
			b.Fatalf("MarshalVTToMsg failed: %v", err)
		}
	}
}

func BenchmarkMsgpackMessage_UnmarshalVTAndDecode(b *testing.B) {
	req, err := NewMsgpackMessage(3, benchData)
	if err != nil {
		b.Fatalf("NewMsgpackMessage failed: %v", err)
	}
	body, err := req.MarshalVT()
	if err != nil {
		b.Fatalf("MarshalVT failed: %v", err)
	}

	outMsg := &MsgpackMessage{}
	var out benchPayload
	m := &zmsg.Message{Data: body}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := ziface.UnmarshalVTFromMsg(m, outMsg); err != nil {
			b.Fatalf("UnmarshalVTFromMsg failed: %v", err)
		}
		if err := outMsg.Decode(&out); err != nil {
			b.Fatalf("Decode failed: %v", err)
		}
	}
}
