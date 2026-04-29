package zactor

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/panjf2000/ants/v2"
	"go.uber.org/zap"

	"github.com/aiyang-zh/zhenyi-base/zbackoff"
	"github.com/aiyang-zh/zhenyi-base/zbatch"
	"github.com/aiyang-zh/zhenyi-base/zerrs"
	baseziface "github.com/aiyang-zh/zhenyi-base/ziface"
	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi-base/zqueue"
	"github.com/aiyang-zh/zhenyi-base/ztime"
	"github.com/aiyang-zh/zhenyi/zbus"
	"github.com/aiyang-zh/zhenyi/ziface"
	"github.com/aiyang-zh/zhenyi/zmetrics"
	"github.com/aiyang-zh/zhenyi/zmodel"
	"github.com/aiyang-zh/zhenyi/zmonitor"
	"github.com/aiyang-zh/zhenyi/zmsg"
)

// Actor is the core runtime unit; each actor behaves like a service.
// Actor 每个 actor 都是一个服务。
type Actor struct {
	ziface.IGroup
	ziface.ISender
	zmodel.ActorConfig
	dispatcher            *Dispatcher
	handle                *HandleRegistry
	logger                *zlog.Logger
	iActor                ziface.IActor
	toClientFastPath      ziface.IToClientFastPath
	tickFns               map[string]*zmodel.TickFnItem
	tickEnabled           atomic.Bool // Whether group tick is required / 是否需要参与 Group tick（注册过 TickFn 才需要）
	initServer            func(ctx context.Context) error
	handleMonitoringTopic string
	mailBoxQueue          *zqueue.UnboundedMPSC[zmodel.ActorCmd]
	workerPool            *ants.PoolWithFunc
	closeCh               chan struct{}
	closeOnce             sync.Once
	mailCount             int64
	processingStart       int64 // atomic: nanotime when handler started (0 = idle)
	stats                 *zmonitor.ActorStats
	batcher               *zbatch.FastAdaptiveBatcher
	tickPending           atomic.Bool                // Prevent Tick accumulation in mailbox / 防止 Tick 在 mailbox 中堆积
	circuitBreakers       map[uint64]*circuitBreaker // targetActorId → CB (Run 单线程访问)

	// Lifecycle ctx derives in Init and cancels in Close for graceful background exit.
	// 生命周期 ctx：Init 时派生，Close 时 cancel，确保后台任务（sender/watchdog/订阅等）可退出。
	ctx    context.Context
	cancel context.CancelFunc

	// Remote-bus subscription handles; unsubscribed in Close to avoid leaks.
	// 远端总线订阅句柄：Close 时取消订阅，避免泄漏。
	subsMu sync.Mutex
	subs   []zbus.Subscription

	// Optional injection: pool observer used by current actor.
	// 可选注入：当前 Actor 使用的对象池观测器（如 Sender 的 timer 池）。
	// If unset, resolve falls back to global observer (usually installed by zmetrics.Enable).
	// 未设置时由 zpoolobs.Resolve(nil) 回退到全局 GetObserver()（通常由 zmetrics.Enable 安装）。
	poolObserver baseziface.IPoolObserver
}

