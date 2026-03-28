# 监控与可观测性（总览）

面向“先接入、再深入”的场景，本文给出最短可用路径。  
完整指标清单与实现细节请看 [MONITORING_METRICS.md](MONITORING_METRICS.md)。

## 1. 快速接入

```go
srv := zmetrics.Enable(ctx, ":9090")
defer srv.Shutdown(context.Background())
```

默认提供端点：

- `/metrics`
- `/healthz`
- `/readyz`

## 2. 建议接入顺序

1. 启动 `zmetrics.Enable(...)`
2. （可选）注册 `zmonitor.Manager` 以导出监控快照
3. 让 Prometheus 抓取 `/metrics`
4. 根据业务补充自定义指标到 `zmetrics.Global()`
5. （可选）需要**持续 CPU/内存剖析**时，再接入 [Grafana Pyroscope](https://grafana.com/docs/pyroscope/latest/)（见下文第 4 节；与指标互补，**不在 zhenyi 核心 `go.mod` 中强制依赖**）

## 3. 相关文档

- 监控指标明细：`MONITORING_METRICS.md`
- 全局变量与钩子：`GLOBALS_AND_HOOKS.md`

## 4. Pyroscope 持续剖析（可选）

`zhenyi` 默认提供 Prometheus 指标；若还要**火焰图 / 持续 Profile**（与 `pprof` 单次抓包互补），请使用仓库内封装包 **`zpyroscope`**（对 [grafana/pyroscope-go](https://github.com/grafana/pyroscope-go) 的薄封装，与 **`zmetrics` 解耦**）。在 `main` 中与 `zmetrics.Enable` **并列**初始化即可。

```go
import (
	"context"

	"github.com/aiyang-zh/zhenyi/zpyroscope"
)

stop, err := zpyroscope.StartWithContext(ctx, zpyroscope.Config{
	ApplicationName: "my-app",
	ServerAddress:   "http://127.0.0.1:4040",
})
if err == nil {
	defer stop()
}
```

业务侧**只需** `import` **`zpyroscope`**，不必再直接 `import github.com/grafana/pyroscope-go`（类型与常量由 `zpyroscope` 重导出）。

也可直接使用 `zpyroscope.Start(cfg)` 并 **`defer prof.Stop()`**（与上游 API 一致）。详见 **`zpyroscope/README.md`**。

**说明**：持续剖析有采样与网络开销，生产环境建议**灰度、按需开启**；Grafana Cloud 认证、标签、采样率等见 [官方文档](https://grafana.com/docs/pyroscope/latest/configure-client/language-sdks/go_push/) 与 [pyroscope-go](https://github.com/grafana/pyroscope-go)。
