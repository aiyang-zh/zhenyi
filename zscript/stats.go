package zscript

import (
	"sync/atomic"
	"time"
)

// EngineStats contains engine statistics.
// EngineStats 引擎统计信息。
type EngineStats struct {
	// EngineType is engine type.
	// EngineType 引擎类型。
	EngineType string

	// CallCount is total invocation count.
	// CallCount 总调用次数。
	CallCount int64

	// ErrorCount is total failed invocation count.
	// ErrorCount 错误次数。
	ErrorCount int64

	// TimeoutCount is total timeout count.
	// TimeoutCount 超时次数。
	TimeoutCount int64

	// TotalDuration is cumulative duration in nanoseconds.
	// TotalDuration 总耗时（纳秒）。
	TotalDuration int64

	// AvgDuration is average duration in nanoseconds.
	// AvgDuration 平均耗时（纳秒）。
	AvgDuration int64

	// MaxDuration is maximum duration in nanoseconds.
	// MaxDuration 最大耗时（纳秒）。
	MaxDuration int64

	// MinDuration is minimum duration in nanoseconds.
	// MinDuration 最小耗时（纳秒）。
	MinDuration int64

	// ReloadCount is total hot-reload count.
	// ReloadCount 热重载次数。
	ReloadCount int64

	// LastReloadTime is timestamp of last reload.
	// LastReloadTime 最后一次重载时间。
	LastReloadTime time.Time

	// ScriptFiles contains loaded script file snapshots.
	// ScriptFiles 已加载的脚本文件列表。
	ScriptFiles []ScriptFileInfo

	// Metadata stores engine-specific extension statistics.
	// Metadata 扩展元数据（引擎特定统计信息）。
	Metadata map[string]interface{}
}

// ScriptFileInfo contains script file metadata.
// ScriptFileInfo 脚本文件信息。
type ScriptFileInfo struct {
	// Path is script file path.
	// Path 文件路径。
	Path string

	// Size is file size in bytes.
	// Size 文件大小（字节）。
	Size int64

	// LoadTime is script load time.
	// LoadTime 加载时间。
	LoadTime time.Time

	// CallCount is per-file invocation count.
	// CallCount 调用次数。
	CallCount int64

	// ErrorCount is per-file error count.
	// ErrorCount 错误次数。
	ErrorCount int64

	// LastCallTime is the timestamp of last call.
	// LastCallTime 最后一次调用时间。
	LastCallTime time.Time
}

// StatsCollector is a thread-safe stats collector.
// StatsCollector 统计收集器（线程安全）。
type StatsCollector struct {
	engineType     string
	callCount      atomic.Int64
	errorCount     atomic.Int64
	timeoutCount   atomic.Int64
	totalDuration  atomic.Int64
	maxDuration    atomic.Int64
	minDuration    atomic.Int64
	reloadCount    atomic.Int64
	lastReloadTime atomic.Value // time.Time
}

// NewStatsCollector creates a stats collector.
// NewStatsCollector 创建统计收集器。
func NewStatsCollector(engineType string) *StatsCollector {
	c := &StatsCollector{
		engineType: engineType,
	}
	c.minDuration.Store(int64(^uint64(0) >> 1)) // Set to int64 max / 设置为 int64 最大值
	c.lastReloadTime.Store(time.Time{})
	return c
}

// RecordCall records one invocation.
// RecordCall 记录调用。
func (c *StatsCollector) RecordCall(duration time.Duration, err error) {
	c.callCount.Add(1)
	durationNs := duration.Nanoseconds()
	c.totalDuration.Add(durationNs)

	if err != nil {
		c.errorCount.Add(1)
	}

	// Update max duration.
	// 更新最大耗时。
	for {
		old := c.maxDuration.Load()
		if durationNs <= old {
			break
		}
		if c.maxDuration.CompareAndSwap(old, durationNs) {
			break
		}
	}

	// Update min duration.
	// 更新最小耗时。
	for {
		old := c.minDuration.Load()
		if durationNs >= old {
			break
		}
		if c.minDuration.CompareAndSwap(old, durationNs) {
			break
		}
	}
}

// RecordTimeout records timeout count.
// RecordTimeout 记录超时。
func (c *StatsCollector) RecordTimeout() {
	c.timeoutCount.Add(1)
}

// RecordReload records reload count and time.
// RecordReload 记录重载。
func (c *StatsCollector) RecordReload() {
	c.reloadCount.Add(1)
	c.lastReloadTime.Store(time.Now())
}

// GetStats returns current stats snapshot.
// GetStats 获取统计信息。
func (c *StatsCollector) GetStats() *EngineStats {
	callCount := c.callCount.Load()
	totalDuration := c.totalDuration.Load()

	var avgDuration int64
	if callCount > 0 {
		avgDuration = totalDuration / callCount
	}

	minDuration := c.minDuration.Load()
	if minDuration == int64(^uint64(0)>>1) {
		minDuration = 0
	}

	lastReloadTime := c.lastReloadTime.Load().(time.Time)

	return &EngineStats{
		EngineType:     c.engineType,
		CallCount:      callCount,
		ErrorCount:     c.errorCount.Load(),
		TimeoutCount:   c.timeoutCount.Load(),
		TotalDuration:  totalDuration,
		AvgDuration:    avgDuration,
		MaxDuration:    c.maxDuration.Load(),
		MinDuration:    minDuration,
		ReloadCount:    c.reloadCount.Load(),
		LastReloadTime: lastReloadTime,
	}
}

// Reset clears all stats.
// Reset 重置统计信息。
func (c *StatsCollector) Reset() {
	c.callCount.Store(0)
	c.errorCount.Store(0)
	c.timeoutCount.Store(0)
	c.totalDuration.Store(0)
	c.maxDuration.Store(0)
	c.minDuration.Store(int64(^uint64(0) >> 1))
	c.reloadCount.Store(0)
	c.lastReloadTime.Store(time.Time{})
}
