package zactor

import (
	"context"
	baseziface "github.com/aiyang-zh/zhenyi-base/ziface"
	"github.com/aiyang-zh/zhenyi-base/ztimer"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/aiyang-zh/zhenyi-base/zerrs"
	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi/zmsg"
	"github.com/aiyang-zh/zhenyi/zpoolobs"
)

const (
	// DefaultMaxPendingRPCs is the default size of RPC pending-slot pool.
	// DefaultMaxPendingRPCs 是 RPC 待回包槽位池的默认大小。
	DefaultMaxPendingRPCs = 4096
	// VersionShift splits rpcId into version bits and slot index bits.
	// VersionShift 用于将 rpcId 划分为版本位与槽位索引位。
	VersionShift = 16
	// CacheLineSize is reserved for cache-line alignment related tuning.
	// CacheLineSize 预留用于缓存行对齐相关调优。
	CacheLineSize       = 128
	defaultWatchdogTick = 100 * time.Millisecond

	SlotFree      = 0
	SlotWaiting   = 1
	SlotAbandoned = 2
)

var (
	// ErrPoolExhausted indicates all RPC slots are occupied.
	// ErrPoolExhausted 表示 RPC 槽位池已耗尽（全部被占用）。
	ErrPoolExhausted = zerrs.New(zerrs.ErrTypeInternal, "rpc pool exhausted: all slots are busy")
)

type rpcSlot struct {
	ch        chan *zmsg.Message
	version   uint64
	state     int32
	timestamp int64 // milliseconds
	_         [100]byte
}

// ActorMsgSender manages RPC slot allocation and reply matching.
// ActorMsgSender 管理 RPC 槽位分配与回包匹配。
type ActorMsgSender struct {
	slots                  []rpcSlot
	cursor                 uint64
	ctx                    context.Context
	timeout                time.Duration
	indexMask              uint64
	poolSize               uint64
	timerPool              *ztimer.TimerPool
	abandonedRecycleDelay  time.Duration
	waitingForceRecycleTTL time.Duration
	watchdogTick           time.Duration
}

func nextPowerOfTwo(n int) int {
	if n <= 0 {
		return 2
	}
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	return n + 1
}

// NewSender creates an RPC sender with default observer behavior.
// NewSender 创建 RPC 发送器（使用默认观测器行为）。
func NewSender(ctx context.Context, maxPending int) *ActorMsgSender {
	return NewSenderWithObserver(ctx, maxPending, nil)
}

// NewSenderWithObserver creates an RPC sender with optional pool observer injection.
// NewSenderWithObserver 创建 RPC 发送器，并支持注入可选对象池观测器。
func NewSenderWithObserver(ctx context.Context, maxPending int, obs baseziface.IPoolObserver) *ActorMsgSender {
	if maxPending <= 0 {
		maxPending = DefaultMaxPendingRPCs
	}
	maxPending = nextPowerOfTwo(maxPending)
	if maxPending > (1 << VersionShift) {
		panic("maxPendingRPCs exceeds VersionShift capacity (65536)")
	}

	s := &ActorMsgSender{
		ctx:                    ctx,
		timeout:                30 * time.Second,
		slots:                  make([]rpcSlot, maxPending),
		indexMask:              uint64(maxPending - 1),
		poolSize:               uint64(maxPending),
		timerPool:              ztimer.NewTimerPool("azctor.sender", zpoolobs.Resolve(obs)),
		abandonedRecycleDelay:  100 * time.Millisecond,
		waitingForceRecycleTTL: 2 * time.Minute,
		watchdogTick:           defaultWatchdogTick,
	}
	for i := 0; i < maxPending; i++ {
		s.slots[i].version = 1
		// Eagerly initialize slot.ch to avoid runtime writes and race warnings.
		// 提前初始化 slot.ch：watchdog/recycleSlot 会读取 slot.ch，AddSender 以前会懒初始化写入 slot.ch，
		// 在 -race 下会触发数据竞争。这里改为 eager init 后，slot.ch 在运行期不再被写入。
		s.slots[i].ch = make(chan *zmsg.Message, 1)
	}
	go s.watchdog()
	return s
}

func (m *ActorMsgSender) watchdog() {
	ticker := time.NewTicker(m.watchdogTick)
	defer ticker.Stop()

	const shards = 16
	shardIdx := 0
	batchSize := int(m.poolSize) / shards
	if batchSize < 1 {
		batchSize = 1
	}

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			now := time.Now().UnixMilli()
			start := shardIdx * batchSize
			end := start + batchSize
			if end > int(m.poolSize) {
				end = int(m.poolSize)
			}

			for i := start; i < end; i++ {
				m.checkSlot(&m.slots[i], now)
			}

			shardIdx = (shardIdx + 1) % shards
		}
	}
}

