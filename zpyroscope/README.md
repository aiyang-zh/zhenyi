# zpyroscope

**可选持续剖析**：对 [Grafana Pyroscope](https://grafana.com/docs/pyroscope/latest/) 客户端的薄封装，与 **`zmetrics`（Prometheus）解耦**——仅在你 `import` 本包时链接 `pyroscope-go`。

## 定位

- 与 `ztrace`、`zmonitor` 同属可观测性周边能力；**不负责** 指标暴露（仍用 `zmetrics.Enable`）。
- 默认填充 `Logger`、`ProfileTypes`（与上游 `pyroscope.DefaultProfileTypes` 一致）。

## API

- `Start(cfg Config) (*Profiler, error)` — 与上游 `pyroscope-go` 行为一致，**须** `defer prof.Stop()`。
- `StartWithContext(ctx, cfg) (stop func(), err)` — `ctx` 取消时自动 `Stop`；`stop` 可幂等调用，便于与进程退出对齐。

业务代码**只需** `import "github.com/aiyang-zh/zhenyi/zpyroscope"`；`Config`、`Profiler`、`ProfileCPU` 等类型与常量由本包重导出，无需再直接依赖 `github.com/grafana/pyroscope-go`。

## 使用示例

`StartWithContext` 会在 **`ctx` 取消**时停止剖析；长生命周期进程请使用 **`context.WithCancel`**（或随进程退出一并 cancel），勿对永不结束的 `context.Background()` 使用本函数（否则后台 goroutine会一直挂到进程结束）。

```go
import (
	"context"

	"github.com/aiyang-zh/zhenyi/zpyroscope"
)

ctx, cancel := context.WithCancel(context.Background())
defer cancel()
stop, err := zpyroscope.StartWithContext(ctx, zpyroscope.Config{
	ApplicationName: "my-app",
	ServerAddress:   "http://127.0.0.1:4040",
})
if err == nil {
	defer stop()
}
```

完整字段与 Grafana Cloud 认证见 [pyroscope-go](https://github.com/grafana/pyroscope-go) 与 [官方文档](https://grafana.com/docs/pyroscope/latest/configure-client/language-sdks/go_push/)。

## 相关文档

- 监控总览（含与 Prometheus 的配合）：`../docs/MONITORING_OVERVIEW.md`
- 模块 API 导航：`../docs/MODULE_API.md`