func NewActor(actorConfig zmodel.ActorConfig) *Actor {
	topic := actorConfig.GetTopic()

	tuning := zmodel.GetFrameworkTuning()
	// Create async worker pool for AsyncRunWithMsg/AsyncRun.
	// 创建异步业务协程池（用于AsyncRunWithMsg/AsyncRun）。
	poolSize := int(actorConfig.WorkSize)
	if poolSize <= 0 {
		poolSize = tuning.ActorWorkSizeDefault
	}
	workerPool, err := ants.NewPoolWithFunc(poolSize, func(arg interface{}) {
		switch task := arg.(type) {
		case *asyncTask:
			if task == nil {
				return
			}
			if task.flowWork != nil {
				task.runFlow()
				return
			}
			if task.msg != nil && task.fMsg != nil {
				task.runWithMsg()
				return
			}
			task.runSimple()
		}
	},
		ants.WithPreAlloc(true),     // Pre-allocate to reduce runtime allocations / 预分配，减少运行时分配
		ants.WithNonblocking(false), // Blocking mode when pool is full / 阻塞模式：池满时等待，而不是返回错误
		ants.WithPanicHandler(func(err interface{}) {
			zmetrics.ActorPanicCount.Inc()
			zlog.Error("WorkerPool panic recovered",
				zap.String("topic", topic),
				zap.Any("panic", err))
		}),
	)
	if err != nil {
		// In normal environments, ants.NewPool failure usually means severe resource shortage.
		// 在绝大多数正常环境中，ants.NewPool 失败意味着进程已严重资源不足。
		// Log clearly and return nil so upper layer decides controlled exit.
		// 这里打出清晰的错误日志并返回 nil，由上层在可控位置决定是否退出。
		zlog.Error("Failed to create worker pool", zap.String("topic", topic), zap.Int("poolSize", poolSize), zap.Error(err))
		return nil
	}
	zlog.Info("Actor created with worker pool",
		zap.String("name", actorConfig.Name),
		zap.Int("poolSize", poolSize))

	zmetrics.EnsureActorPanicHook()

	actor := Actor{
		ActorConfig:           actorConfig,
		logger:                zlog.CloneDefaultLog(topic),
		tickFns:               make(map[string]*zmodel.TickFnItem),
		handleMonitoringTopic: topic,
		closeCh:               make(chan struct{}, 1),
		mailBoxQueue:          zqueue.NewUnboundedMPSC[zmodel.ActorCmd](),
		workerPool:            workerPool,
		stats:                 zmonitor.NewActorStats(),
		circuitBreakers:       make(map[uint64]*circuitBreaker),
		batcher: zbatch.NewFastAdaptiveBatcher(
			tuning.ActorBatchMin,
			tuning.ActorBatchMax,
			tuning.ActorBatchTargetP99,
		),
	}

	actor.handle = NewHandleRegistry(&actor)
	actor.dispatcher = NewDispatcher(&actor)
	return &actor
}

// SetGroup binds the owner group of the actor.
// SetGroup 绑定当前 Actor 所属的 Group。
func (a *Actor) SetGroup(group ziface.IGroup) {
	a.IGroup = group
}

// GetGroup returns the owner group of the actor.
// GetGroup 返回当前 Actor 所属的 Group。
func (a *Actor) GetGroup() ziface.IGroup {
	return a.IGroup
}

// SetIActor sets the business-facing actor implementation.
// SetIActor 设置业务侧 Actor 实现。
func (a *Actor) SetIActor(iActor ziface.IActor) {
	a.iActor = iActor
	if fast, ok := any(iActor).(ziface.IToClientFastPath); ok {
		a.toClientFastPath = fast
	} else {
		a.toClientFastPath = nil
	}
}

// SetPoolObserver injects pool observer for current actor.
// SetPoolObserver 为当前 Actor 注入对象池观测器。
// Call before Init; sender initialization reads this value.
// 需在 Init 之前调用，后续 sender 初始化会读取该值。
func (a *Actor) SetPoolObserver(obs baseziface.IPoolObserver) {
	a.poolObserver = obs
}

// MarkTickPending performs CAS(false->true) for Tick coalescing.
// MarkTickPending CAS(false→true)。返回 true 表示成功标记（之前无 Tick 排队），应 Push。
// False means a Tick is already queued, so current push is skipped to avoid backlog.
// 返回 false 表示已有 Tick 在队列中，跳过本次 Push 以防止堆积。
func (a *Actor) MarkTickPending() bool {
	return a.tickPending.CompareAndSwap(false, true)
}

// GetLogger returns the logger bound to current actor topic.
// GetLogger 返回绑定到当前 Actor topic 的日志器。
func (a *Actor) GetLogger() *zlog.Logger {
	return a.logger
}

// GetDispatcher returns the message dispatcher of current actor.
// GetDispatcher 返回当前 Actor 的消息分发器。
func (a *Actor) GetDispatcher() ziface.IDispatcher {
	return a.dispatcher
}

// GetActorConfig returns the static config of the actor.
// GetActorConfig 返回当前 Actor 的静态配置。
func (a *Actor) GetActorConfig() zmodel.ActorConfig {
	return a.ActorConfig
}

// GetMsgList returns registered message IDs from the handler registry.
// GetMsgList 返回 handler 注册表中的消息 ID 列表。
func (a *Actor) GetMsgList() map[int32]int32 {
	return a.handle.GetMsgIdList()
}

// GetBroadcastTopic returns the default broadcast topic name.
// GetBroadcastTopic 返回默认广播 topic 名称。
func (a *Actor) GetBroadcastTopic() string {
	return "topic_broadcast"
}

