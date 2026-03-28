package zcodec

import (
	"github.com/aiyang-zh/zhenyi-base/zserialize"
)

// JSONMessage implements JSON encoding/decoding for ziface.IMessage.
// JSONMessage 使用 json 编解码实现 ziface.IMessage。
//
// Notes:
// 说明：
// - UnmarshalVT copies input `b` to avoid aliasing external or pool-backed slices (prevents data rewrite after reuse).
// - UnmarshalVT 会拷贝输入 b，避免 alias 到外部/对象池底层切片导致数据在后续回收后被改写。
// - MarshalToVT copies internal `data` into `dst` to support zhenyi's zero-allocation send path (dst is framework-provided).
// - MarshalToVT 会把内部 data 拷贝到 dst，兼容 zhenyi 的零分配发送路径（dst 由框架提供）。
type JSONMessage struct {
	msgID int32
	data  []byte
}

// NewJSONMessage marshals v into JSON and creates a send-ready JSONMessage.
// NewJSONMessage 使用 json.Marshal 编码 v，生成可直接发送的 JSONMessage。
func NewJSONMessage(msgID int32, v any) (*JSONMessage, error) {
	b, err := zserialize.MarshalJson(v)
	if err != nil {
		return nil, err
	}
	return &JSONMessage{
		msgID: msgID,
		data:  b,
	}, nil
}

// NewJSONMessageFromBytes creates JSONMessage from raw bytes (copies input).
// NewJSONMessageFromBytes 直接使用 bytes 作为 payload（会拷贝）。
func NewJSONMessageFromBytes(msgID int32, b []byte) *JSONMessage {
	c := append([]byte(nil), b...)
	return &JSONMessage{msgID: msgID, data: c}
}

// GetMsgId implements ziface.IMessage.
// GetMsgId 实现 ziface.IMessage。
func (m *JSONMessage) GetMsgId() int32 {
	if m == nil {
		return 0
	}
	return m.msgID
}

// SizeVT implements ziface.IMessage.
// SizeVT 实现 ziface.IMessage。
func (m *JSONMessage) SizeVT() int {
	if m == nil {
		return 0
	}
	return len(m.data)
}

// UnmarshalVT implements ziface.IMessage.
// UnmarshalVT 实现 ziface.IMessage。
func (m *JSONMessage) UnmarshalVT(b []byte) error {
	if m == nil {
		return nil
	}
	m.data = append(m.data[:0], b...)
	return nil
}

// MarshalVT implements ziface.IMessage and returns a semantic copy of payload.
// MarshalVT 实现 ziface.IMessage，并返回 payload 的语义拷贝。
func (m *JSONMessage) MarshalVT() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	// Return a semantic copy to avoid external mutation of internal buffer.
	// 返回语义上是“新切片”，避免外部修改影响内部缓冲。
	return append([]byte(nil), m.data...), nil
}

// MarshalToVT implements ziface.IMessage and writes payload into dst.
// MarshalToVT 实现 ziface.IMessage，并将 payload 写入 dst。
func (m *JSONMessage) MarshalToVT(dst []byte) (int, error) {
	if m == nil {
		return 0, nil
	}
	return copy(dst, m.data), nil
}

// Decode unmarshals current JSON payload into out.
// Decode 把当前 payload 反序列化到 out。
func (m *JSONMessage) Decode(out any) error {
	if m == nil {
		return nil
	}
	return zserialize.UnmarshalJson(m.data, out)
}

// Encode marshals v into JSON and overwrites internal payload.
// Encode 把 v 编码为 JSON payload 覆盖当前 data。
func (m *JSONMessage) Encode(v any) error {
	if m == nil {
		return nil
	}
	b, err := zserialize.MarshalJson(v)
	if err != nil {
		return err
	}
	m.data = b
	return nil
}
