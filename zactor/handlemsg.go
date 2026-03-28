package zactor

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aiyang-zh/zhenyi-base/zpool"
	"github.com/aiyang-zh/zhenyi-base/ztime"
	"github.com/aiyang-zh/zhenyi/zmetrics"
	"github.com/aiyang-zh/zhenyi/zmodel"
	"github.com/aiyang-zh/zhenyi/zmsg"
	"github.com/aiyang-zh/zhenyi/zpoolobs"
	"go.uber.org/zap"
)

// asyncTask is the pooled async-task container used to avoid closure allocations.
// asyncTask 异步任务结构体（池化，避免闭包分配）
type asyncTask struct {
	actor        *Actor
	msg          *zmsg.Message                   // Used by AsyncRunWithMsg / AsyncRunWithMsg 用
	fMsg         func(*zmsg.Message) interface{} // Used by AsyncRunWithMsg / AsyncRunWithMsg 用
	fSimple      func() interface{}              // Used by AsyncRun / AsyncRun 用
	callBackFunc func(interface{})
	result       interface{}
	validators   []func() bool
}

var (
	asyncTaskPoolOnce sync.Once
	asyncTaskPool     *zpool.Pool[*asyncTask]
)

func initAsyncTaskPool() {
	asyncTaskPoolOnce.Do(func() {
		asyncTaskPool = zpoolobs.NewObservedPool(zpoolobs.PoolNameZActorAsyncTask, func() *asyncTask {
			return &asyncTask{}
		})
	})
}

func getAsyncTask() *asyncTask {
	initAsyncTaskPool()
	return asyncTaskPool.Get()
}

func putAsyncTask(t *asyncTask) {
	t.actor = nil
	t.msg = nil
	t.fMsg = nil
	t.fSimple = nil
	t.callBackFunc = nil
	t.result = nil
	t.validators = t.validators[:0]
	initAsyncTaskPool()
	asyncTaskPool.Put(t)
}

// runWithMsg executes async tasks that carry a message (for AsyncRunWithMsg).
// runWithMsg 执行带消息的异步任务（AsyncRunWithMsg）
func (t *asyncTask) runWithMsg() {
	defer t.actor.GetLogger().Recover("Actor AsyncRunWithMsg")

	t.result = t.fMsg(t.msg)

	if t.callBackFunc != nil {
		t.actor.Push(zmodel.ActorCmd{Type: zmodel.CmdTypeAsync, Any: t})
	} else {
		t.msg.Release()
		putAsyncTask(t)
	}
}

// runSimple executes simple async tasks (for AsyncRun).
// runSimple 执行简单异步任务（AsyncRun）
func (t *asyncTask) runSimple() {
	defer t.actor.GetLogger().Recover("Actor AsyncRun")

	t.result = t.fSimple()

	if t.callBackFunc != nil {
		t.actor.Push(zmodel.ActorCmd{Type: zmodel.CmdTypeAsync, Any: t})
	} else {
		putAsyncTask(t)
	}
}

func (t *asyncTask) runCallbackOnActor() {
	a := t.actor
	if a == nil {
		putAsyncTask(t)
		return
	}
	if t.msg != nil {
		defer t.msg.Release()
	}
	for i, validator := range t.validators {
		if !validator() {
			if t.msg != nil {
				a.GetLogger().Warn("AsyncRunWithMsg: callback-check failed, callback skipped",
					zap.Int("validatorIndex", i),
					zap.Uint64("traceIdHi", t.msg.TraceIdHi),
					zap.Int32("msgId", t.msg.MsgId))
			} else {
				a.GetLogger().Warn("runSimple: callback-check failed, callback skipped",
					zap.Int("validatorIndex", i))
			}
			putAsyncTask(t)
			return
		}
	}
	t.callBackFunc(t.result)
	putAsyncTask(t)
}

