package zactor

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/aiyang-zh/zhenyi/ziface"
	"github.com/aiyang-zh/zhenyi/zmetrics"
	"github.com/aiyang-zh/zhenyi/zmodel"
	"github.com/aiyang-zh/zhenyi/zmsg"
)

// Dispatcher routes messages to handlers (single handler path).
// Dispatcher 消息分发器（仅 Handler 单一路径）。
// Lock-free design:
// 无锁设计：
//   - Register*() 方法仅在 Actor.Init() 阶段调用（单线程）
//   - Dispatch() 方法在 Actor.Run() 阶段调用（多线程只读）
//   - Go map 支持并发读，因此无需锁保护
//
// Built-in business observability (four dimensions, zero closure allocation):
// 业务层自动可观测性（四维度，零闭包分配）：
//   - Prometheus per-handler 指标（~8ns: atomic inc + histogram observe）
//   - Trace 子 Span（gated by isTraceEnabled，未启用时 0 开销）
//   - Logger Context 注入（业务代码通过 zlog.TraceFieldsFromContext 获取 trace 元数据）
//   - Pyroscope 天然覆盖（进程级 CPU 采样自动包含 handler 调用栈）
type Dispatcher struct {
	ziface.IActor
	handlers        map[int32]ziface.MsgHandlerFunc
	observers       map[int32]*handlerObserver
	monitoringLabel string
	actorId         uint64
	actorType       uint32
}

// NewDispatcher creates a dispatcher bound to one actor.
// NewDispatcher 创建并返回绑定到指定 Actor 的分发器。
func NewDispatcher(a ziface.IActor) *Dispatcher {
	return &Dispatcher{
		handlers:        make(map[int32]ziface.MsgHandlerFunc),
		IActor:          a,
		observers:       make(map[int32]*handlerObserver),
		monitoringLabel: fmt.Sprintf("[%s] Dispatcher.Dispatch", a.GetTopic()),
		actorId:         a.GetActorId(),
		actorType:       a.GetActorType(),
	}
}

// Register is init-only; do not call during runtime.
// Register 方法仅在 Init 阶段调用，禁止在运行时调用。
func (d *Dispatcher) Register(msgId int32, handler ziface.MsgHandlerFunc) {
	d.handlers[msgId] = handler
	d.observers[msgId] = newHandlerObserver(d.actorId, d.actorType, msgId)
}

// RegisterBatch registers handlers in batch during init phase only.
// RegisterBatch 批量注册 Handler，仅在 Init 阶段调用。
func (d *Dispatcher) RegisterBatch(handlers map[int32]ziface.MsgHandlerFunc) {
	for msgId, handler := range handlers {
		d.handlers[msgId] = handler
		d.observers[msgId] = newHandlerObserver(d.actorId, d.actorType, msgId)
	}
}

// Dispatch routes message to the corresponding handler.
// Dispatch 分发消息到对应的 Handler。
// Lock-free: Go map supports concurrent reads; handlers/observers are readonly after init.
// 无锁实现：Go map 支持并发读，初始化后 handlers/observers 只读。
// Observability is fully inlined with zero closure allocation.
// 四维可观测性全部内联，零闭包分配。
//
// Perf overhead (tracing off): ~48ns = time.Now(20) + time.Since(20) + RecordCall(8).
// 性能开销（tracing 未启用）：~48ns = time.Now(20) + time.Since(20) + RecordCall(8)。
// Perf overhead (tracing on): + StartSpan + context.WithValue ~= 250ns (only on sampled hits).
// 性能开销（tracing 已启用）：+ StartSpan + context.WithValue ≈ 250ns（仅采样命中时）。
func (d *Dispatcher) Dispatch(ctx context.Context, msg *zmsg.Message) {
	msgId := msg.MsgId
	handler, ok := d.handlers[msgId]
	if !ok {
		d.GetLogger().Warn("unregistered msgId, message dropped",
			zap.Int32("msgId", msgId),
			zap.String("actor", d.monitoringLabel))
		zmetrics.ActorMsgDropped.Inc()
		return
	}

	obs := d.observers[msgId]
	start := time.Now()

	var endSpan func()
	if isTraceEnabled() {
		ctx, endSpan = traceStartSpan(ctx, obs.spanName)
	}

	ret := handler(ctx, msg)

	cost := time.Since(start)
	obs.metric.RecordCall(cost)

	if endSpan != nil {
		endSpan()
	}

	if cost > zmodel.GetFrameworkTuning().SlowLogThreshold {
		d.GetLogger().Warn("Slow handler",
			zap.Int32("msgId", msgId),
			zap.Duration("cost", cost),
			zap.Uint64("traceIdHi", msg.TraceIdHi))
	}

	if ret != nil {
		d.SendActorReply(msg, ret)
	}
}
