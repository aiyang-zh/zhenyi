package zmonitor

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aiyang-zh/zhenyi-base/ziface"
)

// IMonitorable is the interface for monitorable components.
// IMonitorable 可监控的组件接口。
type IMonitorable interface {
	GetMonitorData() MonitorData
}

// MonitorData is base structure for monitoring payload.
// MonitorData 监控数据基础结构。
type MonitorData struct {
	Type      string                 `json:"type"`      // Component type / 组件类型
	ID        string                 `json:"id"`        // Unique component ID / 组件唯一标识
	Name      string                 `json:"name"`      // Component name / 组件名称
	Status    string                 `json:"status"`    // running|idle|busy|closed / "running" | "idle" | "busy" | "closed"
	Timestamp int64                  `json:"timestamp"` // Unix timestamp in ms / Unix 毫秒时间戳
	Metrics   map[string]interface{} `json:"metrics"`   // Custom metrics / 自定义指标
	Tags      map[string]string      `json:"tags"`      // Tags for filtering / 标签（用于分类过滤）
}

// ============ Actor 监控 ============

// ActorMonitorData is typed actor monitoring payload.
// ActorMonitorData Actor 监控数据。
type ActorMonitorData struct {
	MonitorData
	ActorID      uint64  `json:"actorId"`
	ActorType    uint32  `json:"actorType"`
	MailCount    int64   `json:"mailCount"`    // Mailbox queue length / 消息队列长度
	ProcessedMsg int64   `json:"processedMsg"` // Processed message count / 已处理消息数
	AvgLatencyMs float64 `json:"avgLatencyMs"` // Average latency in ms / 平均延迟（毫秒）
	QPS          float64 `json:"qps"`          // Messages per second / 每秒处理消息数
	SlowCount    int64   `json:"slowCount"`    // Slow-message count / 慢消息数量
	ErrorCount   int64   `json:"errorCount"`   // Error count / 错误数量
}

// ActorStats holds actor runtime counters for monitoring (internal, high-performance).
// ActorStats Actor 统计信息（内部使用，高性能）。
type ActorStats struct {
	processedMsg atomic.Int64 // Processed message count / 已处理消息数
	totalLatency atomic.Int64 // Total latency in ns / 总延迟（纳秒）
	slowCount    atomic.Int64 // Slow-message count / 慢消息数
	errorCount   atomic.Int64 // Error count / 错误数
	startTime    int64        // Start time / 启动时间
}

// NewActorStats creates an ActorStats with current start time.
// NewActorStats 创建 ActorStats，并记录启动时间。
func NewActorStats() *ActorStats {
	return &ActorStats{
		startTime: time.Now().UnixNano(),
	}
}

// RecordMessage records one message latency and slow flag.
// RecordMessage 记录一条消息的处理延迟与是否慢消息标记。
func (s *ActorStats) RecordMessage(latencyNs int64, isSlow bool) {
	s.processedMsg.Add(1)
	s.totalLatency.Add(latencyNs)
	if isSlow {
		s.slowCount.Add(1)
	}
}

// RecordError records one error.
// RecordError 记录一次错误。
func (s *ActorStats) RecordError() {
	s.errorCount.Add(1)
}

// GetSnapshot returns aggregated counters snapshot.
// GetSnapshot 返回聚合计数快照。
func (s *ActorStats) GetSnapshot() (processed, avgLatencyMs int64, qps float64, slowCount, errorCount int64) {
	processed = s.processedMsg.Load()
	totalLatency := s.totalLatency.Load()
	slowCount = s.slowCount.Load()
	errorCount = s.errorCount.Load()

	if processed > 0 {
		avgLatencyMs = (totalLatency / processed) / 1e6 // ns -> ms / 纳秒转毫秒
	}

	elapsed := time.Since(time.Unix(0, s.startTime)).Seconds()
	if elapsed > 0 {
		qps = float64(processed) / elapsed
	}

	return
}

// ============ Session 监控 ============

// SessionMonitorData is typed session monitoring payload.
// SessionMonitorData Session 监控数据。
type SessionMonitorData struct {
	MonitorData
	SessionID     int64   `json:"sessionId"`
	AuthID        int64   `json:"authId"`
	ConnectedTime int64   `json:"connectedTime"` // Connected time (unix ms) / 连接时间（Unix毫秒）
	SendCount     int64   `json:"sendCount"`     // Sent message count / 发送消息数
	RecvCount     int64   `json:"recvCount"`     // Received message count / 接收消息数
	SendBytes     int64   `json:"sendBytes"`     // Sent bytes / 发送字节数
	RecvBytes     int64   `json:"recvBytes"`     // Received bytes / 接收字节数
	MailBoxSize   int64   `json:"mailBoxSize"`   // Mailbox queue size / 邮箱队列长度
	LastActiveMs  int64   `json:"lastActiveMs"`  // Last active time / 最后活跃时间
	RTTMs         float64 `json:"rttMs"`         // RTT（毫秒）
}

