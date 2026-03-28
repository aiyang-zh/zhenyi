package zcodec

// BytesMessage implements raw byte passthrough for ziface.IMessage.
// BytesMessage 使用原始字节透传实现 ziface.IMessage。
//
// Notes:
// 说明：
// - NewBytesMessage and UnmarshalVT copy inputs to avoid aliasing external or pool-backed slices.
// - NewBytesMessage 与 UnmarshalVT 都会拷贝输入，避免别名到外部/对象池底层切片。
type BytesMessage struct {
	msgID int32
	data  []byte
}

// NewBytesMessage creates a send-ready BytesMessage (copies b).
// NewBytesMessage 生成可直接发送的 BytesMessage（会拷贝 b）。
func NewBytesMessage(msgID int32, b []byte) *BytesMessage {
	c := append([]byte(nil), b...)
	return &BytesMessage{msgID: msgID, data: c}
}

// GetMsgId implements ziface.IMessage.
// GetMsgId 实现 ziface.IMessage。
func (m *BytesMessage) GetMsgId() int32 {
	if m == nil {
		return 0
	}
	return m.msgID
}

// SizeVT implements ziface.IMessage.
// SizeVT 实现 ziface.IMessage。
func (m *BytesMessage) SizeVT() int {
	if m == nil {
		return 0
	}
	return len(m.data)
}

// UnmarshalVT implements ziface.IMessage.
// UnmarshalVT 实现 ziface.IMessage。
func (m *BytesMessage) UnmarshalVT(b []byte) error {
	if m == nil {
		return nil
	}
	m.data = append(m.data[:0], b...)
	return nil
}

// MarshalVT implements ziface.IMessage.
// MarshalVT 实现 ziface.IMessage。
func (m *BytesMessage) MarshalVT() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return append([]byte(nil), m.data...), nil
}

// MarshalToVT implements ziface.IMessage.
// MarshalToVT 实现 ziface.IMessage。
func (m *BytesMessage) MarshalToVT(dst []byte) (int, error) {
	if m == nil {
		return 0, nil
	}
	return copy(dst, m.data), nil
}

// Bytes returns a semantic copy of internal payload.
// Bytes 返回内部 payload 的只读拷贝。
func (m *BytesMessage) Bytes() []byte {
	if m == nil {
		return nil
	}
	return append([]byte(nil), m.data...)
}
