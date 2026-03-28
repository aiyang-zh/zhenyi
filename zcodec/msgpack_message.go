package zcodec

import (
	"github.com/aiyang-zh/zhenyi-base/zserialize"
)

// MsgpackMessage implements msgpack encoding/decoding for ziface.IMessage (via zhenyi-base/zserialize).
// MsgpackMessage 使用 msgpack 编解码实现 ziface.IMessage（底层调用 zhenyi-base/zserialize）。
type MsgpackMessage struct {
	msgID int32
	data  []byte
}

// NewMsgpackMessage marshals v into msgpack and creates a send-ready MsgpackMessage.
// NewMsgpackMessage 使用 msgpack.Marshal 编码 v，生成可直接发送的 MsgpackMessage。
func NewMsgpackMessage(msgID int32, v any) (*MsgpackMessage, error) {
	b, err := zserialize.MarshalMsgPack(v)
	if err != nil {
		return nil, err
	}
	return &MsgpackMessage{msgID: msgID, data: b}, nil
}

// NewMsgpackMessageFromBytes creates MsgpackMessage from raw bytes (copies input).
// NewMsgpackMessageFromBytes 直接使用 bytes 作为 payload（会拷贝）。
func NewMsgpackMessageFromBytes(msgID int32, b []byte) *MsgpackMessage {
	c := append([]byte(nil), b...)
	return &MsgpackMessage{msgID: msgID, data: c}
}

// GetMsgId implements ziface.IMessage.
// GetMsgId 实现 ziface.IMessage。
func (m *MsgpackMessage) GetMsgId() int32 {
	if m == nil {
		return 0
	}
	return m.msgID
}

// SizeVT implements ziface.IMessage.
// SizeVT 实现 ziface.IMessage。
func (m *MsgpackMessage) SizeVT() int {
	if m == nil {
		return 0
	}
	return len(m.data)
}

// UnmarshalVT implements ziface.IMessage.
// UnmarshalVT 实现 ziface.IMessage。
func (m *MsgpackMessage) UnmarshalVT(b []byte) error {
	if m == nil {
		return nil
	}
	m.data = append(m.data[:0], b...)
	return nil
}

// MarshalVT implements ziface.IMessage and returns a semantic copy of payload.
// MarshalVT 实现 ziface.IMessage，并返回 payload 的语义拷贝。
func (m *MsgpackMessage) MarshalVT() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return append([]byte(nil), m.data...), nil
}

// MarshalToVT implements ziface.IMessage and writes payload into dst.
// MarshalToVT 实现 ziface.IMessage，并将 payload 写入 dst。
func (m *MsgpackMessage) MarshalToVT(dst []byte) (int, error) {
	if m == nil {
		return 0, nil
	}
	return copy(dst, m.data), nil
}

// Decode unmarshals current msgpack payload into out.
// Decode 把当前 payload 反序列化到 out。
func (m *MsgpackMessage) Decode(out any) error {
	if m == nil {
		return nil
	}
	return zserialize.UnmarshalMsgPack(m.data, out)
}

// Encode marshals v into msgpack and overwrites internal payload.
// Encode 把 v 编码为 msgpack payload 覆盖当前 data。
func (m *MsgpackMessage) Encode(v any) error {
	if m == nil {
		return nil
	}
	b, err := zserialize.MarshalMsgPack(v)
	if err != nil {
		return err
	}
	m.data = b
	return nil
}