// SessionStats holds session counters for monitoring.
// SessionStats Session 统计信息。
type SessionStats struct {
	sendCount    atomic.Int64
	recvCount    atomic.Int64
	sendBytes    atomic.Int64
	recvBytes    atomic.Int64
	lastActiveMs atomic.Int64
	connectedAt  int64
}

// NewSessionStats creates a SessionStats snapshot container.
// NewSessionStats 创建 SessionStats 统计容器。
func NewSessionStats() *SessionStats {
	now := time.Now().UnixMilli()
	return &SessionStats{
		connectedAt: now,
	}
}

// RecordSend records one send operation with count and bytes.
// RecordSend 记录一次发送（条数与字节数）。
func (s *SessionStats) RecordSend(count int, bytes int) {
	s.sendCount.Add(int64(count))
	s.sendBytes.Add(int64(bytes))
	s.lastActiveMs.Store(time.Now().UnixMilli())
}

// RecordRecv records one receive operation with bytes.
// RecordRecv 记录一次接收（字节数）。
func (s *SessionStats) RecordRecv(bytes int) {
	s.recvCount.Add(1)
	s.recvBytes.Add(int64(bytes))
	s.lastActiveMs.Store(time.Now().UnixMilli())
}

// RecordRec implements ziface.ISessionStats (kept consistent with zhenyi-base naming).
// RecordRec 实现 ziface.ISessionStats（与 zhenyi-base 层命名保持一致）。
func (s *SessionStats) RecordRec(bytes int) {
	s.RecordRecv(bytes)
}

var _ ziface.ISessionStats = (*SessionStats)(nil)

// GetSnapshot returns session counter snapshot.
// GetSnapshot 返回会话计数快照。
func (s *SessionStats) GetSnapshot() (sendCount, recvCount, sendBytes, recvBytes, connectedAt, lastActiveMs int64) {
	return s.sendCount.Load(), s.recvCount.Load(), s.sendBytes.Load(), s.recvBytes.Load(), s.connectedAt, s.lastActiveMs.Load()
}

// SessionStatsValues implements ziface.ISessionStatsSnapshot for upstream aggregation (Gate/Prometheus).
// SessionStatsValues 实现 ziface.ISessionStatsSnapshot，供上层聚合会话指标（Gate / Prometheus）。
func (s *SessionStats) SessionStatsValues() (sendCount, recvCount, sendBytes, recvBytes, connectedAtMs, lastActiveMs int64) {
	return s.GetSnapshot()
}

var _ ziface.ISessionStatsSnapshot = (*SessionStats)(nil)

// ============ GateServer 监控 ============

// GateMonitorData is typed gate monitoring payload.
// GateMonitorData GateServer 监控数据。
type GateMonitorData struct {
	MonitorData
	OnlineCount   int     `json:"onlineCount"`   // Online user count / 在线人数
	TotalSessions int64   `json:"totalSessions"` // Total session count / 总连接数
	CurrentQPS    float64 `json:"currentQPS"`    // Current QPS / 当前QPS
	GlobalQPS     float64 `json:"globalQPS"`     // Global QPS / 全局QPS
	AvgRTTMs      float64 `json:"avgRttMs"`      // Average RTT / 平均RTT
	P99RTTMs      float64 `json:"p99RttMs"`      // P99 RTT
	MemAllocMB    float64 `json:"memAllocMB"`    // Memory allocation (MB) / 内存分配（MB）
	GoroutineNum  int     `json:"goroutineNum"`  // Goroutine 数量
	GCCount       uint32  `json:"gcCount"`       // GC 次数
}

// ============ 系统监控 ============

// SystemMonitorData is typed system/runtime monitoring payload.
// SystemMonitorData 系统级监控数据。
type SystemMonitorData struct {
	MonitorData
	MemStats     *MemoryStats  `json:"memStats"`
	GCStats      *GCStats      `json:"gcStats"`
	GoroutineNum int           `json:"goroutineNum"`
	CPUNum       int           `json:"cpuNum"`
	RuntimeStats *RuntimeStats `json:"runtimeStats"`
}

