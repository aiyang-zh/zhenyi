package zmetrics

import (
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Counter is a monotonic increasing counter (Prometheus counter).
// Counter 单调递增计数器（Prometheus counter）。
type Counter struct {
	val int64
}

// Inc increases counter by 1.
// Inc 计数器加 1。
func (c *Counter) Inc() { atomic.AddInt64(&c.val, 1) }

// Add increases counter by v.
// Add 计数器增加 v。
func (c *Counter) Add(v int64) { atomic.AddInt64(&c.val, v) }

// Load returns current counter value.
// Load 返回当前计数值。
func (c *Counter) Load() int64 { return atomic.LoadInt64(&c.val) }

// Swap sets counter to v and returns old value.
// Swap 将计数器设置为 v 并返回旧值。
func (c *Counter) Swap(v int64) int64 { return atomic.SwapInt64(&c.val, v) }

// Gauge is an up/down metric value (Prometheus gauge).
// Gauge 可升可降度量值（Prometheus gauge）。
type Gauge struct {
	val int64
}

// Set sets gauge to v.
// Set 将 gauge 设置为 v。
func (g *Gauge) Set(v int64) { atomic.StoreInt64(&g.val, v) }

// Inc increases gauge by 1.
// Inc 将 gauge 加 1。
func (g *Gauge) Inc() { atomic.AddInt64(&g.val, 1) }

// Dec decreases gauge by 1.
// Dec 将 gauge 减 1。
func (g *Gauge) Dec() { atomic.AddInt64(&g.val, -1) }

// Add increases gauge by v (v can be negative).
// Add 将 gauge 增加 v（v 可为负）。
func (g *Gauge) Add(v int64) { atomic.AddInt64(&g.val, v) }

// Load returns current gauge value.
// Load 返回当前 gauge 值。
func (g *Gauge) Load() int64 { return atomic.LoadInt64(&g.val) }

// SetFloat stores a float64 value into gauge.
// SetFloat 将 float64 值写入 gauge。
func (g *Gauge) SetFloat(v float64) {
	atomic.StoreInt64(&g.val, int64(math.Float64bits(v)))
}

// LoadFloat loads float64 value from gauge.
// LoadFloat 从 gauge 读取 float64 值。
func (g *Gauge) LoadFloat() float64 {
	return math.Float64frombits(uint64(atomic.LoadInt64(&g.val)))
}

// Histogram is a fixed-boundary histogram suitable for latency stats.
// Histogram 直方图（固定桶边界，适合延迟统计）。
type Histogram struct {
	bounds  []float64
	buckets []int64 // len = len(bounds)+1, last bucket is +Inf / len = len(bounds)+1, 最后一个是 +Inf
	sum     int64   // Sum in nanoseconds / 纳秒总和
	count   int64
}

// NewHistogram creates a histogram with given bucket boundaries (in ms).
// NewHistogram 创建直方图，bounds 为桶边界（单位：毫秒）。
func NewHistogram(bounds []float64) *Histogram {
	return &Histogram{
		bounds:  bounds,
		buckets: make([]int64, len(bounds)+1),
	}
}

// DefaultLatencyBounds is default latency bucket boundaries in milliseconds.
// DefaultLatencyBounds 默认延迟桶边界（毫秒）。
var DefaultLatencyBounds = []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 5000}

// NewLatencyHistogram creates a histogram using DefaultLatencyBounds.
// NewLatencyHistogram 使用 DefaultLatencyBounds 创建延迟直方图。
func NewLatencyHistogram() *Histogram {
	return NewHistogram(DefaultLatencyBounds)
}

// Observe adds one observation in milliseconds.
// Observe 记录一次观测值（毫秒）。
func (h *Histogram) Observe(valueMs float64) {
	atomic.AddInt64(&h.count, 1)
	atomic.AddInt64(&h.sum, int64(valueMs*1e6)) // Store as nanoseconds / 存纳秒

	idx := sort.SearchFloat64s(h.bounds, valueMs)
	if idx > len(h.buckets)-1 {
		idx = len(h.buckets) - 1
	}
	atomic.AddInt64(&h.buckets[idx], 1)
}