func (m *ActorMsgSender) checkSlot(slot *rpcSlot, now int64) {
	st := atomic.LoadInt32(&slot.state)
	if st == SlotFree {
		return
	}

	ts := atomic.LoadInt64(&slot.timestamp)

	// Strategy 1: recycle Abandoned slots.
	// 策略 1: 清理 Abandoned (已废弃) 的槽位。
	// Abandoned means GetReply already returned, so recycle promptly.
	// 只要是 Abandoned，立刻回收，因为 GetReply 肯定已经走了。
	if st == SlotAbandoned {
		// Use short configurable delay to reduce long-lived abandoned occupancy.
		// 采用较短可配置延迟，减少 Abandoned 长时间占坑。
		if now-ts > m.abandonedRecycleDelay.Milliseconds() {
			m.recycleSlot(slot)
		}
		return
	}

	// Strategy 2: recycle long-waiting slots (timeout without reply).
	// 策略 2: 清理 Waiting (超时未回包) 的槽位。
	// Threshold is 2 minutes to avoid permanent occupancy.
	// 阈值设为 2 分钟，防止永久占坑。
	if st == SlotWaiting && now-ts > m.waitingForceRecycleTTL.Milliseconds() {
		// Force Waiting -> Free.
		// 强行把 Waiting 变成 Free。
		zlog.Warn("Watchdog force recycled waiting slot",
			zap.Uint64("slotVersion", atomic.LoadUint64(&slot.version)),
			zap.Int64("waitMs", now-ts))
		m.recycleSlot(slot)
	}
}

// recycleSlot 安全回收
func (m *ActorMsgSender) recycleSlot(slot *rpcSlot) {
	// 1. version++ (使旧消息失效)
	atomic.AddUint64(&slot.version, 1)

	// 2. 清空 channel (drain)
	if slot.ch != nil {
		select {
		case msg := <-slot.ch:
			if msg != nil {
				msg.Release()
			}
		default:
		}
	}

	// 3. state -> Free
	atomic.StoreInt32(&slot.state, SlotFree)
}

// AddSender allocates one RPC slot and returns generated rpcId.
// AddSender 分配一个 RPC 槽位并返回生成的 rpcId。
func (m *ActorMsgSender) AddSender() (uint64, error) {
	start := atomic.AddUint64(&m.cursor, 1)
	for i := uint64(0); i < m.poolSize; i++ {
		idx := (start + i) & m.indexMask
		slot := &m.slots[idx]

		// Only Free slot can be claimed.
		// 只能抢占 Free 的槽位。
		if atomic.CompareAndSwapInt32(&slot.state, SlotFree, SlotWaiting) {
			// slot.ch 在 NewSenderWithObserver 中已提前初始化，因此这里无需懒初始化。

			newVer := atomic.AddUint64(&slot.version, 1)
			atomic.StoreInt64(&slot.timestamp, time.Now().UnixMilli())

			rpcId := (newVer << VersionShift) | idx
			if rpcId == 0 {
				atomic.StoreInt32(&slot.state, SlotFree) // Rollback / 回滚
				continue
			}
			return rpcId, nil
		}
	}
	return 0, ErrPoolExhausted
}

// SetReply routes one response envelope into its RPC slot.
// SetReply 将响应信封路由到对应的 RPC 槽位。
func (m *ActorMsgSender) SetReply(data *zmsg.Message) {
	idx := data.RpcId & m.indexMask
	reqVer := data.RpcId >> VersionShift
	slot := &m.slots[idx]

	// 1. 检查 Version (第一道防线)
	if atomic.LoadUint64(&slot.version) != reqVer {
		return
	}

	// 2. 检查 State (第二道防线 - 优化 Issue 3)
	// Reply can be sent only when slot is Waiting.
	// 必须是 Waiting 状态才发送。
	// 如果是 Abandoned (GetReply已超时)，则直接协助回收 (可选优化)
	// 如果是 Free (已被回收)，则不发
	state := atomic.LoadInt32(&slot.state)
	if state != SlotWaiting {
		// 既然 SetReply 来了，发现是 Abandoned，说明 GetReply 刚走。
		// 我们可以顺手帮忙回收，减轻 Watchdog 压力 (可选)
		/*
			if state == SlotAbandoned && atomic.CompareAndSwapInt32(&slot.state, SlotAbandoned, SlotFree) {
				// 帮 cleanup 一下，虽然不必须
				atomic.AddUint64(&slot.version, 1)
			}
		*/
		return
	}

	// Defensive guard: AddSender normally initialized ch, but keep one safety check.
	// 懒初始化保护：正常情况下 AddSender 已经创建过 ch，但这里做一层防御。
	if slot.ch == nil {
		return
	}

	ref := data.Retain()
	select {
	case slot.ch <- ref:
	default:
		ref.Release()
	}
}

