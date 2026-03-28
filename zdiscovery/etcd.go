package zdiscovery

import (
	"context"
	"fmt"
	"github.com/aiyang-zh/zhenyi-base/zcoll"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"

	"github.com/aiyang-zh/zhenyi-base/zerrs"
	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi-base/zrand"
	"github.com/aiyang-zh/zhenyi-base/zserialize"
	"github.com/aiyang-zh/zhenyi/ziface"
	"github.com/aiyang-zh/zhenyi/zmodel"
)

var _ ziface.Discoverer = (*EtcdDiscovery)(nil)

// EtcdDiscovery implements Etcd-based service discovery.
// EtcdDiscovery 实现基于 Etcd 的服务发现。
// Key format: /servers/{actorType}/{actorId}
// Key 格式：/servers/{actorType}/{actorId}
// Value is a JSON-serialized ActorServerRegister.
// Value 为 JSON 序列化的 ActorServerRegister。
//
// NewEtcdDiscovery creates an Etcd-backed Discoverer with shared lease and cache warmup.
// NewEtcdDiscovery 创建基于 Etcd 的 Discoverer，并初始化共享租约与本地缓存。
func NewEtcdDiscovery(ctx context.Context, cli *clientv3.Client) (*EtcdDiscovery, error) {
	if cli == nil {
		return nil, zerrs.New(zerrs.ErrTypeValidation, "etcd client is nil")
	}
	ctx, cancel := context.WithCancel(ctx)

	d := &EtcdDiscovery{
		cli:          cli,
		ctx:          ctx,
		cancel:       cancel,
		ch:           make(chan zmodel.ActorConfig, 100),
		pollCounters: zcoll.NewSyncMap[string, *uint64](),
	}

	if err := d.createSharedLease(); err != nil {
		zlog.Warn("EtcdDiscovery: failed to create shared lease, will retry", zap.Error(err))
	}

	if err := d.loadAllToCache(); err != nil {
		zlog.Warn("EtcdDiscovery: failed to load initial cache", zap.Error(err))
	}

	go d.watch()
	return d, nil
}

// EtcdDiscovery is the Etcd-based implementation of ziface.Discoverer.
// EtcdDiscovery 是基于 Etcd 的 ziface.Discoverer 实现。
type EtcdDiscovery struct {
	cli    *clientv3.Client
	ctx    context.Context
	cancel context.CancelFunc
	ch     chan zmodel.ActorConfig

	// cache uses atomic.Pointer + copy-on-write; reads are lock-free and writes replace the whole pointer.
	// cache 使用 atomic.Pointer + copy-on-write，读无锁，写时整体替换
	cache atomic.Pointer[map[uint32][]zmodel.ActorConfig]

	sharedLeaseID clientv3.LeaseID
	// leaseMu cannot be removed: lease grant/recreate must be serialized to avoid multi-instance grant leakage.
	// leaseMu 不能去掉：创建/重建租约必须串行，否则会多实例 Grant 导致租约泄漏
	leaseMu sync.Mutex

	// registeredActors uses atomic.Pointer + copy-on-write; reads are lock-free (Range is used only on rebuild/close).
	// registeredActors 使用 atomic.Pointer + copy-on-write，读无锁（Range 仅重建/关闭时用）
	registeredActors atomic.Pointer[map[string]zmodel.ActorConfig]

	// FindPoll polling counter: get-or-create per key. sync.Map is more appropriate because atomic+CoW copies the whole map on new keys.
	// FindPoll 轮询计数器：按 key get-or-create，用 sync.Map 更合适（原子+CoW 会每次新 key 复制整表）
	pollCounters *zcoll.SyncMap[string, *uint64]
}

func ptrToMap(p *map[uint32][]zmodel.ActorConfig) map[uint32][]zmodel.ActorConfig {
	if p == nil {
		return nil
	}
	return *p
}

func ptrToRegistered(p *map[string]zmodel.ActorConfig) map[string]zmodel.ActorConfig {
	if p == nil {
		return nil
	}
	return *p
}

