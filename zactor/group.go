package zactor

import (
	"context"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/aiyang-zh/zhenyi-base/zcoll"
	"github.com/aiyang-zh/zhenyi-base/zerrs"
	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi-base/ztime"
	"github.com/aiyang-zh/zhenyi/ziface"
	"github.com/aiyang-zh/zhenyi/zmetrics"
	"github.com/aiyang-zh/zhenyi/zmodel"
)

var _ ziface.IGroup = (*Group)(nil)

// Group owns actor instances and route/discovery runtime state.
// Group 管理 Actor 实例及路由/发现相关运行态。
type Group struct {
	actors      *zcoll.Map[uint64, *ActorItem]
	otherActors *zcoll.Map[uint64, zmodel.ActorConfig]
	discoverer  ziface.Discoverer
	ch          chan zmodel.ActorConfig
	process     uint
	isSingle    bool
	actorCh     chan ziface.IActor
	wg          sync.WaitGroup // Track all actor goroutines for graceful shutdown / 跟踪所有 Actor goroutine，用于 graceful shutdown

	scriptEngines map[string]ziface.IScriptEngine
	engineMu      sync.RWMutex

	// msgRouteTable maintains readonly snapshots with Copy-On-Write + atomic.Value.
	// msgRouteTable 使用 Copy-On-Write + atomic.Value 维护只读快照：
	//  - 读路径（LookupActorsByMsgID）为无锁原子读 + map 查询；
	//  - 写路径（RegisterRoutes）在低频注册/扩容时复制整个 map 并替换快照。
	// Type contract: always stores non-nil map[int32][]ziface.IActor.
	// 类型约定：始终存放 map[int32][]ziface.IActor，初始值为非 nil 空 map。
	msgRouteTable atomic.Value

	// otherRouteTableCache accelerates cross-process lookup: msgId -> []ActorConfig.
	// otherRouteTableCache 用于跨进程路由的快速查表：msgId -> []ActorConfig（来自 discoverer 的 otherActors）。
	// It rebuilds on-demand by version to avoid scanning SupportedMsgIDs every route.
	// 通过版本号实现按需重建，避免每次路由都扫描 SupportedMsgIDs。
	otherActorsVersion atomic.Int64
	otherRouteMu       sync.Mutex
	otherRouteSnapshot atomic.Value // *otherRouteTableSnapshot

	watchdog *Watchdog

	closeOnce sync.Once
}

type otherRouteTableSnapshot struct {
	version int64
	table   map[int32][]zmodel.ActorConfig
}

// ActorItem wraps one actor instance and its lightweight counters.
// ActorItem 封装单个 Actor 实例及其轻量统计计数。
type ActorItem struct {
	ziface.IActor
	count atomic.Int64
}

// NewGroup creates an actor group with process-scoped runtime state.
// NewGroup 创建一个按进程维度管理运行态的 Actor Group。
func NewGroup(process uint, isSingle bool) *Group {
	g := &Group{
		actors:        zcoll.NewMap[uint64, *ActorItem](8),
		otherActors:   zcoll.NewMap[uint64, zmodel.ActorConfig](8),
		ch:            make(chan zmodel.ActorConfig, 1),
		process:       process,
		isSingle:      isSingle,
		actorCh:       make(chan ziface.IActor, 256),
		scriptEngines: make(map[string]ziface.IScriptEngine),
	}
	// Initialize route snapshot with empty map to avoid nil checks in read path.
	// 初始化路由表快照为一个空 map，避免读路径需要做 nil 判断。
	g.msgRouteTable.Store(make(map[int32][]ziface.IActor))
	// Initialize cross-process route snapshot as empty table.
	// 初始化跨进程路由快照为空表。
	g.otherRouteSnapshot.Store(&otherRouteTableSnapshot{
		version: 0,
		table:   make(map[int32][]zmodel.ActorConfig),
	})
	return g
}

// FindPoolActorByType selects one local actor config by actor type using load-aware strategy.
// FindPoolActorByType 按 actorType 从本地 Actor 中选择一个配置，采用负载感知策略。
func (g *Group) FindPoolActorByType(actorType uint32) (zmodel.ActorConfig, error) {
	res := make([]*ActorItem, 0)
	g.actors.Range(func(k uint64, v *ActorItem) bool {
		if v.GetActorType() == actorType {
			res = append(res, v)
		}
		return true
	})
	if len(res) == 0 {
		return zmodel.ActorConfig{}, zerrs.Newf(zerrs.ErrTypeNotFound, "no actor found for type %d", actorType)
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].count.Load() < res[j].count.Load()
	})
	res[0].count.Add(1)
	return res[0].GetActorConfig(), nil
}

