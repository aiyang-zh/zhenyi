package zmsg

import (
	"encoding/binary"
	"errors"
	"github.com/aiyang-zh/zhenyi-base/zpool"
)

// Bitmask flags used to pack bool fields.
// 定义位掩码常量，用于压缩 bool 字段。
const (
	flagToClient   = 1 << 0 // 0000 0001
	flagFromClient = 1 << 1 // 0000 0010
	flagIsResponse = 1 << 2 // 0000 0100
)

// FixedHeaderSize is fixed base-header length in bytes.
// FixedHeaderSize 基础头部固定长度：所有 int/uint/bool 加起来的字节数。
// 1(flags) + 4(MsgId) + 8(Src) + 8(Tar) + 8(Session) + 8(Rpc) + 4(Seq) + 8(TraceHi) + 8(TraceLo) + 8(SpanId)
const FixedHeaderSize = 65
const zeroCopyData = false

// ErrBufferTooSmall indicates destination buffer size is insufficient.
// ErrBufferTooSmall 表示目标 buffer 太小。
var ErrBufferTooSmall = errors.New("buffer too small")

// ErrDataCorrupt indicates input data is invalid/corrupted.
// ErrDataCorrupt 表示输入数据无效或已损坏。
var ErrDataCorrupt = errors.New("data corrupt")

const maxMessageDataSize = 16 << 20 // 16 MB

// Message is wire-protocol envelope (handwritten struct, no msg.proto dependency).
// Message 线协议消息（手写结构体，不再依赖 msg.proto）。
// Memory layout: 8-byte groups -> slice -> 4-byte groups -> bool fields.
// 内存布局：8 字节组 → 切片 → 4 字节组 → bool。
type Message struct {
	SessionId  uint64
	RpcId      uint64
	TraceIdHi  uint64
	TraceIdLo  uint64
	SpanId     uint64
	Data       []byte
	MsgId      int32
	SrcActor   uint64
	TarActor   uint64
	RefCount   int32
	SeqId      uint32
	ToClient   bool
	FromClient bool
	IsResponse bool
}

func (m *Message) Size() int {
	size := FixedHeaderSize

	// Data length + payload bytes.
	// Data 长度 + Data 内容。
	size += 4 + len(m.Data) // 这里假设 Data 长度用 4 字节固定长度存，比 Varint 更快

	return size
}

// Marshal serializes message into a newly allocated byte slice.
// Marshal 将对象序列化为字节流。
// Prefer MarshalTo for buffer reuse.
// 推荐使用 MarshalTo 以复用 buffer。
func (m *Message) Marshal() ([]byte, error) {
	buf := make([]byte, m.Size())
	_, err := m.MarshalTo(buf)
	return buf, err
}

// MarshalPooled serializes using bytespool buffer for low-allocation hot path.
// MarshalPooled 从 bytespool 获取 *pool.Buffer 序列化，零分配热路径。
// Caller must call buf.Release() after use.
// ⚠️ 调用方使用完毕后必须调用 buf.Release() 归还。
func (m *Message) MarshalPooled() (*zpool.Buffer, error) {
	size := m.Size()
	buf := zpool.GetBytesBuffer(size)
	n, err := m.MarshalTo(buf.B)
	if err != nil {
		buf.Release()
		return nil, err
	}
	buf.B = buf.B[:n]
	return buf, nil
}