// receiveRemote consumes cross-process messages from remote bus.
// receiveRemote 从远端消息总线接收消息（跨进程）。
// TopicBus adapts underlying implementations (NATS/other MQ).
// 由 zbus.TopicBus 适配底层实现（NATS / 其他 MQ）。
func (a *Actor) receiveRemote(topic string, data []byte) {
	start := ztime.ServerNowUnixMilli()
	msgData := zmsg.GetMessage()
	err := msgData.Unmarshal(data)
	if err != nil {
		msgData.Release() // Release on decode failure / 失败时释放
		a.GetLogger().Error("Failed to unmarshal message from actor",
			zap.String("topic", topic),
			zap.Int("dataLen", len(data)),
			zap.Error(zerrs.Wrap(err, zerrs.ErrTypeValidation, "message unmarshal failed")))
		return
	}
	if ztime.ServerNowUnixMilli()-start > 10 {
		a.GetLogger().Debug("receiveMsgFromActor msgId:", zap.Int32("msgId", msgData.MsgId), zap.Int64("time:", ztime.ServerNowUnixMilli()-start))
	}
	a.Push(zmodel.ActorCmd{
		Type: zmodel.CmdTypeMsg,
		Msg:  msgData, // No Retain needed; ownership is transferred / 不需要 Retain，直接转移所有权
	})
}

// Push enqueues one actor command into mailbox, with response fast-path handling.
// Push 将一条 Actor 命令入队到 mailbox，并处理响应快路径。
func (a *Actor) Push(msg zmodel.ActorCmd) {
	zmetrics.ActorMsgRecv.Inc()
	// Common path: non-CmdTypeMsg or non-response message goes directly into mailbox.
	// 常见路径：非 CmdTypeMsg 或非响应消息，直接入队。
	if msg.Type != zmodel.CmdTypeMsg {
		a.mailBoxQueue.Enqueue(msg)
		return
	}
	m := msg.Msg
	if m == nil || !m.IsResponse {
		a.mailBoxQueue.Enqueue(msg)
		return
	}

	// Low-latency fast path for response messages.
	// 响应消息低延迟 fast-path：
	// 这里**只允许**做线程安全的 SetReply（原子 + channel），不能走 SafeHandleMessage，
	// 否则会触碰 Actor 其它状态（统计/日志/字段），破坏 Actor 单线程不变性并引入 data race。
	// RPC 响应（非 to-client）直接投递到 sender slot，避免进入 mailbox 增加延迟
	// ToClient 的响应仍走 mailbox，由业务侧处理。
	if !m.ToClient {
		a.SetReply(m)
		m.Release()
		return
	}
	// Optional ToClient fast path: only when actor explicitly declares thread safety.
	// ToClient 快路径（可选）：仅当具体 Actor 显式声明线程安全时才允许直达处理。
	// 典型实现：Gate 对 ToClient 响应只做 sendClient（查 channel + Send 入队）。
	if a.toClientFastPath != nil {
		if a.toClientFastPath.HandleToClientFastPath(m) {
			m.Release()
			return
		}
	}
	a.mailBoxQueue.Enqueue(msg)
}

// GetHandleMgr returns the actor message handler registry.
// GetHandleMgr 返回 Actor 消息处理器注册表。
func (a *Actor) GetHandleMgr() *HandleRegistry {
	return a.handle
}

// Init initializes runtime dependencies (sender/pubsub) for the actor.
// Init 初始化 Actor 运行期依赖（sender/pubsub）。
func (a *Actor) Init(ctx context.Context) error {
	// 绑定 Actor 生命周期：Close 时 cancel，保证后台 goroutine 能退出
	if ctx == nil {
		return zerrs.New(zerrs.ErrTypeValidation, "zactor.Actor.Init: ctx is required (must come from Group.Run)")
	}
	a.ctx, a.cancel = context.WithCancel(ctx) // #nosec G118 -- cancel is invoked in Actor.Close()
	a.ISender = NewSenderWithObserver(a.ctx, int(a.MaxRPCPending), a.poolObserver)
	a.pubSub()
	return nil
}
func (a *Actor) registerTickFn(f *zmodel.TickFnItem) {
	if f == nil {
		return
	}
	if _, ok := a.tickFns[f.Name]; ok {
		return
	}
	a.tickFns[f.Name] = f
	a.tickEnabled.Store(true)
}

// HasTickFns reports whether actor requires periodic Tick.
// HasTickFns 表示该 Actor 是否需要周期性 Tick 推动 Update。
// It returns true only after TickFn registration.
// 仅在注册过 TickFn 后才返回 true。
func (a *Actor) HasTickFns() bool {
	return a.tickEnabled.Load()
}

