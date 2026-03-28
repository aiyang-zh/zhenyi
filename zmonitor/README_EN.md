# zmonitor

**Structured Monitoring Data Module**: Defines component monitoring snapshot structure and aggregation manager, for solution internal collection and export.

## Module Positioning

- Unified `IMonitorable -> MonitorData` collection contract
- Manages monitoring snapshots from multiple components (`Manager`)
- Cooperates with `zmetrics` to export Prometheus monitoring snapshots

## Core Types

| Type | Description |
|------|-------------|
| `IMonitorable` | Collectible interface: GetMonitorData() |
| `MonitorData` | Snapshot structure: Metrics, Labels |
| `Manager` | Aggregates multiple IMonitorable, unified export |

## Typical Implementations

- `zactor.Actor` reports queue depth, processed count, etc.
- `zgate.Server` reports connection count, RTT statistics, etc.

## Minimal Usage

```go
mgr := zmonitor.NewManager()
mgr.Register("gate-1", gateServer)
all := mgr.GetAll()
_ = all
```

## Usage Suggestions

- Keep component `id` stable for external monitoring system correlation
- In high-frequency scenarios, prefer reporting aggregated metrics to avoid high-cardinality snapshot fields
- Recommend unified export via `zmetrics`, rather than each component separately exposing HTTP

## Related Documentation

- Monitoring overview: `../docs/MONITORING_OVERVIEW.md`
- Module API navigation: `../docs/MODULE_API.md`
