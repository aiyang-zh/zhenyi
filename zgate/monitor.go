package zgate

import (
	"fmt"
	"runtime"
	"time"

	"github.com/aiyang-zh/zhenyi/zmonitor"
)

// GetMonitorData implements the IMonitorable interface.
// GetMonitorData 实现 IMonitorable 接口。
func (s *Server) GetMonitorData() zmonitor.MonitorData {
	// Read realtime counters.
	// 获取实时统计。
	recvCount := s.metrics.recvCount.Load()
	sentCount := s.metrics.sentCount.Load()
	recvCountTotal := s.metrics.recvCountTotal.Load()
	sentCountTotal := s.metrics.sentCountTotal.Load()
	onlineCount := int(s.metrics.OnlineUsers.Load())

	// currentQPS: recvCount 是最近一个 tick 周期的增量（被定时 Swap(0) 重置），直接作为瞬时 QPS
	currentQPS := float64(recvCount)

	// Global QPS.
	// 全局 QPS。
	firstPacketNano := s.metrics.FirstPacketTime.Load()
	globalQPS := 0.0
	if firstPacketNano > 0 {
		globalElapsed := time.Since(time.Unix(0, firstPacketNano)).Seconds()
		if globalElapsed > 0 {
			globalQPS = float64(recvCountTotal) / globalElapsed
		}
	}

	// RTT stats (lock-free tracker).
	// RTT 统计（使用无锁追踪器）。
	samples := s.metrics.RTTTracker.GetAndResetSamples()
	stats := CalculateStats(samples)
	avgRTTMs := float64(stats.Avg.Microseconds()) / 1000.0
	p99RTTMs := float64(stats.P99.Microseconds()) / 1000.0

	// System stats from cache to avoid frequent ReadMemStats STW.
	// 系统统计（使用缓存，避免每次调用 ReadMemStats 引发 STW）。
	memAllocMB := float64(s.metrics.MemAllocBytes.Load()) / 1024 / 1024
	goroutineNum := runtime.NumGoroutine()
	gcCount := s.metrics.MemNumGC.Load()

	type sessionAgg interface {
		AggregateChannelSessionStats() (channelsWithStats int, sumSendCount, sumRecvCount, sumSendBytes, sumRecvBytes int64)
	}

	status := "running"
	if onlineCount == 0 {
		status = "idle"
	} else if onlineCount > 8000 {
		status = "busy"
	}

	metrics := map[string]interface{}{
		"onlineCount":   onlineCount,
		"totalSessions": recvCountTotal,
		"currentQPS":    currentQPS,
		"globalQPS":     globalQPS,
		"avgRttMs":      avgRTTMs,
		"p99RttMs":      p99RTTMs,
		"memAllocMB":    memAllocMB,
		"goroutineNum":  goroutineNum,
		"gcCount":       gcCount,
		"recvCount":     recvCount,
		"sentCount":     sentCount,
		"recvTotal":     recvCountTotal,
		"sentTotal":     sentCountTotal,
	}
	if s.server != nil {
		if agg, ok := s.server.(sessionAgg); ok {
			n, ssc, src, ssb, srb := agg.AggregateChannelSessionStats()
			metrics["session_channels_with_stats"] = n
			metrics["session_sum_send_count"] = ssc
			metrics["session_sum_recv_count"] = src
			metrics["session_sum_send_bytes"] = ssb
			metrics["session_sum_recv_bytes"] = srb
		}
	}

	return zmonitor.MonitorData{
		Type:      "gate",
		ID:        fmt.Sprintf("gate_%d", s.GetActorId()),
		Name:      fmt.Sprintf("GateServer-%d", s.GetActorId()),
		Status:    status,
		Timestamp: time.Now().UnixMilli(),
		Metrics:   metrics,
		Tags: map[string]string{
			"type": "gate",
		},
	}
}
