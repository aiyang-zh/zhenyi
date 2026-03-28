# zmonitor

**结构化监控数据模块**：定义组件监控快照结构与聚合管理器，供方案内部采集与导出。

## 模块定位

- 统一 `IMonitorable -> MonitorData` 采集契约
- 管理多个组件的监控快照（`Manager`）
- 与 `zmetrics` 协作导出 Prometheus 监控快照

## 核心类型

| 类型 | 说明 |
|------|------|
| `IMonitorable` | 可被采集的接口：GetMonitorData() |
| `MonitorData` | 快照结构：Metrics、Labels |
| `Manager` | 聚合多个 IMonitorable，统一导出 |

## 典型实现

- `zactor.Actor` 上报队列长度、处理数等
- `zgate.Server` 上报连接数、RTT 统计等

## 最小用法

```go
mgr := zmonitor.NewManager()
mgr.Register("gate-1", gateServer)
all := mgr.GetAll()
_ = all
```

## 使用建议

- 组件 `id` 保持稳定，便于外部监控系统关联
- 高频场景优先上报聚合指标，避免高基数快照字段
- 建议通过 `zmetrics` 统一导出，而非每个组件单独暴露 HTTP

## 相关文档

- 监控总览：`../docs/MONITORING_OVERVIEW.md`
- 模块 API 导航：`../docs/MODULE_API.md`