// MemoryStats is memory statistics snapshot.
// MemoryStats 内存统计。
type MemoryStats struct {
	AllocMB      float64 `json:"allocMB"`      // Current allocated memory (MB) / 当前分配内存（MB）
	TotalAllocMB float64 `json:"totalAllocMB"` // Cumulative allocated memory (MB) / 累计分配内存（MB）
	SysMB        float64 `json:"sysMB"`        // System memory (MB) / 系统内存（MB）
	HeapAllocMB  float64 `json:"heapAllocMB"`  // Heap allocation (MB) / 堆内存（MB）
	HeapSysMB    float64 `json:"heapSysMB"`    // Heap system memory (MB) / 堆系统内存（MB）
	HeapIdleMB   float64 `json:"heapIdleMB"`   // Heap idle memory (MB) / 堆空闲内存（MB）
	HeapInuseMB  float64 `json:"heapInuseMB"`  // Heap in-use memory (MB) / 堆使用内存（MB）
}

// GCStats is GC statistics snapshot.
// GCStats GC 统计。
type GCStats struct {
	NumGC         uint32  `json:"numGC"`         // GC 次数
	PauseTotalMs  float64 `json:"pauseTotalMs"`  // Total GC pause (ms) / 总暂停时间（毫秒）
	PauseAvgMs    float64 `json:"pauseAvgMs"`    // Average GC pause (ms) / 平均暂停时间（毫秒）
	LastPauseMs   float64 `json:"lastPauseMs"`   // Last GC pause (ms) / 最后一次暂停时间（毫秒）
	GCCPUFraction float64 `json:"gcCpuFraction"` // GC CPU 占用率
}

// RuntimeStats is runtime statistics snapshot.
// RuntimeStats 运行时统计。
type RuntimeStats struct {
	Uptime       float64 `json:"uptime"`       // Uptime in seconds / 运行时间（秒）
	NumCPU       int     `json:"numCPU"`       // CPU 核心数
	NumGoroutine int     `json:"numGoroutine"` // Goroutine 数量
	NumCgoCall   int64   `json:"numCgoCall"`   // CGO 调用次数
}

// ============ 工具函数 ============

const defaultSystemMonitorCacheInterval = 1 * time.Second

var systemMonitorCache struct {
	mu sync.Mutex

	// intervalNs 控制最短刷新间隔；0 表示禁用缓存（每次都采样）。
	intervalNs atomic.Int64

	lastSampleNs atomic.Int64
	last         *SystemMonitorData
}

func init() {
	systemMonitorCache.intervalNs.Store(int64(defaultSystemMonitorCacheInterval))
}

// SetSystemMonitorCacheInterval 设置系统监控采样缓存的最短刷新间隔。
// Set to 0 to disable cache and run ReadMemStats on every call.
// 设为 0 表示禁用缓存（每次调用都执行一次 ReadMemStats）。
func SetSystemMonitorCacheInterval(d time.Duration) {
	if d < 0 {
		d = 0
	}
	systemMonitorCache.intervalNs.Store(int64(d))
}

// CollectSystemMonitor collects system/runtime monitoring data with cache.
// CollectSystemMonitor 收集系统监控数据（带缓存）。
func CollectSystemMonitor() *SystemMonitorData {
	interval := time.Duration(systemMonitorCache.intervalNs.Load())
	if interval <= 0 {
		return collectSystemMonitorFresh()
	}

	nowNs := time.Now().UnixNano()
	lastNs := systemMonitorCache.lastSampleNs.Load()
	if last := systemMonitorCache.last; last != nil && nowNs-lastNs < int64(interval) {
		return cloneSystemMonitorData(last)
	}

	systemMonitorCache.mu.Lock()
	defer systemMonitorCache.mu.Unlock()

	lastNs = systemMonitorCache.lastSampleNs.Load()
	if last := systemMonitorCache.last; last != nil && nowNs-lastNs < int64(interval) {
		return cloneSystemMonitorData(last)
	}

	fresh := collectSystemMonitorFresh()
	systemMonitorCache.last = fresh
	systemMonitorCache.lastSampleNs.Store(nowNs)
	return cloneSystemMonitorData(fresh)
}

func cloneSystemMonitorData(in *SystemMonitorData) *SystemMonitorData {
	if in == nil {
		return nil
	}
	out := *in // shallow copy
	if in.MemStats != nil {
		mem := *in.MemStats
		out.MemStats = &mem
	}
	if in.GCStats != nil {
		gc := *in.GCStats
		out.GCStats = &gc
	}
	if in.RuntimeStats != nil {
		rs := *in.RuntimeStats
		out.RuntimeStats = &rs
	}
	if in.Metrics != nil {
		out.Metrics = make(map[string]interface{}, len(in.Metrics))
		for k, v := range in.Metrics {
			out.Metrics[k] = v
		}
	}
	if in.Tags != nil {
		out.Tags = make(map[string]string, len(in.Tags))
		for k, v := range in.Tags {
			out.Tags[k] = v
		}
	}
	return &out
}