// AsyncRunWithMsg executes async work with optional business-condition checks.
// AsyncRunWithMsg 带业务条件检查的异步执行。
//
// Problem this solves:
// 解决问题：
// During async RPC/DB operations, actor business state may be modified by other messages.
// 在异步 RPC/DB 操作期间，Actor 的业务数据可能被其他消息修改，
// This can invalidate callback preconditions by the time callback execution happens (e.g., insufficient gold or status changes).
// 导致回调执行时基于的前提条件已经失效（例如金币不足、状态变化等）
//
// Parameters:
// Parameter details:
// 参数说明：
//   - msg: message object (auto Retain/Release)
//   - msg: 消息对象（会自动 Retain/Release）
//   - f: async work function (usually RPC/DB calls)
//   - f: 异步执行的函数（通常是 RPC/DB 调用）
//   - callBackFunc: callback function, signature func(result interface{})
//   - callBackFunc: 回调函数，签名为 func(result interface{})
//   - validator: optional business-condition check function; return true means condition satisfied
//   - validator: 可选的业务条件检查函数，返回 true 表示条件满足
//   - If provided, the framework calls it once before the async operation starts and once before the callback runs.
//   - 如果提供，框架会在异步操作开始前和回调执行前各调用一次
//   - The callback runs only when both checks return true.
//   - 只有两次检查都返回 true，回调才会执行
//   - If conditions are not met, the framework logs a warning and skips the callback.
//   - 如果条件不满足，框架会记录警告日志，不执行回调
//   - Example: func() bool { return p.Gold >= 100 }
//   - 示例：func() bool { return p.Gold >= 100 }
//
// Example 1: without condition checks (backward compatible).
// 使用示例 1：不带条件检查（向后兼容）。
//
//	p.AsyncRunWithMsg(msg,
//	    func(msg *zmsg.Message) interface{} {
//	        return CallShopService(msg.ItemID)
//	    },
//	    func(result interface{}) {
//	        p.AddItem(result.(int))
//	    },
//	)
//
// Example 2: with condition checks.
// 使用示例 2：带条件检查。
//
//	func (p *PlayerActor) HandleBuyItem(msg *MsgBuyItem) {
//	    requiredGold := 100
//
//	    p.AsyncRunWithMsg(msg,
//	        func(msg *zmsg.Message) interface{} {
//	            // Async RPC call to shop service. / 异步 RPC 到商城服务
//	            return CallShopService(msg.ItemID)
//	        },
//	        func(result interface{}) {
//	            // ✅ Reaching this point means conditions are satisfied (if validator is provided). / ✅ 能执行到这里，说明条件满足（如果提供了 validator）
//	            p.Gold -= requiredGold
//	            p.AddItem(result.(int))
//	        },
//	        func() bool {
//	            // ✅ Condition-check function: defines business condition. / ✅ 检查函数：定义业务条件
//	            return p.Gold >= requiredGold && !p.IsBanned()
//	        },
//	    )
//	}
//
// Notes:
// Note details:
// 注意事项：
//  1. validator is optional; if not provided, no condition checks are performed. / validator 是可选参数，不传时不做条件检查
//  2. validator is called twice: once before the async starts + once before the callback runs. / validator 函数会被调用两次：异步开始前 + 回调执行前
//  3. validator should be a pure function with no side effects (do not modify data inside it). / validator 应该是纯函数，无副作用（不要在其中修改数据）
//  4. when conditions are not met, the callback will be skipped and the framework logs a warning. / 如果条件不满足，回调不会执行，框架会记录警告日志
//  5. do not access Actor data inside `f` (since `f` runs in another goroutine). / 不要在 f 函数中访问 Actor 的数据（f 在另一个 goroutine 执行）
//  6. validator and callBackFunc run on the Actor main thread, so it is safe to access Actor data. / validator 和 callBackFunc 都在 Actor 主线程执行，可以安全访问 Actor 数据
func (a *Actor) AsyncRunWithMsg(msg *zmsg.Message, f func(*zmsg.Message) interface{}, callBackFunc func(interface{}), validators ...func() bool) {
	msg.Retain()

	for i, validator := range validators {
		if !validator() {
			a.GetLogger().Warn("AsyncRunWithMsg: pre-check failed, async operation cancelled",
				zap.Int("validatorIndex", i),
				zap.Uint64("traceIdHi", msg.TraceIdHi),
				zap.Int32("msgId", msg.MsgId))
			msg.Release()
			return
		}
	}

	task := getAsyncTask()
	task.actor = a
	task.msg = msg
	task.fMsg = f
	task.callBackFunc = callBackFunc
	if len(validators) > 0 {
		task.validators = append(task.validators, validators...)
	}

	err := a.workerPool.Invoke(task)
	if err != nil {
		a.GetLogger().Error("AsyncRunWithMsg: failed to submit to workerPool",
			zap.Error(err),
			zap.Int("poolCap", a.workerPool.Cap()),
			zap.Int("poolRunning", a.workerPool.Running()),
			zap.Int("poolFree", a.workerPool.Free()),
			zap.Uint64("traceIdHi", msg.TraceIdHi),
			zap.Int32("msgId", msg.MsgId))
		msg.Release()
		putAsyncTask(task)
	}
}