// MarshalTo writes message into preallocated buffer (key to zero-allocation path).
// MarshalTo 将数据写入预分配的 buffer，这是零分配的关键。
// 注意：RefCount 是运行时引用计数字段，仅用于本地对象池管理，不参与网络序列化。
// proto 生成代码中包含 RefCount 是历史遗留，禁止使用 proto.Marshal 序列化 Message 信封。
func (m *Message) MarshalTo(buf []byte) (int, error) {
	if len(buf) < m.Size() {
		return 0, ErrBufferTooSmall
	}

	offset := 0

	// 1) Bit packing (3 bools into 1 byte).
	// 1. Bit Packing (压缩 3 个 bool 到 1 个 byte)
	var flags uint8
	if m.ToClient {
		flags |= flagToClient
	}
	if m.FromClient {
		flags |= flagFromClient
	}
	if m.IsResponse {
		flags |= flagIsResponse
	}
	buf[offset] = flags
	offset++

	// 2) Write fixed-width numeric fields (LittleEndian, CPU-friendly).
	// 2. 写入固定长度数值 (使用 LittleEndian，CPU 友好)
	// Compiler usually inlines these into MOV-like instructions.
	// 编译器会将这些内联优化为 MOV 指令。
	binary.LittleEndian.PutUint32(buf[offset:], uint32(m.MsgId))
	offset += 4

	binary.LittleEndian.PutUint64(buf[offset:], m.SrcActor)
	offset += 8

	binary.LittleEndian.PutUint64(buf[offset:], m.TarActor)
	offset += 8

	binary.LittleEndian.PutUint64(buf[offset:], m.SessionId)
	offset += 8

	binary.LittleEndian.PutUint64(buf[offset:], m.RpcId)
	offset += 8

	binary.LittleEndian.PutUint32(buf[offset:], m.SeqId)
	offset += 4

	binary.LittleEndian.PutUint64(buf[offset:], m.TraceIdHi)
	offset += 8

	binary.LittleEndian.PutUint64(buf[offset:], m.TraceIdLo)
	offset += 8

	binary.LittleEndian.PutUint64(buf[offset:], m.SpanId)
	offset += 8

	// 3) Write Data []byte: length (4 bytes) then payload.
	// 3. 写入 Data []byte
	// 先写长度 (4字节)，再写内容。
	dataLen := uint32(len(m.Data))
	binary.LittleEndian.PutUint32(buf[offset:], dataLen)
	offset += 4
	if dataLen > 0 {
		copy(buf[offset:], m.Data)
		offset += int(dataLen)
	}

	return offset, nil
}

// Unmarshal decodes message from byte slice.
// Unmarshal 从字节流反序列化。
// zeroCopyData：当前固定为 false（安全优先，避免对 buf 生命周期的隐式依赖）。
// 若误改为 true，将导致 m.Data 直接引用 buf 内存（buf 一旦被复用/回收就会出现诡异数据错误）。
// 在开启零拷贝前必须满足严格约束并补齐基准/压力测试；默认禁止误改。
func (m *Message) Unmarshal(buf []byte) error {
	if len(buf) < FixedHeaderSize {
		return ErrDataCorrupt
	}

	offset := 0

	// 1. 解码 Flags
	flags := buf[offset]
	m.ToClient = (flags & flagToClient) != 0
	m.FromClient = (flags & flagFromClient) != 0
	m.IsResponse = (flags & flagIsResponse) != 0
	offset++

	// 2. 解码固定数值
	m.MsgId = int32(binary.LittleEndian.Uint32(buf[offset:]))
	offset += 4

	m.SrcActor = binary.LittleEndian.Uint64(buf[offset:])
	offset += 8

	m.TarActor = binary.LittleEndian.Uint64(buf[offset:])
	offset += 8

	m.SessionId = binary.LittleEndian.Uint64(buf[offset:])
	offset += 8

	m.RpcId = binary.LittleEndian.Uint64(buf[offset:])
	offset += 8

	m.SeqId = binary.LittleEndian.Uint32(buf[offset:])
	offset += 4

	m.TraceIdHi = binary.LittleEndian.Uint64(buf[offset:])
	offset += 8

	m.TraceIdLo = binary.LittleEndian.Uint64(buf[offset:])
	offset += 8

	m.SpanId = binary.LittleEndian.Uint64(buf[offset:])
	offset += 8

	// 3. 解码 Data
	if offset+4 > len(buf) {
		return ErrDataCorrupt
	}
	dataLen := int(binary.LittleEndian.Uint32(buf[offset:]))
	offset += 4

	if dataLen > maxMessageDataSize {
		return ErrDataCorrupt
	}
	if dataLen > 0 {
		if offset+dataLen > len(buf) {
			return ErrDataCorrupt
		}

		if zeroCopyData {
			// Fast mode: direct slice reference, no allocation/copy.
			// 【极速模式】直接切片引用，无内存分配，无拷贝
			// ⚠️ 警告：调用方必须保证 buf 在 Message 使用期间不会被修改或回收
			m.Data = buf[offset : offset+dataLen]
		} else {
			// Safe mode: reuse existing capacity to avoid repeated make.
			// 【安全模式】复用已有容量，避免每次 make
			if cap(m.Data) >= dataLen {
				m.Data = m.Data[:dataLen]
			} else {
				m.Data = make([]byte, dataLen)
			}
			copy(m.Data, buf[offset:offset+dataLen])
		}
		offset += dataLen
	} else {
		m.Data = nil
	}
	// RefCount 是本地运行时字段，Unmarshal 后必须保持调用方设定的值，
	// 不从网络数据恢复（自定义协议本身不序列化此字段，这里是防御性保护）。
	return nil
}
