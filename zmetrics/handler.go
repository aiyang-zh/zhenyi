package zmetrics

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// HandlerMetric is per-handler Prometheus metric holder for zero-allocation hot path.
// HandlerMetric 单个 Handler 的 Prometheus 指标（零分配热路径）。
//
// Each (actorId, msgId) pair maps to one metric instance at actor-instance granularity.
// 每个 (actorId, msgId) 组合对应一个实例，精确到每个 Actor 实例。
// 热路径上只有 3 次 atomic 操作：Inc + ObserveDuration + (条件)慢消息 Inc
type HandlerMetric struct {
	total   Counter
	latency Histogram
	slow    Counter
}

// HandlerSlowLogThreshold is slow-call threshold for handlers.
// HandlerSlowLogThreshold 为 handler 级慢调用阈值。
// 默认 10ms，可在进程启动阶段按需覆盖（例如从统一配置加载）。
var HandlerSlowLogThreshold = 10 * time.Millisecond

// RecordCall records one handler invocation (~8ns hot path).
// RecordCall 记录一次 handler 调用（热路径，~8ns: 1 atomic inc + 1 histogram observe）。
func (m *HandlerMetric) RecordCall(cost time.Duration) {
	m.total.Inc()
	m.latency.ObserveDuration(cost)
	if cost > HandlerSlowLogThreshold {
		m.slow.Inc()
	}
}

type handlerKey struct {
	actorId uint64
	msgId   int32
}

type handlerLabelEntry struct {
	key    handlerKey
	metric *HandlerMetric
	label  string // pre-formatted: `{handler="1001",actor_id="1",actor_type="2"}`
}

var (
	handlerMu      sync.Mutex
	handlerMap     = make(map[handlerKey]*HandlerMetric)
	handlerEntries []handlerLabelEntry
	handlerSorted  bool
)

// GetHandlerMetric gets or creates per-handler metrics.
// GetHandlerMetric 获取/创建 per-handler 指标。
// 每个 (actorId, msgId) 对应独立实例，精确到 Actor 实例级别。
// 同时携带 actorType 标签，支持按类型聚合（防止 actorId 基数膨胀时仍可降维查询）。
// 仅在 Init 阶段调用（单线程注册），运行时零开销。
func GetHandlerMetric(actorId uint64, actorType uint32, msgId int32) *HandlerMetric {
	key := handlerKey{actorId, msgId}

	handlerMu.Lock()
	defer handlerMu.Unlock()

	if m, ok := handlerMap[key]; ok {
		return m
	}

	m := &HandlerMetric{
		latency: *NewHistogram(DefaultLatencyBounds),
	}
	handlerMap[key] = m

	label := fmt.Sprintf(`{handler="%d",actor_id="%d",actor_type="%d"}`, msgId, actorId, actorType)
	handlerEntries = append(handlerEntries, handlerLabelEntry{
		key:    key,
		metric: m,
		label:  label,
	})
	handlerSorted = false

	return m
}

// WriteHandlerPrometheus outputs per-handler metrics in Prometheus text exposition format.
// WriteHandlerPrometheus 输出 per-handler 指标的 Prometheus text exposition 格式。
// 在 Registry.WritePrometheus 之后调用。
func WriteHandlerPrometheus(b *strings.Builder) {
	handlerMu.Lock()
	if !handlerSorted {
		sort.Slice(handlerEntries, func(i, j int) bool {
			if handlerEntries[i].key.actorId != handlerEntries[j].key.actorId {
				return handlerEntries[i].key.actorId < handlerEntries[j].key.actorId
			}
			return handlerEntries[i].key.msgId < handlerEntries[j].key.msgId
		})
		handlerSorted = true
	}
	entries := handlerEntries
	handlerMu.Unlock()

	if len(entries) == 0 {
		return
	}

	// ---- zhenyi_handler_total (Counter) ----
	b.WriteString("# HELP zhenyi_handler_total Per-handler call count\n")
	b.WriteString("# TYPE zhenyi_handler_total counter\n")
	for _, e := range entries {
		val := e.metric.total.Load()
		if val == 0 {
			continue
		}
		b.WriteString("zhenyi_handler_total")
		b.WriteString(e.label)
		b.WriteByte(' ')
		appendInt(b, val)
		b.WriteByte('\n')
	}

	// ---- zhenyi_handler_slow_total (Counter) ----
	b.WriteString("# HELP zhenyi_handler_slow_total Per-handler slow call count (>10ms)\n")
	b.WriteString("# TYPE zhenyi_handler_slow_total counter\n")
	for _, e := range entries {
		val := e.metric.slow.Load()
		if val == 0 {
			continue
		}
		b.WriteString("zhenyi_handler_slow_total")
		b.WriteString(e.label)
		b.WriteByte(' ')
		appendInt(b, val)
		b.WriteByte('\n')
	}

	// ---- zhenyi_handler_latency_ms (Histogram) ----
	b.WriteString("# HELP zhenyi_handler_latency_ms Per-handler latency in ms\n")
	b.WriteString("# TYPE zhenyi_handler_latency_ms histogram\n")
	for _, e := range entries {
		bounds, cumulative, sum, count := e.metric.latency.Snapshot()
		if count == 0 {
			continue
		}
		for i, bound := range bounds {
			b.WriteString("zhenyi_handler_latency_ms_bucket")
			writeLabelsWithLE(b, e.label, bound)
			b.WriteByte(' ')
			appendInt(b, cumulative[i])
			b.WriteByte('\n')
		}
		b.WriteString("zhenyi_handler_latency_ms_bucket")
		writeLabelsWithLEInf(b, e.label)
		b.WriteByte(' ')
		appendInt(b, cumulative[len(cumulative)-1])
		b.WriteByte('\n')

		b.WriteString("zhenyi_handler_latency_ms_sum")
		b.WriteString(e.label)
		b.WriteByte(' ')
		appendFloat(b, sum)
		b.WriteByte('\n')

		b.WriteString("zhenyi_handler_latency_ms_count")
		b.WriteString(e.label)
		b.WriteByte(' ')
		appendInt(b, count)
		b.WriteByte('\n')
	}
}

// writeLabelsWithLE converts `{handler="1001",actor_id="1",actor_type="2"}` into `{handler="1001",actor_id="1",actor_type="2",le="5"}`.
// writeLabelsWithLE 将 `{handler="1001",actor_id="1",actor_type="2"}` 转换为 `{handler="1001",actor_id="1",actor_type="2",le="5"}`
func writeLabelsWithLE(b *strings.Builder, baseLabel string, le float64) {
	b.WriteString(baseLabel[:len(baseLabel)-1]) // 去掉末尾 `}`
	b.WriteString(",le=\"")
	appendFloat(b, le)
	b.WriteString("\"}")
}

func writeLabelsWithLEInf(b *strings.Builder, baseLabel string) {
	b.WriteString(baseLabel[:len(baseLabel)-1])
	b.WriteString(",le=\"+Inf\"}")
}