// RegisterTickFn registers a periodic tick callback by name and interval.
// RegisterTickFn 按名称和间隔注册周期 Tick 回调。
func (a *Actor) RegisterTickFn(name string, interval time.Duration, f func(ctx context.Context, nowTs int64)) {
	a.Push(zmodel.ActorCmd{
		Type:   zmodel.CmdTypeTickFn,
		TickFn: zmodel.NewTickFnItem(name, interval, f),
	})
}

// Update executes due tick callbacks.
// Update 执行到期的 Tick 回调。
func (a *Actor) Update(ctx context.Context, nowTs int64) {
	for _, v := range a.tickFns {
		if v.Interval > 0 {
			if nowTs < v.LastTime {
				continue
			}
			v.LastTime = nowTs + v.Interval.Milliseconds()
		}
		startTime := time.Now()
		v.Do(ctx, nowTs)
		elapsed := time.Since(startTime)
		if elapsed > 100*time.Millisecond {
			a.GetLogger().Warn("slow update task",
				zap.String("func", v.Name),
				zap.Duration("elapsed", elapsed))
		}
		if elapsed > 1*time.Second {
			a.GetLogger().Error("very slow update task",
				zap.String("func", v.Name),
				zap.Duration("elapsed", elapsed))
		}
	}

}

// Close stops background resources and releases actor-owned components.
// Close 停止后台资源并释放 Actor 持有组件。
func (a *Actor) Close(ctx context.Context) error {
	a.closeOnce.Do(func() {
		if a.cancel != nil {
			a.cancel()
		}
		// 取消远端订阅（尽力而为）
		a.subsMu.Lock()
		subs := a.subs
		a.subs = nil
		a.subsMu.Unlock()
		for _, sub := range subs {
			if sub != nil {
				_ = sub.Unsubscribe()
			}
		}
		close(a.closeCh)
		if a.mailBoxQueue != nil {
			a.mailBoxQueue.Close()
		}
		if a.workerPool != nil {
			a.workerPool.Release()
			a.GetLogger().Info("Actor worker pool released")
		}
	})
	return nil
}

// SetInitServer sets optional server-init hook for this actor.
// SetInitServer 设置当前 Actor 的可选服务初始化钩子。
func (a *Actor) SetInitServer(initServer func(ctx context.Context) error) {
	a.initServer = initServer
}

// CallInitServer executes optional server-init hook if configured.
// CallInitServer 在已配置时执行可选服务初始化钩子。
func (a *Actor) CallInitServer(ctx context.Context) error {
	if a.initServer == nil {
		return nil
	}
	return a.initServer(ctx)
}

// GetProcessingStart returns the nanotime when the current handler started.
// GetProcessingStart 返回当前处理开始的纳秒时间戳；空闲时返回 0，用于 watchdog 阻塞检测。
func (a *Actor) GetProcessingStart() int64 {
	return atomic.LoadInt64(&a.processingStart)
}

// GetWorkerPoolStats returns async worker-pool statistics for observability.
// GetWorkerPoolStats 返回异步协程池统计信息（用于观测）。
func (a *Actor) GetWorkerPoolStats() (cap, running, free int) {
	if a.workerPool == nil {
		return 0, 0, 0
	}
	return a.workerPool.Cap(), a.workerPool.Running(), a.workerPool.Free()
}

// LogWorkerPoolStats logs async worker-pool runtime stats.
// LogWorkerPoolStats 记录异步协程池运行状态（调试用）。
func (a *Actor) LogWorkerPoolStats() {
	if a.workerPool == nil {
		return
	}
	capCount, running, free := a.GetWorkerPoolStats()
	usage := float64(running) / float64(capCount) * 100

	zmetrics.ActorPoolCapacity.Set(int64(capCount))
	zmetrics.ActorPoolRunning.Set(int64(running))

	if usage > 80 {
		a.GetLogger().Warn("WorkerPool high usage",
			zap.Int("capacity", capCount),
			zap.Int("running", running),
			zap.Int("free", free),
			zap.Float64("usage%", usage))
	} else {
		a.GetLogger().Info("WorkerPool stats",
			zap.Int("capacity", capCount),
			zap.Int("running", running),
			zap.Int("free", free),
			zap.Float64("usage%", usage))
	}
}

