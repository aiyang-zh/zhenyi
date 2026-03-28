package zactor

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/aiyang-zh/zhenyi-base/zerrs"
	"github.com/aiyang-zh/zhenyi/ziface"
	"github.com/aiyang-zh/zhenyi/zmodel"
	"github.com/aiyang-zh/zhenyi/zmsg"
)

// HandleRegistry manages client msgId -> handler mappings.
// HandleRegistry 客户端消息处理器注册表，管理 msgId → handler 的映射。
type HandleRegistry struct {
	actor     ziface.IActor
	msgIdList map[int32]int32
	handlers  map[int32]ziface.Handle
	observers map[int32]*handlerObserver // Per-handler observability (readonly after init) / per-handler 可观测性（初始化后只读）
	actorId   uint64
	actorType uint32
}

// NewHandleRegistry creates a handle registry for one actor.
// NewHandleRegistry 为指定 Actor 创建消息处理器注册表。
func NewHandleRegistry(actor ziface.IActor) *HandleRegistry {
	return &HandleRegistry{
		actor:     actor,
		msgIdList: make(map[int32]int32),
		handlers:  make(map[int32]ziface.Handle),
		observers: make(map[int32]*handlerObserver),
		actorId:   actor.GetActorId(),
		actorType: actor.GetActorType(),
	}
}

// GetMsgIdList returns all registered client msg IDs.
// GetMsgIdList 返回已注册的客户端消息 ID 集合。
func (h *HandleRegistry) GetMsgIdList() map[int32]int32 {
	return h.msgIdList
}

// AddMsgId records one supported message ID.
// AddMsgId 记录一个支持的消息 ID。
func (h *HandleRegistry) AddMsgId(msgId int32) {
	h.msgIdList[msgId] = 1
}

// RegisterHandle registers one client handler by msg ID.
// RegisterHandle 按消息 ID 注册一个客户端处理器。
func (h *HandleRegistry) RegisterHandle(msgId int32, handle ziface.Handle) {
	if _, ok := h.handlers[msgId]; ok {
		h.actor.GetLogger().Warn("RegisterHandle: duplicate msgId, skipping",
			zap.Int32("msgId", msgId))
		return
	}
	h.handlers[msgId] = handle
	h.msgIdList[msgId] = 1
	h.observers[msgId] = newHandlerObserver(h.actorId, h.actorType, msgId)
	// Route registration is completed in InitActors (AddActor before RegisterRoutes).
	// 路由注册在 InitActors 中完成（先 AddActor 再 RegisterRoutes，确保有 Group）。
}

// GetClientHandle returns handler for the given msg ID.
// GetClientHandle 返回指定消息 ID 对应的处理器。
func (h *HandleRegistry) GetClientHandle(msgId int32) ziface.Handle {
	if entry, ok := h.handlers[msgId]; ok {
		return entry
	}
	return nil
}

// HandleClientMessage handles client messages.
// HandleClientMessage 处理客户端消息。
// Four-dimensional observability is inlined with zero closure allocation.
// 四维可观测性（内联，零闭包分配）。
func (h *HandleRegistry) HandleClientMessage(ctx context.Context, msg *zmsg.Message) {
	handle, ok := h.handlers[msg.MsgId]
	if !ok {
		h.actor.GetLogger().Error("Message handler not found",
			zap.Int32("msgId", msg.MsgId),
			zap.String("actorTopic", h.actor.GetTopic()),
			zap.Error(zerrs.Newf(zerrs.ErrTypeNotFound, "no handler registered for msgId %d", msg.MsgId)))
		return
	}

	obs := h.observers[msg.MsgId]
	start := time.Now()

	var endSpan func()
	if isTraceEnabled() {
		ctx, endSpan = traceStartSpan(ctx, obs.spanName)
	}

	// Execute business handler.
	// 执行业务 Handler。
	handle(ctx, msg)

	// [Prometheus] per-handler 指标
	cost := time.Since(start)
	obs.metric.RecordCall(cost)

	// [Trace] 结束 Span
	if endSpan != nil {
		endSpan()
	}

	// per-handler 慢日志
	if cost > zmodel.GetFrameworkTuning().SlowLogThreshold {
		h.actor.GetLogger().Warn("Slow client handler",
			zap.Int32("msgId", msg.MsgId),
			zap.Duration("cost", cost),
			zap.Uint64("traceIdHi", msg.TraceIdHi))
	}
}
