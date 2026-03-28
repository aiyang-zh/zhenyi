package ziface

import (
	"context"

	"github.com/aiyang-zh/zhenyi/zmsg"
)

// IMessageHandler defines message handling callbacks.
// IMessageHandler 消息处理接口。
type IMessageHandler interface {
	HandleMessage(ctx context.Context, msg *zmsg.Message)
	HandleRespMessage(ctx context.Context, msg *zmsg.Message)
}

// IToClientFastPath is an optional direct fast path for ToClient responses.
// IToClientFastPath 可选的 ToClient 直达快路径。
//
// Usage:
// 使用场景：
//   - For Actors (typically Gate) where ToClient handling is pure forwarding and thread-safe,
//     processing can happen directly in caller goroutine to reduce latency.
//   - 某些 Actor（典型是 Gate）对 ToClient 响应消息的处理是“纯转发”，实现完全线程安全；
//     为降低延迟，可以允许在 Push 调用方 goroutine 直接处理，而不进入 mailbox。
//
// Notes:
// 注意：
// - Implement this interface only when concurrent calls are guaranteed thread-safe.
// - Return true to indicate handled/taken-over; false to fallback to mailbox path.
// - 只有当实现方确保该方法在并发调用下是线程安全的，才能实现该接口。
// - 返回 true 表示已处理并“接管”消息；返回 false 表示不处理，框架会走常规 mailbox 路径。
type IToClientFastPath interface {
	HandleToClientFastPath(msg *zmsg.Message) bool
}

// IMessageSender defines message sending operations.
// IMessageSender 消息发送接口。
type IMessageSender interface {
	SendMessage(msg *zmsg.Message) error
	SendToClient(msg *zmsg.Message) error
}

// IMessageDispatcher defines message dispatch operations.
// IMessageDispatcher 消息分发接口。
type IMessageDispatcher interface {
	Dispatch(ctx context.Context, handler IMessageHandler, msg *zmsg.Message)
	RegisterHandler(msgId int32, handler IMessageHandler)
}

// IMessage defines VT serialization contract for business messages.
// IMessage 定义业务消息需实现的 VT 序列化契约。
type IMessage interface {
	UnmarshalVT([]byte) error
	MarshalVT() (dAtA []byte, err error)
	MarshalToVT(dAtA []byte) (int, error) // Zero-allocation serialization to external buffer / 零分配序列化到外部 buffer
	SizeVT() int                          // Serialized byte size / 序列化后字节大小
	GetMsgId() int32
}

// MarshalVTToMsg serializes IMessage into Message.Data with buffer reuse.
// MarshalVTToMsg 序列化 IMessage 直接写入 Message.Data（复用已有 Data 容量，减少堆分配）
//
// Rationale:
// 优化原理：
// Message 从对象池获取时自带 Data cap=256，大多数业务消息 < 256 字节，
// 直接写入无需 make。容量不够时自动扩容，扩容后的 cap 随 Message 回池保留给下次复用。
// Message objects from pool usually have Data cap=256; most payloads fit directly without make.
// When capacity is insufficient, it grows and retained for future pooled reuse.
//
// Applicable scenarios:
// 适用场景：
// SendActor / CallActor / SendToClient 等单消息发送
// BatchSendToClients：MarshalVT 一次后对每条投递复制 Data（见 zactor.Actor.BatchSendToClients）。
func MarshalVTToMsg(proto IMessage, m *zmsg.Message) error {
	size := proto.SizeVT()
	if size == 0 {
		m.Data = m.Data[:0]
		return nil
	}
	// 复用已有容量，避免 make
	if cap(m.Data) >= size {
		m.Data = m.Data[:size]
	} else {
		m.Data = make([]byte, size)
	}
	n, err := proto.MarshalToVT(m.Data)
	if err != nil {
		return err
	}
	m.Data = m.Data[:n]
	return nil
}

// UnmarshalVTFromMsg deserializes Message.Data into IMessage.
// UnmarshalVTFromMsg 将 Message.Data 反序列化到 IMessage。
// 与 MarshalVTToMsg 配套，用于需要显式解码业务消息的场景。
// It pairs with MarshalVTToMsg for scenarios requiring explicit business decoding.
func UnmarshalVTFromMsg(m *zmsg.Message, proto IMessage) error {
	if m == nil || proto == nil {
		return nil
	}
	if len(m.Data) == 0 {
		return nil
	}
	return proto.UnmarshalVT(m.Data)
}

// RpcErrCode represents RPC result code.
// RpcErrCode 表示 RPC 返回状态码。
type RpcErrCode int32

const (
	// ErrCode_Success means RPC call succeeded.
	// ErrCode_Success 表示 RPC 调用成功。
	ErrCode_Success RpcErrCode = iota
	// ErrorCode_RpcTimeOut means RPC wait timed out.
	// ErrorCode_RpcTimeOut 表示 RPC 等待超时。
	ErrorCode_RpcTimeOut
	// ErrorCode_Serialize means request serialization failed.
	// ErrorCode_Serialize 表示请求序列化失败。
	ErrorCode_Serialize
	// ErrorCode_DeSerialize means response deserialization failed.
	// ErrorCode_DeSerialize 表示响应反序列化失败。
	ErrorCode_DeSerialize
	// ErrorCode_RpcErr means generic RPC error.
	// ErrorCode_RpcErr 表示通用 RPC 错误（例如槽位分配失败）。
	ErrorCode_RpcErr
)

// RpcReply is the standard return envelope of an RPC call.
// RpcReply 表示一次 RPC 调用的标准返回体。
type RpcReply struct {
	Code RpcErrCode
	Msg  string
	Data IMessage
}