//	func (a *Actor) RunServer(ctx context.Context) error {
//		panic("not implemented")
//	}
//
// Run starts actor main loop and processes mailbox commands until graceful exit.
// Run 启动 Actor 主循环并处理 mailbox 命令，直到优雅退出。
func (a *Actor) Run(ctx context.Context) {
	defer a.GetLogger().Recover("Group processActor")
	a.GetLogger().Info("Actor Run", zap.Uint64("workerIdx", a.Id))
	maxBatchSize := zmodel.GetFrameworkTuning().ActorBatchMax
	if maxBatchSize <= 0 {
		maxBatchSize = 200
	}
	msgs := make([]zmodel.ActorCmd, maxBatchSize)

	idleCount := 0
	lastBatchSize := 1
	processedTotal := int64(0)
	lastSyncCount := int64(0)

	var shouldExit bool
	handleCtx := ctx

	for {
		if !shouldExit {
			select {
			case <-ctx.Done():
				shouldExit = true
				a.GetLogger().Info("Actor received shutdown signal, draining queue...",
					zap.Uint64("workerIdx", a.Id))
			case <-a.closeCh:
				shouldExit = true
				a.GetLogger().Info("Actor received shutdown signal, draining queue...",
					zap.Uint64("workerIdx", a.Id))
			default:
			}
		}

		batchSize := a.batcher.GetBatchSize(int64(lastBatchSize))
		n := a.mailBoxQueue.DequeueBatch(msgs[:batchSize])

		if shouldExit && n == 0 {
			a.GetLogger().Info("Actor graceful shutdown: all messages processed",
				zap.Uint64("workerIdx", a.Id))
			return
		}

		if n == 0 {
			idleCount++
			zbackoff.Backoff(idleCount, 10, 30, time.Millisecond)

			// ✅ 空闲时重置 lastBatchSize，避免使用过期的预测数据
			lastBatchSize = 1

			// ✅ 空闲时归零监控计数，避免监控假阳性（显示队列有积压但实际为空）
			atomic.StoreInt64(&a.mailCount, 0)

			continue
		}

		// 有消息处理，重置空闲计数
		idleCount = 0

		// ✅ 更新上次批量大小（驱动下次预测）
		lastBatchSize = n

		// 4. 批量处理消息（顺序模式：单线程顺序处理）
		batchStart := ztime.ServerNow()

		// ✅ 性能优化：批量共享时间戳（批量处理时间 < 1ms，误差可忽略）
		batchStartMs := ztime.ServerNowUnixMilli()
		const timestampRefreshInterval = 50 // 每 50 条消息刷新一次时间戳

		for i := 0; i < n; i++ {
			// ✅ 每 50 条消息更新一次时间戳（平衡性能与精度）
			if i > 0 && i%timestampRefreshInterval == 0 {
				batchStartMs = ztime.ServerNowUnixMilli()
			}
			a.SafeHandleMessage(handleCtx, msgs[i], batchStartMs)
			msgs[i].Release()
		}
		elapsed := time.Since(batchStart)

		// ✅ 记录批处理延迟
		a.batcher.RecordLatency(elapsed)

		// ✅ 定期更新监控计数（每处理 1000 条消息同步一次）
		// 单消费者写入，无伪共享，性能影响可忽略
		processedTotal += int64(n)
		if processedTotal-lastSyncCount >= 1000 {
			atomic.StoreInt64(&a.mailCount, int64(lastBatchSize))
			zmetrics.ActorQueueDepth.Set(int64(lastBatchSize))
			lastSyncCount = processedTotal
		}

		if elapsed > zmodel.GetFrameworkTuning().SlowBatchThreshold {
			a.GetLogger().Warn("mpm slow message batch",
				zap.Uint64("workerIdx", a.Id),
				zap.Int("count", n), // 实际处理的数量
				zap.Duration("elapsed", elapsed),
				zap.Int64("timeout", elapsed.Milliseconds()))
		}
	}
}

// SelectActor selects one actor config for specified actor type.
// SelectActor 按 actorType 选择一个目标 Actor 配置。
func (a *Actor) SelectActor(actorType uint32) zmodel.ActorConfig {
	if a.GetDiscoverer() == nil {
		cfg, err := a.FindPoolActorByType(actorType)
		if err != nil {
			a.GetLogger().Warn("SelectActor", zap.Uint32("actorType", actorType), zap.Error(err))
			return zmodel.ActorConfig{}
		}
		return cfg
	}
	// 随机game actor
	key := fmt.Sprintf("/servers/%d", actorType)
	actorConfig := a.GetDiscoverer().FindPoll(key)
	return actorConfig
}
