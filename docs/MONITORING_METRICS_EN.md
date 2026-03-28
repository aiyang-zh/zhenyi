# Monitoring Metrics Details

This document provides the monitoring metrics and export details currently implemented in `zhenyi`.

**Note:** Continuous profiling is provided by the optional `zpyroscope` package; it complements Prometheus `/metrics` and is not listed here. See [MONITORING_OVERVIEW.md](MONITORING_OVERVIEW_EN.md) section 4.

## 1. HTTP Endpoints

Start an independent HTTP service (non-blocking) via `zmetrics.Enable(ctx, addr)` or `zmetrics.EnableWithOptions`.

| Path | Method | Description |
|------|--------|-------------|
| `/metrics` | GET | Prometheus text: global Registry + per-handler + object pool + (optional) `zmonitor` snapshots |
| `/healthz` | GET | Liveness probe: aggregates `RegisterHealthCheck`; returns 503 when `SetDraining` |
| `/readyz` | GET | Readiness probe: returns JSON based on `SetReady` / `SetDraining` / starting state |

## 2. Metrics Naming and Export

- Prefix: Framework metrics are `zhenyi_*`, Go runtime is `go_*`
- Histogram output: `_bucket` / `_sum` / `_count`
- Default latency buckets (ms): `1, 5, 10, 25, 50, 100, 250, 500, 1000, 5000`

## 3. Framework Pre-registered Metrics (`zhenyi_*`)

### 3.1 Network / Connection

- `zhenyi_conn_active`
- `zhenyi_conn_accepted_total`
- `zhenyi_conn_rejected_total`
- `zhenyi_bytes_recv_total`
- `zhenyi_bytes_sent_total`
- `zhenyi_conn_errors_total`
- `zhenyi_conn_heartbeat_timeout_total`

### 3.2 Actor

- `zhenyi_actor_msg_recv_total`
- `zhenyi_actor_msg_handled_total`
- `zhenyi_actor_msg_dropped_total`
- `zhenyi_actor_tick_total`
- `zhenyi_actor_tick_latency_ms`
- `zhenyi_actor_panic_total`
- `zhenyi_actor_restarts_total`
- `zhenyi_actor_queue_depth`
- `zhenyi_actor_msg_latency_ms`
- `zhenyi_actor_workerpool_running`
- `zhenyi_actor_workerpool_capacity`
- `zhenyi_actor_blocked_total`

### 3.3 RPC

- `zhenyi_rpc_sent_total`
- `zhenyi_rpc_success_total`
- `zhenyi_rpc_timeout_total`
- `zhenyi_rpc_circuit_breaker_tripped_total`
- `zhenyi_rpc_latency_ms`

### 3.4 Gate

- `zhenyi_gate_online_users`
- `zhenyi_gate_recv_qps`
- `zhenyi_gate_sent_qps`
- `zhenyi_gate_rtt_ms`

Routing metrics:

- `zhenyi_gate_route_gate_self_total`
- `zhenyi_gate_route_local_total`
- `zhenyi_gate_route_remote_total`
- `zhenyi_gate_route_no_route_total`
- `zhenyi_gate_route_remote_fail_total`
- `zhenyi_gate_route_remote_candidates`
- `zhenyi_gate_route_remote_try_total`
- `zhenyi_gate_route_remote_fallback_total`

### 3.5 Message Pool / NATS

- `zhenyi_msgpool_double_release_total`
- `zhenyi_nats_publish_total`
- `zhenyi_nats_publish_errors_total`
- `zhenyi_nats_request_total`
- `zhenyi_nats_request_errors_total`
- `zhenyi_nats_request_latency_ms`

### 3.6 Object Pool (`zpool` + `zpoolobs`)

Series with `pool` tag:

- `zhenyi_zpool_get_total`
- `zhenyi_zpool_put_total`
- `zhenyi_zpool_new_total`
- `zhenyi_zpool_put_nil_total`
- `zhenyi_zpool_outstanding`

## 4. Go Runtime Metrics (`go_*`)

Periodically updated by `StartRuntimeCollector` (enabled by default):

- Memory: `go_memstats_*`
- GC: `go_gc_*`
- Scheduling: `go_goroutines`, `go_threads`, `go_gomaxprocs`

## 5. Per-handler Metrics (with tags)

Registered via `zmetrics.GetHandlerMetric`, tags include `handler/actor_id/actor_type`:

- `zhenyi_handler_total`
- `zhenyi_handler_slow_total`
- `zhenyi_handler_latency_ms`

## 6. zmonitor Snapshot Export

After injecting via `zmetrics.RegisterMonitorManager` or `(*Server).SetMonitorManager`, appended to `/metrics`:

- `zhenyi_monitor_registry_components`
- `zhenyi_monitor_snapshot`

## 7. Scrape Example

```yaml
scrape_configs:
  - job_name: 'zhenyi'
    scrape_interval: 15s
    static_configs:
      - targets: ['<host>:9090']
```

## 8. Notes

- If overriding `zlog` panic hook, use `AppendPanicHook` to avoid overwriting metrics hook
- In large-scale scenarios, watch label cardinality (especially per-handler and monitor snapshots)
- When documentation differs from code, implementation in `zmetrics/*.go` and `zgate/net_metrics.go` prevails