// AddActor adds one actor instance into current group.
// AddActor 向当前 Group 添加一个 Actor 实例。
func (g *Group) AddActor(iActor ziface.IActor) {
	iActor.SetGroup(g)
	g.actors.Store(iActor.GetActorId(), &ActorItem{
		IActor: iActor,
	})
}

// RegisterRoutes registers supported msgIds of an actor for local routing.
// RegisterRoutes 为给定 Actor 注册其支持的 msgId 集合，用于进程内路由。
// Multiple actors can share the same msgId; upper-layer strategy decides final target.
// 同一个 msgId 允许注册到多个 Actor 上，由上层路由策略决定具体选哪个实例。
func (g *Group) RegisterRoutes(actor ziface.IActor, msgIDs []int32) {
	if actor == nil || len(msgIDs) == 0 {
		return
	}
	// Copy-On-Write.
	// Copy-On-Write：
	// - 读路径保持无锁（atomic.Value 快照）。
	// - 写路径避免“复制整张表 + 复制所有 slice”的放大：只复制 map，
	//   并且仅对本次触达的 msgId 的 slice 做 copy+append。
	oldVal := g.msgRouteTable.Load()
	oldTable, _ := oldVal.(map[int32][]ziface.IActor)
	if oldTable == nil {
		oldTable = make(map[int32][]ziface.IActor)
	}

	// Copy only map; slices are readonly/shared unless key is updated.
	// 仅复制 map（slice 视为只读，可在多个快照间共享；被更新的 key 会生成新 slice）。
	newTable := make(map[int32][]ziface.IActor, len(oldTable)+1)
	for k, v := range oldTable {
		newTable[k] = v
	}

	for _, msgID := range msgIDs {
		list := newTable[msgID]
		duplicate := false
		for _, existing := range list {
			if existing == actor {
				duplicate = true
				break
			}
		}
		if duplicate {
			zlog.Warn("RegisterRoutes: duplicate actor for msgId, skipping",
				zap.Int32("msgId", msgID),
				zap.Uint64("actorId", actor.GetActorId()),
				zap.String("actorTopic", actor.GetTopic()))
			continue
		}

		// Build a new slice for this msgID to avoid mutating old snapshot array.
		// 为该 msgID 创建新 slice，避免修改旧快照的底层数组。
		newSlice := make([]ziface.IActor, 0, len(list)+1)
		newSlice = append(newSlice, list...)
		newSlice = append(newSlice, actor)
		newTable[msgID] = newSlice
	}

	g.msgRouteTable.Store(newTable)
}

// LookupActorsByMsgID returns all local actors that can handle msgId.
// LookupActorsByMsgID 根据 msgId 返回当前进程内所有可处理该消息的 Actor 列表。
// It returns a copy, so callers may safely mutate it.
// 返回的切片为副本，调用方可安全修改。
func (g *Group) LookupActorsByMsgID(msgID int32) []ziface.IActor {
	val := g.msgRouteTable.Load()
	table, _ := val.(map[int32][]ziface.IActor)
	if table == nil {
		return nil
	}
	list, ok := table[msgID]
	if !ok || len(list) == 0 {
		return nil
	}
	out := make([]ziface.IActor, len(list))
	copy(out, list)
	return out
}

// LookupActorsByMsgIDView returns readonly local-route view (allocation-free fast path).
// LookupActorsByMsgIDView 返回进程内路由表的只读视图（零分配快路径）。
// ⚠️ 调用方不得修改返回的 slice（包括重切片/写元素），否则会破坏快照共享语义。
// Use LookupActorsByMsgID when a mutable copy is required.
// 若需要可修改的副本，请使用 LookupActorsByMsgID。
func (g *Group) LookupActorsByMsgIDView(msgID int32) []ziface.IActor {
	val := g.msgRouteTable.Load()
	table, _ := val.(map[int32][]ziface.IActor)
	if table == nil {
		return nil
	}
	list, ok := table[msgID]
	if !ok || len(list) == 0 {
		return nil
	}
	return list
}

