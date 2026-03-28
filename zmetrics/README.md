# zmetrics

**可观测性指标模块**：提供方案预置指标、运行时指标、健康探针与 Prometheus 导出。

## 模块定位

- 统一管理 `zhenyi_*` 指标与导出格式
- 提供 `Enable` 一键启动监控 HTTP 服务
- 支持 runtime、handler、对象池、monitor 快照等指标汇聚

## 核心能力

- 指标注册：`Global().Counter/Gauge/Histogram`
- 服务启动：`Enable` / `EnableWithOptions`
- 导出端点：`/metrics`、`/healthz`、`/readyz`
- 运行时采集：Go runtime 指标定时更新
- 监控快照：`zmonitor.Manager` 聚合导出

## 最小用法

```go
srv := zmetrics.Enable(ctx, ":9090")
defer srv.Shutdown(context.Background())
```

## 使用建议

- 业务进程建议独立监控端口，避免与业务路由耦合
- 生产环境结合 Prometheus 定时抓取 `/metrics`
- 如需方案外业务组件指标，可统一注册到 `Global()` 保持同一出口

## 相关文档

- 监控与可观测性总览：`../docs/MONITORING_OVERVIEW.md`
- 模块 API 导航：`../docs/MODULE_API.md`
