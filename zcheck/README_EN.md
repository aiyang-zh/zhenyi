# zcheck

**Global Dependency Self-Check at Startup**: `zbus.DefaultBus`, `znats.DefaultNatsClient`, whether NATS is connected, optional `zmetrics` registry touch.

## Module Positioning

- Quickly expose global dependency missing before application startup
- Move "runtime failure" problems forward to startup
- Supports returning aggregated errors (`errors.Join`), convenient to see all gaps at once

## Minimal Usage

```go
import "github.com/aiyang-zh/zhenyi/zcheck"

if err := zcheck.Validate(zcheck.Config{
	RequireRemoteBus:      true,
	RequireNatsPool:       true,
	RequireNatsConnected:  true, // call after DefaultNatsClient.Connect(ctx)
	TouchMetricsRegistry:  true,
}); err != nil {
	log.Fatal(err)
}
```

Can disable `RequireRemoteBus`/`RequireNatsPool`/`RequireNatsConnected` for single-node Gate without cross-process.

## Core API

- `Validate(cfg Config) error`
- `ValidateOrPanic(cfg Config)`

## Related Documentation

- Global variables and startup checks: `../docs/GLOBALS_AND_HOOKS.md`
- Startup orchestration: `../zstartup/README.md`