// GetActorById returns a local actor by ID; nil when not found.
// GetActorById 按 ID 返回本地 Actor，未找到时返回 nil。
func (g *Group) GetActorById(actorId uint64) ziface.IActor {
	a, ok := g.actors.Load(actorId)
	if ok {
		return a.IActor
	}
	return nil
}

// GetActorCh returns actor event channel used by the group.
// GetActorCh 返回 Group 使用的 Actor 事件通道。
func (g *Group) GetActorCh() chan ziface.IActor {
	return g.actorCh
}

// GetOtherActorById returns cached remote actor config by ID.
// GetOtherActorById 按 ID 返回缓存的远端 Actor 配置。
func (g *Group) GetOtherActorById(actorId uint64) (zmodel.ActorConfig, bool) {
	return g.otherActors.Load(actorId)
}

// EnableWatchdog activates the runtime handler-block detector.
// When a handler runs longer than threshold, the watchdog captures
// the goroutine stack trace and logs it. Call before Run().
// EnableWatchdog 启用运行时处理阻塞检测。
// 当 Handler 执行超过阈值时，watchdog 会抓取 goroutine 栈并记录日志；请在 Run 前调用。
//
//	g.EnableWatchdog(100 * time.Millisecond) // detect handlers > 100ms
func (g *Group) EnableWatchdog(threshold time.Duration) {
	if threshold <= 0 {
		return
	}
	g.watchdog = newWatchdog(g, threshold)
}

// Run initializes and starts all actors in the group.
// Run 初始化并启动 Group 中所有 Actor。
func (g *Group) Run(ctx context.Context) error {
	if g.watchdog != nil {
		go func() {
			defer zlog.Recover("group run watchdog err")
			g.watchdog.run(ctx)
		}()
	}

	actorItems := make([]*ActorItem, 0)
	g.actors.Range(func(actorId uint64, item *ActorItem) bool {
		actorItems = append(actorItems, item)
		return true
	})
	sort.Slice(actorItems, func(i, j int) bool {
		return actorItems[i].IActor.GetActorId() < actorItems[j].IActor.GetActorId()
	})

	var succeeded []*ActorItem
	for _, item := range actorItems {
		actorId := item.IActor.GetActorId()
		g.registerActor(item)
		if err := item.IActor.Init(ctx); err != nil {
			zlog.Error("Actor init failed, rolling back",
				zap.Uint64("actorId", actorId),
				zap.String("actorName", item.GetNameTopic()),
				zap.Error(zerrs.Wrap(err, zerrs.ErrTypeInternal, "actor init failed")))
			g.rollbackRunInit(ctx, succeeded, item, true)
			return zerrs.Wrap(err, zerrs.ErrTypeInternal, "actor init failed")
		}
		if serverActor, ok := item.IActor.(ziface.IServerActor); ok {
			if err := serverActor.RunServer(ctx); err != nil {
				zlog.Error("Actor server start failed, rolling back",
					zap.Uint64("actorId", actorId),
					zap.String("actorName", item.GetNameTopic()),
					zap.Error(zerrs.Wrap(err, zerrs.ErrTypeInternal, "actor server initialization failed")))
				g.rollbackRunInit(ctx, succeeded, item, false)
				return zerrs.Wrap(err, zerrs.ErrTypeInternal, "actor server initialization failed")
			}
		}
		item.IActor.GetLogger().Info("actor run success", zap.String("actor name", item.GetNameTopic()))
		succeeded = append(succeeded, item)
	}

	// Start discoverer watch only after all local actors are successfully initialized.
	// 仅在本地 Actor 全部启动成功后再开启 watch，避免失败回滚时残留后台 goroutine。
	go g.watchActor(ctx)

	g.StartWorkers(ctx)
	g.tick(ctx)
	return nil
}

func (g *Group) rollbackRunInit(ctx context.Context, succeeded []*ActorItem, failed *ActorItem, initFailed bool) {
	closeCtx := ctx
	for i := len(succeeded) - 1; i >= 0; i-- {
		item := succeeded[i]
		if item != nil && item.IActor != nil {
			_ = item.IActor.Close(closeCtx)
		}
		g.unregisterActor(item)
	}
	if failed != nil && failed.IActor != nil {
		if !initFailed {
			_ = failed.IActor.Close(closeCtx)
		}
		g.unregisterActor(failed)
	}
}