// cloneCache 深拷贝 cache map，用于 copy-on-write 写路径。
func cloneCache(m map[uint32][]zmodel.ActorConfig) map[uint32][]zmodel.ActorConfig {
	if m == nil {
		return nil
	}
	out := make(map[uint32][]zmodel.ActorConfig, len(m))
	for k, v := range m {
		out[k] = append([]zmodel.ActorConfig{}, v...)
	}
	return out
}

func cloneRegistered(m map[string]zmodel.ActorConfig) map[string]zmodel.ActorConfig {
	if m == nil {
		return nil
	}
	out := make(map[string]zmodel.ActorConfig, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// createSharedLeaseLocked 在已持有 leaseMu 时创建共享租约并启动保活；调用方必须已 Lock。
func (d *EtcdDiscovery) createSharedLeaseLocked() error {
	grantResp, err := d.cli.Grant(d.ctx, 10)
	if err != nil {
		return zerrs.Wrap(err, zerrs.ErrTypeNetwork, "create shared lease")
	}
	d.sharedLeaseID = grantResp.ID
	go d.runLeaseKeepalive()
	return nil
}

func (d *EtcdDiscovery) createSharedLease() error {
	d.leaseMu.Lock()
	defer d.leaseMu.Unlock()
	return d.createSharedLeaseLocked()
}

// runLeaseKeepalive 保活租约；channel 关闭时重新创建租约、重新注册，并启动新 goroutine 继续保活
func (d *EtcdDiscovery) runLeaseKeepalive() {
	defer zlog.Recover("EtcdDiscovery lease keepalive", zap.String("component", "lease"))
	for {
		select {
		case <-d.ctx.Done():
			return
		default:
		}
		d.leaseMu.Lock()
		leaseID := d.sharedLeaseID
		d.leaseMu.Unlock()
		if leaseID == 0 {
			return
		}
		ch, err := d.cli.KeepAlive(d.ctx, leaseID)
		if err != nil {
			zlog.Warn("EtcdDiscovery: keepalive failed, will recreate lease", zap.Error(err))
			time.Sleep(2 * time.Second)
			continue
		}
		// Normal keepalive loop.
		// 正常保活。
		for ka := range ch {
			if ka == nil {
				zlog.Warn("EtcdDiscovery: keepalive channel closed, recreating lease")
				break
			}
		}
		// keepalive channel 关闭时，优先检查是否正在退出。
		// Avoid unnecessary lease-recreate/reregister flow during CloseAll().
		// 避免在 CloseAll() 过程中不必要地进入“重建租约/重注册”逻辑。
		select {
		case <-d.ctx.Done():
			return
		default:
		}
		// channel 关闭：重新创建租约、重新注册，并启动新的保活 goroutine；当前 goroutine 退出
		if err := d.recreateLeaseAndReregister(); err != nil {
			zlog.Warn("EtcdDiscovery: recreate lease failed", zap.Error(err))
			time.Sleep(2 * time.Second)
			continue
		}
		return // New goroutine started; exit current one / 新 goroutine 已启动，当前退出
	}
}

// recreateLeaseAndReregister 先创建新租约、用新租约重新注册当前集合，最后撤销旧租约，避免服务中断与“注销被复活”
func (d *EtcdDiscovery) recreateLeaseAndReregister() error {
	d.leaseMu.Lock()
	oldLease := d.sharedLeaseID
	if err := d.createSharedLeaseLocked(); err != nil {
		d.leaseMu.Unlock()
		return err
	}
	newLease := d.sharedLeaseID
	d.leaseMu.Unlock()

	// Re-put only keys still in registeredActors to avoid reviving already-unregistered actors.
	// 仅对当前仍存在于 registeredActors 的 key 做 Put，避免已注销的 actor 被复活。
	pReg := d.registeredActors.Load()
	if pReg == nil {
		if oldLease != 0 {
			revCtx, revCancel := context.WithTimeout(d.ctx, 5*time.Second)
			_, _ = d.cli.Revoke(revCtx, oldLease)
			revCancel()
		}
		return nil
	}
	reg := *pReg
	for key, c := range reg {
		regItem := &zmodel.ActorServerRegister{Key: key, ActorConfig: c}
		val, err := zserialize.MarshalJson(regItem)
		if err != nil {
			zlog.Warn("EtcdDiscovery: reregister serialize failed", zap.String("key", key), zap.Error(err))
			continue
		}
		_, err = d.cli.Put(d.ctx, key, string(val), clientv3.WithLease(newLease))
		if err != nil {
			zlog.Warn("EtcdDiscovery: reregister put failed", zap.String("key", key), zap.Error(err))
		}
	}

	if oldLease != 0 {
		revCtx, revCancel := context.WithTimeout(d.ctx, 5*time.Second)
		_, _ = d.cli.Revoke(revCtx, oldLease)
		revCancel()
	}
	return nil
}

func (d *EtcdDiscovery) loadAllToCache() error {
	resp, err := d.cli.Get(d.ctx, "/servers/", clientv3.WithPrefix())
	if err != nil {
		return zerrs.Wrap(err, zerrs.ErrTypeNetwork, "load from etcd")
	}
	newCache := make(map[uint32][]zmodel.ActorConfig)
	for _, kv := range resp.Kvs {
		var item zmodel.ActorServerRegister
		if err := zserialize.UnmarshalJson(kv.Value, &item); err != nil {
			zlog.Warn("EtcdDiscovery: skip invalid entry", zap.String("key", string(kv.Key)), zap.Error(err))
			continue
		}
		c := item.ActorConfig
		newCache[c.ActorType] = append(newCache[c.ActorType], c)
	}
	d.cache.Store(&newCache)
	return nil
}

func (d *EtcdDiscovery) watch() {
	defer zlog.Recover("EtcdDiscovery watch", zap.String("component", "watch"))
	defer close(d.ch) // Close channel on exit to prevent upper-layer range from blocking forever / 退出时关闭通道，避免上层 range ch 永久阻塞
	for {
		select {
		case <-d.ctx.Done():
			return
		default:
		}
		wch := d.cli.Watch(d.ctx, "/servers/", clientv3.WithPrefix())
		for resp := range wch {
			if resp.Err() != nil {
				zlog.Error("EtcdDiscovery: watch error", zap.Error(resp.Err()))
				time.Sleep(2 * time.Second)
				break // Break inner loop and re-watch / 跳出内层循环，重新 Watch
			}
			for _, ev := range resp.Events {
				switch ev.Type {
				case clientv3.EventTypePut:
					d.handlePut(ev)
				case clientv3.EventTypeDelete:
					d.handleDelete(ev)
				}
			}
		}
		// channel 关闭，检查是否因 ctx 取消
		select {
		case <-d.ctx.Done():
			return
		default:
		}
		// Reload cache and retry watch.
		// 重新加载缓存并重试 Watch。
		if err := d.loadAllToCache(); err != nil {
			zlog.Warn("EtcdDiscovery: reload cache after watch close failed", zap.Error(err))
		}
		time.Sleep(2 * time.Second)
	}
}

func (d *EtcdDiscovery) handlePut(ev *clientv3.Event) {
	var item zmodel.ActorServerRegister
	if err := zserialize.UnmarshalJson(ev.Kv.Value, &item); err != nil {
		return
	}
	c := item.ActorConfig

	old := d.cache.Load()
	m := cloneCache(ptrToMap(old))
	if m == nil {
		m = make(map[uint32][]zmodel.ActorConfig)
	}
	actors := m[c.ActorType]
	found := false
	for i, a := range actors {
		if a.Id == c.Id {
			actors[i] = c
			found = true
			break
		}
	}
	if !found {
		m[c.ActorType] = append(actors, c)
	}
	d.cache.Store(&m)

	select {
	case d.ch <- c:
	default:
	}
}

func (d *EtcdDiscovery) handleDelete(ev *clientv3.Event) {
	// Deserialize from pre-delete value to avoid relying on key format parsing.
	// 从删除前的 value 反序列化，避免依赖 Key 格式（如含空格等导致解析失败）。
	var deleted zmodel.ActorConfig
	if ev.PrevKv != nil && len(ev.PrevKv.Value) > 0 {
		var item zmodel.ActorServerRegister
		if err := zserialize.UnmarshalJson(ev.PrevKv.Value, &item); err == nil {
			deleted = item.ActorConfig
		}
	}
	if deleted.Id == 0 && deleted.ActorType == 0 {
		// Fallback: when PrevKv is absent, parse from Key for compatibility.
		// 回退：无 PrevKv 时仍尝试从 Key 解析（兼容旧事件或实现差异）。
		parts := strings.Split(strings.TrimPrefix(string(ev.Kv.Key), "/servers/"), "/")
		if len(parts) >= 2 {
			if at, e1 := strconv.ParseUint(parts[0], 10, 32); e1 == nil {
				if id, e2 := strconv.ParseUint(parts[1], 10, 64); e2 == nil {
					deleted.ActorType = uint32(at)
					deleted.Id = id
				}
			}
		}
	}

	old := d.cache.Load()
	m := cloneCache(ptrToMap(old))
	if m != nil {
		actors := m[deleted.ActorType]
		for i, a := range actors {
			if a.Id == deleted.Id {
				m[deleted.ActorType] = append(actors[:i], actors[i+1:]...)
				break
			}
		}
		d.cache.Store(&m)
	}

	if deleted.Id != 0 || deleted.ActorType != 0 {
		select {
		case d.ch <- zmodel.ActorConfig{Id: deleted.Id, ActorType: 0}:
		default:
		}
	}
}

// Register registers an ActorConfig into Etcd with shared lease and updates local cache.
// Register 将 ActorConfig 注册到 Etcd（复用共享租约）并更新本地缓存。
func (d *EtcdDiscovery) Register(c zmodel.ActorConfig) error {
	key := fmt.Sprintf("/servers/%d/%d", c.ActorType, c.Id)
	item := &zmodel.ActorServerRegister{
		Key:         key,
		ActorConfig: c,
	}
	val, err := zserialize.MarshalJson(item)
	if err != nil {
		return zerrs.Wrap(err, zerrs.ErrTypeValidation, "serialize actor register")
	}

	// Atomic flow: lock-check and create lease when absent to avoid concurrent old-lease leakage.
	// 原子化：持锁检查并在无租约时创建，避免多 goroutine 同时创建导致旧租约泄漏。
	d.leaseMu.Lock()
	if d.sharedLeaseID == 0 {
		if err := d.createSharedLeaseLocked(); err != nil {
			d.leaseMu.Unlock()
			return err
		}
	}
	leaseID := d.sharedLeaseID
	d.leaseMu.Unlock()

	_, err = d.cli.Put(d.ctx, key, string(val), clientv3.WithLease(leaseID))
	if err != nil {
		return zerrs.Wrap(err, zerrs.ErrTypeNetwork, "register to etcd")
	}

	oldReg := d.registeredActors.Load()
	newReg := cloneRegistered(ptrToRegistered(oldReg))
	if newReg == nil {
		newReg = make(map[string]zmodel.ActorConfig)
	}
	newReg[key] = c
	d.registeredActors.Store(&newReg)

	old := d.cache.Load()
	m := cloneCache(ptrToMap(old))
	if m == nil {
		m = make(map[uint32][]zmodel.ActorConfig)
	}
	actors := m[c.ActorType]
	exists := false
	for _, a := range actors {
		if a.Id == c.Id {
			exists = true
			break
		}
	}
	if !exists {
		m[c.ActorType] = append(actors, c)
		d.cache.Store(&m)
	}

	return nil
}

// Unregister removes an ActorConfig from Etcd and local cache.
// Unregister 从 Etcd 及本地缓存中注销指定 ActorConfig。
func (d *EtcdDiscovery) Unregister(c zmodel.ActorConfig) error {
	key := fmt.Sprintf("/servers/%d/%d", c.ActorType, c.Id)
	// Unregister 是“退出清理/资源回收”路径：需要继承调用链 value，但避免被 cancel 立即打断。
	ctx, cancel := context.WithTimeout(context.WithoutCancel(d.ctx), 5*time.Second)
	defer cancel()
	_, err := d.cli.Delete(ctx, key)
	if err != nil {
		return zerrs.Wrap(err, zerrs.ErrTypeNetwork, "unregister from etcd")
	}

	oldReg := d.registeredActors.Load()
	newReg := cloneRegistered(ptrToRegistered(oldReg))
	if newReg != nil {
		delete(newReg, key)
		d.registeredActors.Store(&newReg)
	}

	old := d.cache.Load()
	m := cloneCache(ptrToMap(old))
	if m != nil {
		actors := m[c.ActorType]
		for i, a := range actors {
			if a.Id == c.Id {
				m[c.ActorType] = append(actors[:i], actors[i+1:]...)
				d.cache.Store(&m)
				break
			}
		}
	}

	return nil
}

// CloseAll unregisters all registered actors, revokes lease and cancels background context.
// CloseAll 注销所有已注册的 Actor，撤销租约并取消后台上下文（退出时调用）。
func (d *EtcdDiscovery) CloseAll() error {
	pReg := d.registeredActors.Load()
	var actors []zmodel.ActorConfig
	if pReg != nil {
		for _, a := range *pReg {
			actors = append(actors, a)
		}
	}
	for _, a := range actors {
		_ = d.Unregister(a)
	}

	d.leaseMu.Lock()
	if d.sharedLeaseID != 0 {
		revCtx, revCancel := context.WithTimeout(context.WithoutCancel(d.ctx), 5*time.Second)
		_, _ = d.cli.Revoke(revCtx, d.sharedLeaseID)
		revCancel()
		d.sharedLeaseID = 0
	}
	d.leaseMu.Unlock()

	// Cancel last so watch/keepalive background goroutines can exit gracefully.
	// 最后再 cancel：让 watch/keepalive 等后台协程退出。
	d.cancel()
	return nil
}

// findActorsByType returns cached ActorConfig slice for given actorType (readonly, no allocation).
// findActorsByType 按类型返回 cache 内的 []ActorConfig（只读、无分配）；cache 为 CoW 只读视图。
func (d *EtcdDiscovery) findActorsByType(actorType uint32) []zmodel.ActorConfig {
	p := d.cache.Load()
	if p == nil {
		return nil
	}
	return (*p)[actorType]
}

// parseKeyToActorType parses "/servers/{actorType}" into actorType; returns (0, false) on mismatch.
// parseKeyToActorType 解析 "/servers/{actorType}" 得到 actorType；非该格式返回 (0, false)。
// Use LastIndex + substring parsing to avoid per-call strings.Split allocations.
// 使用 LastIndex + 子串解析，避免 strings.Split 每次分配 []string。
func parseKeyToActorType(key string) (uint32, bool) {
	if len(key) == 0 {
		return 0, false
	}
	key = strings.TrimPrefix(key, "/")
	if !strings.HasPrefix(key, "servers/") {
		return 0, false
	}
	idx := strings.LastIndex(key, "/")
	if idx < 0 {
		return 0, false
	}
	n, err := strconv.ParseUint(key[idx+1:], 10, 32)
	if err != nil {
		return 0, false
	}
	return uint32(n), true
}

func (d *EtcdDiscovery) findAllByPrefix(key string) []zmodel.ActorServerRegister {
	// /servers 或 /servers/{actorType}
	key = strings.TrimPrefix(key, "/")
	parts := strings.Split(key, "/")
	if len(parts) < 1 || parts[0] != "servers" {
		return nil
	}

	p := d.cache.Load()
	if p == nil {
		return nil
	}
	cache := *p

	if len(parts) == 1 {
		var out []zmodel.ActorServerRegister
		for actorType, actors := range cache {
			for _, c := range actors {
				out = append(out, zmodel.ActorServerRegister{
					Key:         fmt.Sprintf("/servers/%d/%d", actorType, c.Id),
					ActorConfig: c,
				})
			}
		}
		return out
	}

	actorTypeU, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return nil
	}
	actorType := uint32(actorTypeU)
	actors, ok := cache[actorType]
	if !ok || len(actors) == 0 {
		return nil
	}
	out := make([]zmodel.ActorServerRegister, 0, len(actors))
	for _, c := range actors {
		out = append(out, zmodel.ActorServerRegister{
			Key:         fmt.Sprintf("/servers/%d/%d", c.ActorType, c.Id),
			ActorConfig: c,
		})
	}
	return out
}

// FindAllByPrefix implements ziface.Discoverer and returns cached ActorServerRegister slice by prefix.
// FindAllByPrefix 实现 ziface.Discoverer，按前缀返回缓存中的 ActorServerRegister 列表。
func (d *EtcdDiscovery) FindAllByPrefix(key string) []zmodel.ActorServerRegister {
	return d.findAllByPrefix(key)
}

// FindRandom implements random-selection strategy for a given discovery key.
// FindRandom 实现按给定发现 key 的随机选取策略。
func (d *EtcdDiscovery) FindRandom(key string) zmodel.ActorConfig {
	if actorType, ok := parseKeyToActorType(key); ok {
		actors := d.findActorsByType(actorType)
		if len(actors) == 0 {
			return zmodel.ActorConfig{}
		}
		return zrand.RandomList(actors)
	}
	servers := d.findAllByPrefix(key)
	if len(servers) == 0 {
		return zmodel.ActorConfig{}
	}
	return zrand.RandomList(servers).ActorConfig
}

// FindPoll implements round-robin selection strategy for a given discovery key.
// FindPoll 实现按给定发现 key 的轮询选取策略。
func (d *EtcdDiscovery) FindPoll(key string) zmodel.ActorConfig {
	if actorType, ok := parseKeyToActorType(key); ok {
		actors := d.findActorsByType(actorType)
		n := len(actors)
		if n == 0 {
			return zmodel.ActorConfig{}
		}
		counter, _ := d.pollCounters.LoadOrStore(key, new(uint64))
		idx := int(atomic.AddUint64(counter, 1)-1) % n
		return actors[idx]
	}
	servers := d.findAllByPrefix(key)
	n := len(servers)
	if n == 0 {
		return zmodel.ActorConfig{}
	}
	counter, _ := d.pollCounters.LoadOrStore(key, new(uint64))
	idx := int(atomic.AddUint64(counter, 1)-1) % n
	return servers[idx].ActorConfig
}

// FindMod implements stable modulo-based selection for given actorType and userId.
// FindMod 实现按 actorType 和 userId 的稳定取模选取策略。
func (d *EtcdDiscovery) FindMod(actorType uint32, userId uint64) zmodel.ActorConfig {
	actors := d.findActorsByType(actorType)
	n := len(actors)
	if n == 0 {
		return zmodel.ActorConfig{}
	}
	// Uniform modulo maps uint64 userId to [0, n-1] with stable distribution.
	// 均匀取模：将 uint64 的 userId 映射到 [0, n-1]，分布均匀且稳定。
	mod := uint64(n)
	idx := userId % mod
	return actors[int(idx)]
}

// Watch implements ziface.Discoverer and returns change-notification channel.
// Watch 实现 ziface.Discoverer，返回发现变更通知通道。
func (d *EtcdDiscovery) Watch() chan zmodel.ActorConfig {
	return d.ch
}