// AsyncRun is a safe async executor with pooled tasks.
// AsyncRun 安全的异步执行器（池化 task，避免闭包分配）。
func (a *Actor) AsyncRun(f func() interface{}, callBackFunc func(interface{}), validators ...func() bool) {
	task := getAsyncTask()
	task.actor = a
	task.fSimple = f
	task.callBackFunc = callBackFunc
	if len(validators) > 0 {
		task.validators = append(task.validators, validators...)
	}
	err := a.workerPool.Invoke(task)
	if err != nil {
		a.GetLogger().Error("AsyncRun: failed to submit to workerPool",
			zap.Error(err),
			zap.Int("poolCap", a.workerPool.Cap()),
			zap.Int("poolRunning", a.workerPool.Running()),
			zap.Int("poolFree", a.workerPool.Free()))
		putAsyncTask(task)
	}
}

// Core trick: safely write back data via self-message.
// 核心魔法：安全回写数据。
// Package mutation logic as a message and send to self.
// 把修改数据的逻辑打包成消息，发给自己。
func (a *Actor) safeUpdate(fn func()) {
	if fn == nil {
		return
	}
	cmd := zmodel.ActorCmd{
		Type: zmodel.CmdTypeSafeFn,
		Fn:   fn,
	}
	a.Push(cmd)
}
func (a *Actor) safeExecute(fn func()) {
	defer a.GetLogger().Recover("Actor AsyncRun")
	if fn != nil {
		fn()
	}
}

