package ziface

import (
	"context"
	"time"

	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi/zmodel"
	"github.com/aiyang-zh/zhenyi/zmsg"
)

// IActorConfig defines read-only accessors for Actor config.
// IActorConfig 定义 Actor 配置读取能力。
type IActorConfig interface {
	GetTopic() string
	GetNameTopic() string
	GetActorId() uint64
	GetActorType() uint32
}

// RpcCallSpec describes one CallActor invocation; Reply MUST be an independent instance
// (do not share the same Reply pointer across multiple specs).
// RpcCallSpec 描述一次 CallActor；Reply 必须为**独立实例**（不可多个 spec 共用同一 Reply 指针）。
type RpcCallSpec struct {
	ActorID uint64
	Request IMessage
	Reply   IMessage
}

// ISendMsg defines outbound messaging capabilities of an Actor.
// ISendMsg 定义 Actor 对外消息发送能力。
type ISendMsg interface {
	SendMsg(msg *zmsg.Message)
	SendActor(actorId uint64, msg IMessage)
	CallActor(actorId uint64, request IMessage, reply IMessage, timeout time.Duration) RpcReply
	// CallActorAll concurrently issues multi-way RPC calls and blocks until all return (or each times out);
	// the result indices match the input specs.
	// Note: DO NOT call this in the current Actor mailbox thread and block-wait, otherwise callbacks may need
	// mailbox delivery and can deadlock. Use non-mailbox goroutines (e.g., inside AsyncRun) instead.
	//
	// CallActorAll 并发发起多路 RPC，阻塞至全部返回（或各自超时）；结果下标与 specs 一致。
	// 注意：不要在本 Actor 邮箱线程中阻塞等待，否则回调投递可能造成自锁；应在非邮箱线程（例如 AsyncRun 的工作协程）调用。
	CallActorAll(specs []RpcCallSpec, timeout time.Duration) []RpcReply
	SendActorReply(msg *zmsg.Message, reply IMessage)
	Broadcast(topic string, msg IMessage) error
}

// IActor defines the core runtime contract of an Actor.
// IActor 定义 Actor 运行时核心契约。
type IActor interface {
	IMessageHandler
	IActorConfig
	ISendMsg
	ISender
	SetIActor(iActor IActor)
	SetGroup(group IGroup)
	// GetGroup returns the owner group of this actor.
	// GetGroup 返回当前 Actor 所属的 Group（例如在 Actor 内获取脚本引擎：a.GetGroup().GetScriptEngine(ziface.ScriptEngineLua)）。
	GetGroup() IGroup
	Init(ctx context.Context) error
	Push(msg zmodel.ActorCmd)
	// MarkTickPending performs CAS(false->true) for Tick coalescing to prevent mailbox backlog.
	// MarkTickPending CAS(false->true)，用于 Tick 合并，防止 Tick 在 mailbox 中堆积。
	MarkTickPending() bool
	GetMsgList() map[int32]int32
	GetLogger() *zlog.Logger
	Update(ctx context.Context, nowTs int64)
	GetActorConfig() zmodel.ActorConfig
	RegisterTickFn(string, time.Duration, func(ctx context.Context, nowTs int64))
	Close(ctx context.Context) error
	SetInitServer(initServer func(ctx context.Context) error)
	CallInitServer(ctx context.Context) error
	HandleClientMessage(ctx context.Context, msg *zmsg.Message)
	SafeHandleMessage(ctx context.Context, msg zmodel.ActorCmd, nowTs int64)
	Run(ctx context.Context)
	SelectActor(actorType uint32) zmodel.ActorConfig
}

// IServerActor is for Actors requiring an explicit server start phase.
// IServerActor 仅用于需要显式“服务启动阶段”的 Actor。
// 普通纯逻辑 Actor 可以只实现 IActor 而无需实现 RunServer。
type IServerActor interface {
	IActor
	RunServer(ctx context.Context) error
}

// LocalRouter abstracts in-process message routing strategy.
// LocalRouter 抽象本进程内的消息路由策略。
// 框架侧提供默认实现，业务可按需替换以支持更复杂的分片/粘性路由。
type LocalRouter interface {
	// RouteLocal chooses a target Actor in current process by message and group.
	// RouteLocal 根据消息内容和当前 Group，选择一个本进程内的目标 Actor。
	// 若无可用 Actor 或路由失败，应返回非 nil error。
	RouteLocal(group IGroup, msg *zmsg.Message) (IActor, error)
}