func collectSystemMonitorFresh() *SystemMonitorData {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	data := &SystemMonitorData{
		MonitorData: MonitorData{
			Type:      "system",
			ID:        "system",
			Name:      "System",
			Status:    "running",
			Timestamp: time.Now().UnixMilli(),
			Metrics:   make(map[string]interface{}),
			Tags:      make(map[string]string),
		},
		MemStats: &MemoryStats{
			AllocMB:      float64(memStats.Alloc) / 1024 / 1024,
			TotalAllocMB: float64(memStats.TotalAlloc) / 1024 / 1024,
			SysMB:        float64(memStats.Sys) / 1024 / 1024,
			HeapAllocMB:  float64(memStats.HeapAlloc) / 1024 / 1024,
			HeapSysMB:    float64(memStats.HeapSys) / 1024 / 1024,
			HeapIdleMB:   float64(memStats.HeapIdle) / 1024 / 1024,
			HeapInuseMB:  float64(memStats.HeapInuse) / 1024 / 1024,
		},
		GCStats: &GCStats{
			NumGC:         memStats.NumGC,
			PauseTotalMs:  float64(memStats.PauseTotalNs) / 1e6,
			GCCPUFraction: memStats.GCCPUFraction,
		},
		GoroutineNum: runtime.NumGoroutine(),
		CPUNum:       runtime.NumCPU(),
		RuntimeStats: &RuntimeStats{
			NumCPU:       runtime.NumCPU(),
			NumGoroutine: runtime.NumGoroutine(),
			NumCgoCall:   runtime.NumCgoCall(),
		},
	}

	// Calculate average pause time.
	// 计算平均暂停时间。
	if memStats.NumGC > 0 {
		data.GCStats.PauseAvgMs = data.GCStats.PauseTotalMs / float64(memStats.NumGC)
	}

	// Last pause duration.
	// 最后一次暂停时间。
	if memStats.NumGC > 0 {
		data.GCStats.LastPauseMs = float64(memStats.PauseNs[(memStats.NumGC+255)%256]) / 1e6
	}

	return data
}

// ============ 监控管理器 ============

// Manager aggregates monitorable components and provides snapshot queries.
// Manager 监控管理器（并发安全）。
type Manager struct {
	mu         sync.RWMutex
	components map[string]IMonitorable
}

// NewManager creates a monitoring manager.
// NewManager 创建监控管理器。
func NewManager() *Manager {
	return &Manager{
		components: make(map[string]IMonitorable),
	}
}

// Register registers a monitorable component by id.
// Register 注册可监控组件。
func (m *Manager) Register(id string, component IMonitorable) {
	m.mu.Lock()
	m.components[id] = component
	m.mu.Unlock()
}

// Unregister removes a registered component by id.
// Unregister 注销组件。
func (m *Manager) Unregister(id string) {
	m.mu.Lock()
	delete(m.components, id)
	m.mu.Unlock()
}

// GetAll returns monitoring snapshots of all registered components.
// GetAll 获取所有组件监控数据。
func (m *Manager) GetAll() []MonitorData {
	// Avoid calling comp.GetMonitorData() while lock is held:
	// 不要在持锁期间调用 comp.GetMonitorData()：
	// component collection may slow/block (or indirectly register/unregister), amplifying read-lock into global latency.
	// 组件采集可能变慢/阻塞（甚至间接触发注册/注销），会把读锁放大成全局延迟。
	m.mu.RLock()
	comps := make([]IMonitorable, 0, len(m.components))
	for _, comp := range m.components {
		comps = append(comps, comp)
	}
	m.mu.RUnlock()

	result := make([]MonitorData, 0, len(comps))
	for _, comp := range comps {
		result = append(result, comp.GetMonitorData())
	}
	return result
}

// Get returns monitoring snapshot for specified component id.
// Get 获取指定组件监控数据。
func (m *Manager) Get(id string) (MonitorData, bool) {
	m.mu.RLock()
	comp, ok := m.components[id]
	m.mu.RUnlock()
	if !ok {
		return MonitorData{}, false
	}
	return comp.GetMonitorData(), true
}

// GetByType returns monitoring snapshots filtered by MonitorData.Type.
// GetByType 按类型获取监控数据。
func (m *Manager) GetByType(typ string) []MonitorData {
	m.mu.RLock()
	comps := make([]IMonitorable, 0, len(m.components))
	for _, comp := range m.components {
		comps = append(comps, comp)
	}
	m.mu.RUnlock()

	result := make([]MonitorData, 0)
	for _, comp := range comps {
		data := comp.GetMonitorData()
		if data.Type == typ {
			result = append(result, data)
		}
	}
	return result
}