// SafeHandleMessage handles one actor command with monitoring and panic safety.
// SafeHandleMessage 在带监控与 panic 保护的模式下处理一条 Actor 命令。
func (a *Actor) SafeHandleMessage(ctx context.Context, msg zmodel.ActorCmd, nowTs int64) {
	start := ztime.ServerNow()
	atomic.StoreInt64(&a.processingStart, start.UnixNano())
	defer a.GetLogger().RecoverWith("handleMessage panic", func() {
		atomic.StoreInt64(&a.processingStart, 0)
		cost := time.Since(start)

		if msg.Type == zmodel.CmdTypeMsg || msg.Type == zmodel.CmdTypeClient {
			isSlow := cost > zmodel.GetFrameworkTuning().SlowLogThreshold
			a.stats.RecordMessage(cost.Nanoseconds(), isSlow)
			zmetrics.ActorMsgHandled.Inc()
			zmetrics.ActorMsgLatency.ObserveDuration(cost)
		} else if msg.Type == zmodel.CmdTypeTick {
			zmetrics.ActorTickCount.Inc()
			zmetrics.ActorTickLatency.ObserveDuration(cost)
		}

		if cost > zmodel.GetFrameworkTuning().SlowLogThreshold {
			switch msg.Type {
			case zmodel.CmdTypeMsg, zmodel.CmdTypeClient:
				if msg.Msg != nil {
					a.GetLogger().Warn("Slow actor message handled",
						zap.Uint64("workerID", a.Id),
						zap.Uint8("cmdType", msg.Type),
						zap.Duration("cost", cost),
						zap.Int32("msgId", msg.Msg.MsgId),
						zap.Uint64("traceIdHi", msg.Msg.TraceIdHi))
				}
			case zmodel.CmdTypeTick:
				a.GetLogger().Warn("Slow actor tick message handled", zap.Uint64("workerID", a.Id),
					zap.Uint8("cmdType", msg.Type),
					zap.Duration("cost", cost))
			case zmodel.CmdTypeSafeFn:
				a.GetLogger().Warn("Slow actor safe fn message handled", zap.Uint64("workerID", a.Id),
					zap.Uint8("cmdType", msg.Type),
					zap.Duration("cost", cost))
			case zmodel.CmdTypeTickFn:
				a.GetLogger().Warn("Slow actor update message handled", zap.Uint64("workerID", a.Id),
					zap.Uint8("cmdType", msg.Type),
					zap.Duration("cost", cost))
			}
		}
	}, zap.Uint64("workerID", a.Id), zap.Uint8("type", msg.Type))

	// Prefer message context (trace/cancel chain); fallback to actor lifecycle ctx.
	// 优先使用消息自带的 context（携带调用链 trace/cancel），否则使用 Actor 生命周期 ctx。
	msgCtx := ctx
	if msg.Ctx != nil {
		msgCtx = msg.Ctx
	}

	switch msg.Type {
	case zmodel.CmdTypeTick:
		a.tickPending.Store(false)
		a.Update(msgCtx, nowTs)
	case zmodel.CmdTypeMsg, zmodel.CmdTypeClient:
		if isTraceEnabled() && msg.Msg != nil {
			parentCtx := traceContextFromMsg(msgCtx, msg.Msg)
			var endSpan func()
			msgCtx, endSpan = traceStartSpan(parentCtx, "actor.handle")
			defer endSpan()
		}
		a.handleMessage(msgCtx, msg.Msg)
	case zmodel.CmdTypeSafeFn:
		a.safeExecute(msg.Fn)
	case zmodel.CmdTypeTickFn:
		a.registerTickFn(msg.TickFn)
	case zmodel.CmdTypeAsync:
		if t, ok := msg.Any.(*asyncTask); ok && t != nil {
			t.runCallbackOnActor()
		}
	}
}

// HandleClientMessage handles one client-side message.
// HandleClientMessage 处理一条客户端侧消息。
func (a *Actor) HandleClientMessage(ctx context.Context, msg *zmsg.Message) {
	a.GetHandleMgr().HandleClientMessage(ctx, msg)
}

// HandleMessage processes non-response messages in the high-performance path (no monitoring overhead).
// HandleMessage 消息处理（高性能路径，无监控开销）
func (a *Actor) HandleMessage(ctx context.Context, msg *zmsg.Message) {
	if msg.FromClient {
		a.iActor.HandleClientMessage(ctx, msg)
	} else {
		a.GetDispatcher().Dispatch(ctx, msg)
	}
}

// HandleRespMessage handles response messages.
// HandleRespMessage 响应消息处理
func (a *Actor) HandleRespMessage(ctx context.Context, msg *zmsg.Message) {
	if msg.IsResponse {
		a.SetReply(msg)
		return
	}
}

// handleMessage is the internal message dispatcher with optional monitoring/tracing.
// handleMessage 内部消息处理（带可选监控）
func (a *Actor) handleMessage(ctx context.Context, msg *zmsg.Message) {
	if msg.IsResponse {
		if !msg.ToClient {
			a.SetReply(msg)
			return
		}
		a.iActor.HandleRespMessage(ctx, msg)
		return
	}
	a.iActor.HandleMessage(ctx, msg)
}
