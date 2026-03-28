package zmsg

// Methods below let *Message implement zhenyi-base/ziface.IWireMessage.
// 以下方法使 *Message 实现 zhenyi-base/ziface.IWireMessage，便于 Gate 直接 channel.Send(msg)。

func (m *Message) GetMessageData() []byte     { return m.Data }
func (m *Message) SetMessageData(data []byte) { m.Data = data }
func (m *Message) GetMsgId() int32            { return m.MsgId }
func (m *Message) SetMsgId(id int32)          { m.MsgId = id }
func (m *Message) GetSeqId() uint32           { return m.SeqId }
func (m *Message) SetSeqId(id uint32)         { m.SeqId = id }
func (m *Message) GetRefCount() int32         { return m.RefCount }
func (m *Message) Reset()                     { m.SmartReset() }

func (m *Message) SmartReset() {
	m.MsgId = 0
	m.Data = nil
	m.ToClient = false
	m.SrcActor = 0
	m.TarActor = 0
	m.SessionId = 0
	m.FromClient = false
	m.IsResponse = false
	m.RpcId = 0
	m.TraceIdHi = 0
	m.TraceIdLo = 0
	m.SpanId = 0
	m.SeqId = 0
	m.RefCount = 0
}

// PoolReset is pool-specific reset: clear fields while preserving Data capacity.
// PoolReset 池专用重置：清空字段值但保留 Data/AuthIds 的底层容量。
// Only for GetMessage(); do not call elsewhere.
// ⚠️ 仅供 GetMessage() 使用，不要在其他地方调用。
func (m *Message) PoolReset() {
	m.MsgId = 0
	m.Data = m.Data[:0] // 保留 cap，避免重新分配
	m.ToClient = false
	m.SrcActor = 0
	m.TarActor = 0
	m.SessionId = 0
	m.FromClient = false
	m.IsResponse = false
	m.RpcId = 0
	m.TraceIdHi = 0
	m.TraceIdLo = 0
	m.SpanId = 0
	m.SeqId = 0
}