// ObserveDuration adds one observation from time.Duration.
// ObserveDuration 从 time.Duration 记录一次观测值。
func (h *Histogram) ObserveDuration(d time.Duration) {
	// Use milliseconds consistently while preserving sub-ms precision.
	// 统一使用“毫秒”为单位，保留 sub-ms 精度。
	h.Observe(float64(d) / float64(time.Millisecond))
}

// Snapshot returns histogram snapshot (bounds, cumulative counts, sum, total count).
// Snapshot 返回直方图快照（桶边界、累积计数、总和、总数）。
func (h *Histogram) Snapshot() (bounds []float64, cumulative []int64, sum float64, count int64) {
	count = atomic.LoadInt64(&h.count)
	sum = float64(atomic.LoadInt64(&h.sum)) / 1e6 // Milliseconds / 毫秒
	bounds = h.bounds
	cumulative = make([]int64, len(h.buckets))
	var acc int64
	for i := range h.buckets {
		acc += atomic.LoadInt64(&h.buckets[i])
		cumulative[i] = acc
	}
	return
}

// Registry is the global metric registry.
// Registry 全局指标注册表。
type Registry struct {
	mu         sync.RWMutex
	counters   map[string]*counterEntry
	gauges     map[string]*gaugeEntry
	histograms map[string]*histogramEntry
}

type counterEntry struct {
	metric *Counter
	help   string
}
type gaugeEntry struct {
	metric *Gauge
	help   string
}
type histogramEntry struct {
	metric *Histogram
	help   string
}

var globalRegistry = &Registry{
	counters:   make(map[string]*counterEntry),
	gauges:     make(map[string]*gaugeEntry),
	histograms: make(map[string]*histogramEntry),
}

// Global returns the global metric registry.
// Global 返回全局指标注册表。
func Global() *Registry { return globalRegistry }

// Counter registers (or returns existing) counter metric.
// Counter 注册（或返回已存在的）counter 指标。
func (r *Registry) Counter(name, help string) *Counter {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e, ok := r.counters[name]; ok {
		return e.metric
	}
	c := &Counter{}
	r.counters[name] = &counterEntry{metric: c, help: help}
	return c
}

// Gauge registers (or returns existing) gauge metric.
// Gauge 注册（或返回已存在的）gauge 指标。
func (r *Registry) Gauge(name, help string) *Gauge {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e, ok := r.gauges[name]; ok {
		return e.metric
	}
	g := &Gauge{}
	r.gauges[name] = &gaugeEntry{metric: g, help: help}
	return g
}

// Histogram registers (or returns existing) histogram metric.
// Histogram 注册（或返回已存在的）histogram 指标。
func (r *Registry) Histogram(name, help string, bounds []float64) *Histogram {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e, ok := r.histograms[name]; ok {
		return e.metric
	}
	h := NewHistogram(bounds)
	r.histograms[name] = &histogramEntry{metric: h, help: help}
	return h
}

