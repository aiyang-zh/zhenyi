package ziface

import (
	"context"

	"github.com/aiyang-zh/zhenyi/zmsg"
)

// MsgHandlerFunc defines a handler signature with optional reply.
// MsgHandlerFunc 消息处理函数，入参 ctx/msg，返回回复消息（nil 表示不回复）。
type MsgHandlerFunc func(ctx context.Context, msg *zmsg.Message) IMessage

// IDispatcher dispatches messages to registered handlers by msgId.
// IDispatcher 按 msgId 分发到注册的 Handler，仅 handler 单一路径。
type IDispatcher interface {
	Register(msgId int32, handler MsgHandlerFunc)
	RegisterBatch(handlers map[int32]MsgHandlerFunc)
	Dispatch(ctx context.Context, msg *zmsg.Message)
}
