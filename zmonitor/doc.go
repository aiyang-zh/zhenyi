// Package zmonitor provides unified runtime monitoring models and aggregation.
// Package zmonitor 提供统一的运行时监控数据模型与聚合管理能力。
//
// It outputs structured monitor data for Actor, Session, Gate, and runtime system.
// 该包用于为 Actor、Session、Gate 以及系统运行时输出结构化监控数据，
// 可直接用于 HTTP/JSON 暴露，或接入 Prometheus 等外部观测系统。
//
// Main capabilities:
// 主要能力：
//   - Use IMonitorable to define component monitoring data collection entry (GetMonitorData).
//   - 通过 IMonitorable 约定组件监控采集入口（GetMonitorData）
//   - Use Manager to register/unregister monitoring components and aggregate queries.
//   - 通过 Manager 完成监控组件注册、注销与聚合查询
//   - Use CollectSystemMonitor to collect Go runtime level system metrics.
//   - 通过 CollectSystemMonitor 采集 Go runtime 级别系统指标
//
// Concurrency notes:
// 并发说明：
//   - Manager is concurrency-safe and supports multi-goroutine registration and reads.
//   - Manager 是并发安全的，支持多 goroutine 注册和读取
//   - The system monitor sampling includes a cache mechanism; refresh interval can be adjusted via SetSystemMonitorCacheInterval.
//   - 系统监控采样包含缓存机制，可通过 SetSystemMonitorCacheInterval 调整刷新间隔
//
// Minimal example:
// 最小示例：
//
//	mgr := zmonitor.NewManager()
//	mgr.Register("actor_10001", actor)
//	all := mgr.GetAll()
//	_ = all
//
//	sys := zmonitor.CollectSystemMonitor()
//	_ = sys
package zmonitor
