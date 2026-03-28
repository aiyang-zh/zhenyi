# zpyroscope

**Optional continuous profiling**: thin wrapper around [Grafana Pyroscope](https://grafana.com/docs/pyroscope/latest/) (`github.com/grafana/pyroscope-go`), **decoupled from `zmetrics` (Prometheus)**—Pyroscope is linked only when you import this package.

## Role

- Same observability family as `ztrace` / `zmonitor`; does **not** expose metrics (use `zmetrics.Enable` for that).
- Fills in default `Logger` and `ProfileTypes` when unset (same defaults as upstream `pyroscope`).

## API

- `Start(cfg Config) (*Profiler, error)` — same contract as upstream `pyroscope-go`; **must** `defer prof.Stop()`.
- `StartWithContext(ctx, cfg) (stop func(), err)` — stops when `ctx` is done; `stop` is idempotent for explicit shutdown.

Application code only needs `import "github.com/aiyang-zh/zhenyi/zpyroscope"`; `Config`, `Profiler`, `ProfileCPU`, and related types/constants are re-exported here—no direct dependency on `github.com/grafana/pyroscope-go` in your modules.

## Example

```go
import (
	"context"

	"github.com/aiyang-zh/zhenyi/zpyroscope"
)

ctx := context.Background()
stop, err := zpyroscope.StartWithContext(ctx, zpyroscope.Config{
	ApplicationName: "my-app",
	ServerAddress:   "http://127.0.0.1:4040",
})
if err == nil {
	defer stop()
}
```

See [pyroscope-go](https://github.com/grafana/pyroscope-go) and the [Go push SDK](https://grafana.com/docs/pyroscope/latest/configure-client/language-sdks/go_push/) for full options (Grafana Cloud auth, tags, etc.).

## Related docs

- Monitoring overview: `../docs/MONITORING_OVERVIEW_EN.md`
- Module API index: `../docs/MODULE_API_EN.md`