func (g *Group) unregisterActor(item *ActorItem) {
	if g.discoverer == nil || item == nil {
		return
	}
	cfg := item.GetActorConfig()
	if err := g.discoverer.Unregister(cfg); err != nil {
		zlog.Error("Failed to unregister actor from discoverer",
			zap.Uint64("actorId", cfg.Id),
			zap.Uint32("actorType", cfg.ActorType),
			zap.Error(err))
	}
}

// Close gracefully closes script engines and all local actors (best-effort).
// Close 优雅关闭脚本引擎与本组全部 Actor（尽力而为）。
func (g *Group) Close(ctx context.Context) error {
	g.closeOnce.Do(func() {
		// Close actors first to avoid breaking in-flight script calls.
		// Actor 先关：避免 handler 仍在执行脚本时先关闭引擎导致异常。
		g.actors.Range(func(_ uint64, item *ActorItem) bool {
			if item != nil && item.IActor != nil {
				_ = item.IActor.Close(ctx)
			}
			return true
		})
		// Then close engines to release VM resources.
		g.CloseScriptEngines()
	})
	return nil
}

// StartWorkers starts actor worker goroutines with supervision.
// StartWorkers 启动并托管 Actor 工作协程。
func (g *Group) StartWorkers(ctx context.Context) {
	g.actors.Range(func(actorId uint64, item *ActorItem) bool {
		g.wg.Add(1)
		actor := item.IActor
		go g.superviseActor(ctx, actor)
		return true
	})
}

const defaultMaxRestarts = 3

func (g *Group) superviseActor(ctx context.Context, actor ziface.IActor) {
	defer zlog.Recover("superviseActor", zap.String("actor", actor.GetNameTopic()))
	defer g.wg.Done()

	maxRestarts := defaultMaxRestarts
	if cfg := actor.GetActorConfig(); cfg.MaxRestarts > 0 {
		maxRestarts = int(cfg.MaxRestarts)
	}

	restarts := 0
	// 重启窗口：超过窗口期后重置计数，避免“历史累计”导致永久不可重启
	const restartWindow = 30 * time.Second
	lastRestart := time.Now()
	for {
		actor.Run(ctx)

		select {
		case <-ctx.Done():
			return // graceful shutdown
		default:
		}

		// 超过窗口期则重置计数（Actor 已稳定运行了一段时间）
		if time.Since(lastRestart) > restartWindow {
			restarts = 0
		}

		restarts++
		zmetrics.ActorRestarts.Inc()
		if restarts > maxRestarts {
			zlog.Error("Actor exceeded max restarts, permanently stopped",
				zap.String("actor", actor.GetTopic()),
				zap.Int("restarts", restarts),
				zap.Int("maxRestarts", maxRestarts))
			return
		}

		zlog.Warn("Actor exited unexpectedly, restarting",
			zap.String("actor", actor.GetTopic()),
			zap.Int("restart", restarts),
			zap.Int("maxRestarts", maxRestarts))

		lastRestart = time.Now()
		// 指数退避（上限 30s），避免抖动时疯狂重启刷日志
		backoff := time.Second << uint(min(restarts-1, 5)) // 1,2,4,8,16,32...
		if backoff > 30*time.Second {
			backoff = 30 * time.Second
		}
		time.Sleep(backoff)
	}
}

func (g *Group) tick(ctx context.Context) {
	go func() {
		defer zlog.Recover("Group tick", zap.Int("workerIdx", 0))
		timer := time.NewTicker(1 * time.Second / 30)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				nowTs := ztime.ServerNowUnixMilli()
				g.actors.Range(func(actorId uint64, item *ActorItem) bool {
					// 仅对需要 TickFn 的 Actor 推送 Tick，避免无谓的全量扫描与消息投递。
					if ta, ok := item.IActor.(interface{ HasTickFns() bool }); ok && !ta.HasTickFns() {
						return true
					}
					// 防堆积：如果上一个 Tick 还未消费，跳过本次
					if item.IActor.MarkTickPending() {
						item.IActor.Push(zmodel.ActorCmd{
							Type:    zmodel.CmdTypeTick,
							TickNow: nowTs,
						})
					}
					return true
				})
			}
		}
	}()
}

// GetDiscoverer returns current service discoverer.
// GetDiscoverer 返回当前服务发现器。
func (g *Group) GetDiscoverer() ziface.Discoverer {
	return g.discoverer
}

