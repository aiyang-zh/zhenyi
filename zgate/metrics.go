package zgate

import (
	"runtime"
	"slices"
	"sync/atomic"
	"time"
)

// EncodeReqKey encodes SessionID + SeqId into uint64 without allocation.
// EncodeReqKey 将 SessionID + SeqId 编码为 uint64（零分配）。
// High 32 bits: low 32 bits of SessionID; low 32 bits: SeqId.
// 高 32 位: SessionID 低 32 位, 低 32 位: SeqId。
func EncodeReqKey(sessionId uint64, seqId uint32) uint64 {
	return (sessionId << 32) | uint64(seqId)
}

// LatencyStats stores latency aggregation results in time.Duration.
// LatencyStats 统计结果容器（单位：time.Duration）。
type LatencyStats struct {
	Min, Max, Avg time.Duration
	P50, P90, P99 time.Duration
	Count         int
}

// ServerMetrics holds gate runtime metrics and lightweight caches.
// ServerMetrics 保存 Gate 运行期指标与轻量缓存数据。
type ServerMetrics struct {
	FirstPacketTime atomic.Int64
	recvCount       atomic.Int64
	sentCount       atomic.Int64
	recvCountTotal  atomic.Int64
	sentCountTotal  atomic.Int64
	OnlineUsers     atomic.Int32

	// RTT tracker (lock-free implementation).
	// RTT 追踪器（无锁实现）。
	RTTTracker *LockFreeRTTTracker

	// Global RTT accumulators.
	// RTT 全局统计。
	GlobalTotalRTT atomic.Int64 // Historical total RTT in ns / 历史总耗时（纳秒）
	GlobalCountRTT atomic.Int64 // Historical total request count / 历史总请求数

	// GC-monitoring helper fields.
	// GC 监控辅助字段。
	lastMemStats    *runtime.MemStats // Last collected memory snapshot / 上一次采集的内存快照
	lastCollectTime time.Time         // Last collection time / 上一次采集的时间

	// Lightweight cache to avoid frequent ReadMemStats STW.
	// 轻量缓存（避免监控接口频繁 ReadMemStats 引发 STW）。
	MemAllocBytes atomic.Uint64
	MemSysBytes   atomic.Uint64
	MemNumGC      atomic.Uint64
}

// RecordRTT accumulates RTT into global counters.
// RecordRTT 记录 RTT 到全局统计。
func (m *ServerMetrics) RecordRTT(cost time.Duration) {
	// Add to global counters in nanoseconds.
	// 累加到全局统计（纳秒）。
	m.GlobalTotalRTT.Add(int64(cost))
	m.GlobalCountRTT.Add(1)
}

// CalculateStats computes latency statistics from nanosecond samples.
// CalculateStats 核心计算逻辑（输入：纳秒，输出：time.Duration）。
func CalculateStats(samples []int64) LatencyStats {
	count := len(samples)
	if count == 0 {
		return LatencyStats{}
	}

	// 1) Sort.
	// 1. 排序。
	slices.Sort(samples)

	// 2) Compute sum and average.
	// 2. 计算 Sum 和 Avg。
	var sum int64
	for _, v := range samples {
		sum += v
	}

	stats := LatencyStats{
		Count: count,
		Min:   time.Duration(samples[0]),
		Max:   time.Duration(samples[count-1]),
		Avg:   time.Duration(sum / int64(count)),
	}

	// 3) Compute percentiles.
	// 3. 计算百分位。
	stats.P50 = time.Duration(samples[int(float64(count)*0.50)])
	stats.P90 = time.Duration(samples[int(float64(count)*0.90)])

	p99Idx := int(float64(count) * 0.99)
	if p99Idx >= count {
		p99Idx = count - 1
	}
	stats.P99 = time.Duration(samples[p99Idx])

	return stats
}