// GetReply waits for reply of expected rpcId within timeout.
// GetReply 在超时限制内等待指定 rpcId 的回包。
func (m *ActorMsgSender) GetReply(expectedRpcId uint64, timeout time.Duration) (*zmsg.Message, bool) {
	idx := expectedRpcId & m.indexMask
	slot := &m.slots[idx]

	// Helper for successful receive.
	// 成功收到消息的 Helper。
	success := func(msg *zmsg.Message) (*zmsg.Message, bool) {
		// Current owner releases slot immediately after matching reply arrives.
		// 当前持有者收到匹配回包后直接释放槽位；
		// Watchdog 只会处理超时/废弃槽，不影响该成功路径。
		atomic.StoreInt32(&slot.state, SlotFree)
		return msg, true
	}

	select {
	case <-m.ctx.Done():
		// Stop waiting when context is done.
		// 上下文结束，放弃等待。
		// 尝试 Waiting -> Abandoned，交给 Watchdog 收尸
		if atomic.CompareAndSwapInt32(&slot.state, SlotWaiting, SlotAbandoned) {
			atomic.StoreInt64(&slot.timestamp, time.Now().UnixMilli())
		}
		return nil, false
	default:
	}

	// AddSender should have initialized ch; keep defensive check for nil-channel dead wait.
	// 理论上 AddSender 已完成 ch 初始化；这里防御性检查，避免 nil channel 永久阻塞。
	if slot.ch == nil {
		atomic.CompareAndSwapInt32(&slot.state, SlotWaiting, SlotAbandoned)
		return nil, false
	}

	if timeout <= 0 {
		if atomic.CompareAndSwapInt32(&slot.state, SlotWaiting, SlotAbandoned) {
			atomic.StoreInt64(&slot.timestamp, time.Now().UnixMilli())
			return nil, false
		}
		return nil, false
	}

	// Fast path: return immediately if reply already arrived to avoid timer allocation/churn.
	// 快路径：若回包已到达，直接返回，避免创建/归还 timer 的开销与分配。
	select {
	case resp := <-slot.ch:
		if resp != nil && resp.RpcId == expectedRpcId {
			return success(resp)
		}
		if resp != nil {
			resp.Release()
			zlog.Warn("Received stale RPC message (version mismatch)",
				zap.Uint64("expectedRpcId", expectedRpcId),
				zap.Uint64("receivedRpcId", resp.RpcId),
				zap.Uint64("slotIndex", idx))
		}
	default:
	}

	timer := m.timerPool.Get(timeout)
	defer m.timerPool.Put(timer)

	for {
		select {
		case <-m.ctx.Done():
			if atomic.CompareAndSwapInt32(&slot.state, SlotWaiting, SlotAbandoned) {
				atomic.StoreInt64(&slot.timestamp, time.Now().UnixMilli())
			}
			return nil, false

		case <-timer.C:
			// 超时！
			// ★关键状态流转★：尝试 Waiting -> Abandoned
			if atomic.CompareAndSwapInt32(&slot.state, SlotWaiting, SlotAbandoned) {
				atomic.StoreInt64(&slot.timestamp, time.Now().UnixMilli())
				// 标记成功，Slot 变为废弃状态。
				// 我们不立即释放(Free)，防止串包。
				// Watchdog 会在几秒后将其回收。
				return nil, false
			}
			// 如果 CAS 失败，说明状态变了？
			// 唯一的可能是 Watchdog 把它强行 Free 了 (极小概率，超时时间设得很长)
			// 或者... 并没有其他并发修改者。
			return nil, false

		case resp := <-slot.ch:
			// 收到消息
			if resp.RpcId != expectedRpcId {
				resp.Release()
				zlog.Warn("Received stale RPC message (version mismatch)",
					zap.Uint64("expectedRpcId", expectedRpcId),
					zap.Uint64("receivedRpcId", resp.RpcId),
					zap.Uint64("slotIndex", idx))
				continue
			}
			return success(resp)
		}
	}
}
