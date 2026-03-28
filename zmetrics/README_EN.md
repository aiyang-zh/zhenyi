# zmetrics

**Observability Metrics Module**: Provides solution preset metrics, runtime metrics, health probes, and Prometheus export.

## Module Positioning

- Unified management of `zhenyi_*` metrics and export format
- Provides `Enable` one-click startup of monitoring HTTP service
- Supports runtime, handler, object pool, monitor snapshot aggregation

## Core Capabilities

- Metrics registration: `Global().Counter/Gauge/Histogram`
- Service startup: `Enable` / `EnableWithOptions`
- Export endpoints: `/metrics`, `/healthz`, `/readyz`
- Runtime collection: Go runtime metrics periodic updates
- Monitor snapshot: `zmonitor.Manager` aggregation export

## Minimal Usage

```go
srv := zmetrics.Enable(ctx, ":9090")
defer srv.Shutdown(context.Background())
```

## Usage Suggestions

- Business processes recommend independent monitoring port to avoid coupling with business routes
- Production environment combines with Prometheus periodic scraping of `/metrics`
- If metrics from business components outside the solution are needed, can uniformly register to `Global()` to keep same export

## Related Documentation

- Monitoring and observability overview: `../docs/MONITORING_OVERVIEW.md`
- Module API navigation: `../docs/MODULE_API.md`