// SetDiscoverer sets service discoverer used by this group.
// SetDiscoverer 设置当前 Group 使用的服务发现器。
func (g *Group) SetDiscoverer(discover ziface.Discoverer) {
	g.discoverer = discover
}

func (g *Group) watchActor(ctx context.Context) {
	defer zlog.Recover("Group watchActor error recover")
	if g.isSingle { // 单机版不支持动态增加
		return
	}
	if g.discoverer == nil {
		return
	}
	if ctx == nil {
		return
	}
	// 监听变化之前加载存量数据
	items := g.discoverer.FindAllByPrefix("/servers")
	for _, item := range items {
		if uint64(item.ActorConfig.Process) == uint64(g.process) { // 当前进程不处理
			continue
		}
		g.otherActors.Store(item.ActorConfig.Id, item.ActorConfig)
	}
	// 批量加载结束后，递增一次版本，触发下次按需重建跨进程路由表。
	g.otherActorsVersion.Add(1)

	// 用 ctx 可中断退出，避免 Close/Cancel 后该 goroutine 仍可能卡在 channel recv 上。
	ch := g.discoverer.Watch()
	for {
		select {
		case <-ctx.Done():
			return
		case cfg, ok := <-ch:
			if !ok {
				return
			}
			if cfg.Id <= 0 {
				continue
			}
			if cfg.ActorType <= 0 { // 删除
				g.otherActors.Delete(cfg.Id)
			} else { // 更新或添加
				g.otherActors.Store(cfg.Id, cfg)
			}
			// 标记 otherActors 视图已变化（不在这里重建，避免频繁 O(N) 扫描；由查询方按需重建）。
			g.otherActorsVersion.Add(1)
		}
	}
}

// registerActor 将本进程内的 Actor 注册到发现服务，同时附带其支持的 msgId 集合。
// 该信息用于跨进程路由构建“msgId -> Actor 实例”全局视图。
func (g *Group) registerActor(item *ActorItem) {
	if item == nil {
		return
	}
	cfg := item.GetActorConfig()
	msgList := item.GetMsgList()
	if len(msgList) > 0 {
		supported := make([]int32, 0, len(msgList))
		for id := range msgList {
			supported = append(supported, id)
		}
		sort.Slice(supported, func(i, j int) bool { return supported[i] < supported[j] })
		cfg.SupportedMsgIDs = supported
	}
	g.register(cfg)
}

func (g *Group) register(a zmodel.ActorConfig) {
	if g.discoverer == nil {
		return
	}
	err := g.discoverer.Register(a)
	if err != nil {
		zlog.Error("Failed to register actor to discoverer",
			zap.Uint64("actorId", a.Id),
			zap.Uint32("actorType", a.ActorType),
			zap.Error(zerrs.Wrap(err, zerrs.ErrTypeInternal, "discoverer registration failed")))
	}
}

// IsSingle reports whether group runs in single-node mode.
// IsSingle 返回当前 Group 是否为单机模式。
func (g *Group) IsSingle() bool {
	return g.isSingle
}

// GetActors returns a snapshot slice of local actors.
// GetActors 返回本地 Actor 快照切片。
func (g *Group) GetActors() []ziface.IActor {
	actors := make([]ziface.IActor, 0)
	g.actors.Range(func(actorId uint64, item *ActorItem) bool {
		actors = append(actors, item.IActor)
		return true
	})
	return actors
}

// GetOtherActorConfigs returns snapshot of remote actor configs seen by current process.
// GetOtherActorConfigs 返回当前进程视角下的远端 Actor 配置快照。
// Each config carries SupportedMsgIDs for cross-process route-table construction.
// 每个配置包含 SupportedMsgIDs，可用于构建跨进程路由表。
func (g *Group) GetOtherActorConfigs() []zmodel.ActorConfig {
	out := make([]zmodel.ActorConfig, 0)
	g.otherActors.Range(func(id uint64, cfg zmodel.ActorConfig) bool {
		out = append(out, cfg)
		return true
	})
	return out
}

