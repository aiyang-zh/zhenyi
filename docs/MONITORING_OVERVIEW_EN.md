# Monitoring and Observability (Overview)

Targeting the "first integrate, then dive deeper" scenario, this document provides the shortest usable path.  
For complete metrics list and implementation details, see [MONITORING_METRICS.md](MONITORING_METRICS.md).

## 1. Quick Integration

```go
srv := zmetrics.Enable(ctx, ":9090")
defer srv.Shutdown(context.Background())
```

Default endpoints:

- `/metrics`
- `/healthz`
- `/readyz`

## 2. Recommended Integration Order

1. Start `zmetrics.Enable(...)`
2. (Optional) Register `zmonitor.Manager` to export monitoring snapshots
3. Let Prometheus scrape `/metrics`
4. Add custom metrics to `zmetrics.Global()` based on business needs
5. (Optional) For **continuous CPU/memory profiling**, add [Grafana Pyroscope](https://grafana.com/docs/pyroscope/latest/) (see section 4 below; complements metrics; **not** a hard dependency in zhenyi’s core `go.mod`)

## 3. Related Documentation

- Monitoring metrics details: `MONITORING_METRICS.md`
- Global variables and hooks: `GLOBALS_AND_HOOKS.md`

## 4. Pyroscope continuous profiling (optional)

zhenyi exposes Prometheus metrics by default. For **continuous profiles / flame graphs**, use the **`zpyroscope`** package (thin wrapper over [grafana/pyroscope-go](https://github.com/grafana/pyroscope-go), **decoupled from `zmetrics`**). Initialize next to `zmetrics.Enable` in `main`.

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

You only need to **`import zpyroscope`** in application code; you do **not** need a direct `import` of `github.com/grafana/pyroscope-go` (types and constants are re-exported from `zpyroscope`).

Or use `zpyroscope.Start(cfg)` and **`defer prof.Stop()`** like upstream. See **`zpyroscope/README.md`**.

**Notes**: Sampling and upload have overhead; use **gradual rollout** in production. Auth, labels, sampling: [Go push SDK](https://grafana.com/docs/pyroscope/latest/configure-client/language-sdks/go_push/) and [pyroscope-go](https://github.com/grafana/pyroscope-go).