// WritePrometheus writes Prometheus text exposition format (including per-handler metrics).
// WritePrometheus 输出 Prometheus text exposition format（含 per-handler 指标）。
func (r *Registry) WritePrometheus(b *strings.Builder) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Counters
	names := sortedKeys(r.counters)
	for _, name := range names {
		e := r.counters[name]
		if e.help != "" {
			b.WriteString("# HELP ")
			b.WriteString(name)
			b.WriteByte(' ')
			b.WriteString(e.help)
			b.WriteByte('\n')
		}
		b.WriteString("# TYPE ")
		b.WriteString(name)
		b.WriteString(" counter\n")
		b.WriteString(name)
		b.WriteByte(' ')
		appendInt(b, e.metric.Load())
		b.WriteByte('\n')
	}

	// Gauges
	gNames := sortedGaugeKeys(r.gauges)
	for _, name := range gNames {
		e := r.gauges[name]
		if e.help != "" {
			b.WriteString("# HELP ")
			b.WriteString(name)
			b.WriteByte(' ')
			b.WriteString(e.help)
			b.WriteByte('\n')
		}
		b.WriteString("# TYPE ")
		b.WriteString(name)
		b.WriteString(" gauge\n")
		b.WriteString(name)
		b.WriteByte(' ')
		appendInt(b, e.metric.Load())
		b.WriteByte('\n')
	}

	// Histograms
	hNames := sortedHistKeys(r.histograms)
	for _, name := range hNames {
		e := r.histograms[name]
		if e.help != "" {
			b.WriteString("# HELP ")
			b.WriteString(name)
			b.WriteByte(' ')
			b.WriteString(e.help)
			b.WriteByte('\n')
		}
		b.WriteString("# TYPE ")
		b.WriteString(name)
		b.WriteString(" histogram\n")
		bounds, cumulative, sum, count := e.metric.Snapshot()
		for i, bound := range bounds {
			b.WriteString(name)
			b.WriteString("_bucket{le=\"")
			appendFloat(b, bound)
			b.WriteString("\"} ")
			appendInt(b, cumulative[i])
			b.WriteByte('\n')
		}
		b.WriteString(name)
		b.WriteString("_bucket{le=\"+Inf\"} ")
		appendInt(b, cumulative[len(cumulative)-1])
		b.WriteByte('\n')
		b.WriteString(name)
		b.WriteString("_sum ")
		appendFloat(b, sum)
		b.WriteByte('\n')
		b.WriteString(name)
		b.WriteString("_count ")
		appendInt(b, count)
		b.WriteByte('\n')
	}

	// Per-handler labeled metrics
	WriteHandlerPrometheus(b)
	// Object-pool labeled metrics (same source as zmetrics.GlobalPoolObserver).
	// Object pool labeled metrics（与 zmetrics.GlobalPoolObserver 同源）
	WritePoolPrometheus(b)
	// zmonitor.Manager snapshots (numeric Metrics fields -> zhenyi_monitor_snapshot).
	// zmonitor.Manager 快照（数值型 Metrics 字段 → zhenyi_monitor_snapshot）
	WriteMonitorSnapshotPrometheus(b)
}

func sortedKeys[V any](m map[string]*V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Helper aliases to avoid generic-constraint limitations.
// 类型别名辅助（避免泛型约束限制）。
func sortedGaugeKeys(m map[string]*gaugeEntry) []string    { return sortedKeys(m) }
func sortedHistKeys(m map[string]*histogramEntry) []string { return sortedKeys(m) }

// appendInt/appendFloat avoid fmt.Sprintf allocations.
// appendInt/appendFloat 避免 fmt.Sprintf 分配。
func appendInt(b *strings.Builder, v int64) {
	var buf [20]byte
	n := formatInt(buf[:], v)
	_, _ = b.Write(buf[n:])
}

func formatInt(buf []byte, v int64) int {
	neg := v < 0
	if neg {
		v = -v
	}
	i := len(buf)
	for v >= 10 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	i--
	buf[i] = byte('0' + v)
	if neg {
		i--
		buf[i] = '-'
	}
	return i
}

func appendFloat(b *strings.Builder, v float64) {
	// Simple implementation with sufficient precision.
	// 简单实现，精度够用。
	if v == float64(int64(v)) {
		appendInt(b, int64(v))
		return
	}
	// 6 decimal places.
	// 6 位小数。
	intPart := int64(v)
	fracPart := int64((v - float64(intPart)) * 1e6)
	if fracPart < 0 {
		fracPart = -fracPart
	}
	appendInt(b, intPart)
	b.WriteByte('.')
	// Zero-padding.
	// 补零。
	for d := int64(100000); d > 1; d /= 10 {
		if fracPart < d {
			b.WriteByte('0')
		}
	}
	// Trim trailing zeros.
	// 去掉尾零。
	for fracPart > 0 && fracPart%10 == 0 {
		fracPart /= 10
	}
	if fracPart > 0 {
		appendInt(b, fracPart)
	}
}