// LookupOtherActorConfigsByMsgID returns remote actor candidates that can handle msgID.
// LookupOtherActorConfigsByMsgID 返回可处理指定 msgID 的远端 Actor 候选列表。
// This is an optional extension (not required by ziface.IGroup); callers receive a mutable copy.
// 该方法是可选扩展（不属于 ziface.IGroup 强制接口）；调用方拿到的是可修改副本。
func (g *Group) LookupOtherActorConfigsByMsgID(msgID int32) []zmodel.ActorConfig {
	list := g.LookupOtherActorConfigsByMsgIDView(msgID)
	if len(list) > 0 {
		out := make([]zmodel.ActorConfig, len(list))
		copy(out, list)
		return out
	}
	return nil
}

// LookupOtherActorConfigsByMsgIDView returns readonly view of remote candidates (allocation-free fast path).
// LookupOtherActorConfigsByMsgIDView 返回远端候选列表的只读视图（零分配快路径）。
// Callers must not mutate returned slice; use LookupOtherActorConfigsByMsgID for mutable copy.
// 调用方不得修改返回切片；如需可修改副本请使用 LookupOtherActorConfigsByMsgID。
func (g *Group) LookupOtherActorConfigsByMsgIDView(msgID int32) []zmodel.ActorConfig {
	if msgID == 0 {
		return nil
	}
	currentVer := g.otherActorsVersion.Load()
	snapAny := g.otherRouteSnapshot.Load()
	snap, _ := snapAny.(*otherRouteTableSnapshot)
	if snap != nil && snap.version == currentVer {
		if list := snap.table[msgID]; len(list) > 0 {
			return list
		}
		return nil
	}

	// 版本变化：按需重建快照（串行化，避免并发下重复构建）。
	g.otherRouteMu.Lock()
	defer g.otherRouteMu.Unlock()

	currentVer = g.otherActorsVersion.Load()
	snapAny = g.otherRouteSnapshot.Load()
	snap, _ = snapAny.(*otherRouteTableSnapshot)
	if snap != nil && snap.version == currentVer {
		if list := snap.table[msgID]; len(list) > 0 {
			return list
		}
		return nil
	}

	table := make(map[int32][]zmodel.ActorConfig)
	g.otherActors.Range(func(_ uint64, cfg zmodel.ActorConfig) bool {
		if len(cfg.SupportedMsgIDs) == 0 {
			return true
		}
		for _, id := range cfg.SupportedMsgIDs {
			table[id] = append(table[id], cfg)
		}
		return true
	})
	g.otherRouteSnapshot.Store(&otherRouteTableSnapshot{
		version: currentVer,
		table:   table,
	})
	if list := table[msgID]; len(list) > 0 {
		return list
	}
	return nil
}

// ==========================================
// 脚本引擎管理方法
// ==========================================

// GetScriptEngine returns script engine by type in a thread-safe way.
// GetScriptEngine 线程安全地返回指定类型的脚本引擎。
// Use constants like ziface.ScriptEngineLua.
// 建议使用 ziface.ScriptEngineLua 等常量。
func (g *Group) GetScriptEngine(engineType ziface.ScriptEngineType) ziface.IScriptEngine {
	g.engineMu.RLock()
	defer g.engineMu.RUnlock()
	return g.scriptEngines[string(engineType)]
}

// SetScriptEngine registers script engine by type.
// SetScriptEngine 按类型注册脚本引擎（推荐方式）。
// Use constants like ziface.ScriptEngineLua.
// 建议使用 ziface.ScriptEngineLua 等常量。
func (g *Group) SetScriptEngine(engineType ziface.ScriptEngineType, engine ziface.IScriptEngine) {
	g.engineMu.Lock()
	defer g.engineMu.Unlock()

	if engine == nil {
		zlog.Warn("Attempting to set nil script engine", zap.String("engineType", string(engineType)))
		return
	}

	g.scriptEngines[string(engineType)] = engine
	zlog.Info("Script engine registered",
		zap.String("engineType", string(engineType)),
		zap.String("actualType", engine.GetType()))
}

// CloseScriptEngines closes and clears all registered script engines.
// CloseScriptEngines 关闭并清空所有已注册脚本引擎。
func (g *Group) CloseScriptEngines() {
	g.engineMu.Lock()
	defer g.engineMu.Unlock()

	for engineType, engine := range g.scriptEngines {
		if engine != nil {
			engine.Close()
			zlog.Info("Script engine closed", zap.String("engineType", engineType))
		}
	}
	g.scriptEngines = make(map[string]ziface.IScriptEngine)
}
