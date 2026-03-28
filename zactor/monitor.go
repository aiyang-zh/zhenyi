package zactor

import (
	"fmt"
	"github.com/aiyang-zh/zhenyi/zmonitor"
	"sync/atomic"
	"time"
)

// GetMonitorData returns actor monitoring snapshot for zmonitor.IMonitorable.
// GetMonitorData 返回用于 zmonitor.IMonitorable 的 Actor 监控快照。
func (a *Actor) GetMonitorData() zmonitor.MonitorData {
	processed, avgLatencyMs, qps, slowCount, errorCount := a.stats.GetSnapshot()

	status := "running"
	mailCount := atomic.LoadInt64(&a.mailCount)
	if mailCount > 1000 {
		status = "busy"
	} else if mailCount == 0 {
		status = "idle"
	}

	return zmonitor.MonitorData{
		Type:      "actor",
		ID:        fmt.Sprintf("actor_%d", a.GetActorId()),
		Name:      a.GetTopic(),
		Status:    status,
		Timestamp: time.Now().UnixMilli(),
		Metrics: map[string]interface{}{
			"actorId":      a.GetActorId(),
			"actorType":    a.GetTopic(),
			"mailCount":    mailCount,
			"processedMsg": processed,
			"avgLatencyMs": float64(avgLatencyMs),
			"qps":          qps,
			"slowCount":    slowCount,
			"errorCount":   errorCount,
		},
		Tags: map[string]string{
			"type":  "actor",
			"topic": a.GetTopic(),
		},
	}
}

// RecordError records one actor-level error for monitoring counters.
// RecordError 记录一次 Actor 级别错误（供外部计数统计）。
func (a *Actor) RecordError() {
	a.stats.RecordError()
}
