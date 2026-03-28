# 监控指标明细

本文档给出 `zhenyi` 当前实现的监控指标与导出细节。

**说明**：持续剖析由可选包 `zpyroscope` 提供，与 Prometheus `/metrics` 互补，指标不在本文枚举；接入见 [MONITORING_OVERVIEW.md](MONITORING_OVERVIEW.md) 第 4 节。

## 1. HTTP 端点

通过 `zmetrics.Enable(ctx, addr)` 或 `zmetrics.EnableWithOptions` 启动独立 HTTP 服务（非阻塞）。

| 路径 | 方法 | 说明 |
|------|------|------|
| `/metrics` | GET | Prometheus text：全局 Registry + per-handler + 对象池 +（可选）`zmonitor` 快照 |
| `/healthz` | GET | 存活探针：聚合 `RegisterHealthCheck`；`SetDraining` 时返回 503 |
| `/readyz` | GET | 就绪探针：依据 `SetReady` / `SetDraining` / 启动中状态返回 JSON |

## 2. 指标命名与导出

- 命名前缀：方案指标为 `zhenyi_*`，Go 运行时为 `go_*`
- 直方图输出：`_bucket` / `_sum` / `_count`
- 默认延迟桶（ms）：`1, 5, 10, 25, 50, 100, 250, 500, 1000, 5000`

## 3. 方案预注册指标（`zhenyi_*`）

### 3.1 网络 / 连接

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

路由指标：

- `zhenyi_gate_route_gate_self_total`
- `zhenyi_gate_route_local_total`
- `zhenyi_gate_route_remote_total`
- `zhenyi_gate_route_no_route_total`
- `zhenyi_gate_route_remote_fail_total`
- `zhenyi_gate_route_remote_candidates`
- `zhenyi_gate_route_remote_try_total`
- `zhenyi_gate_route_remote_fallback_total`

### 3.5 消息池 / NATS

- `zhenyi_msgpool_double_release_total`
- `zhenyi_nats_publish_total`
- `zhenyi_nats_publish_errors_total`
- `zhenyi_nats_request_total`
- `zhenyi_nats_request_errors_total`
- `zhenyi_nats_request_latency_ms`

### 3.6 对象池（`zpool` + `zpoolobs`）

带 `pool` 标签的系列：

- `zhenyi_zpool_get_total`
- `zhenyi_zpool_put_total`
- `zhenyi_zpool_new_total`
- `zhenyi_zpool_put_nil_total`
- `zhenyi_zpool_outstanding`

## 4. Go 运行时指标（`go_*`）

由 `StartRuntimeCollector` 周期性更新（默认启用）：

- 内存：`go_memstats_*`
- GC：`go_gc_*`
- 调度：`go_goroutines`、`go_threads`、`go_gomaxprocs`

## 5. Per-handler 指标（带标签）

由 `zmetrics.GetHandlerMetric` 注册，标签包含 `handler/actor_id/actor_type`：

- `zhenyi_handler_total`
- `zhenyi_handler_slow_total`
- `zhenyi_handler_latency_ms`

## 6. zmonitor 快照导出

`zmetrics.RegisterMonitorManager` 或 `(*Server).SetMonitorManager` 注入后，会在 `/metrics` 追加：

- `zhenyi_monitor_registry_components`
- `zhenyi_monitor_snapshot`

## 7. 抓取示例

```yaml
scrape_configs:
  - job_name: 'zhenyi'
    scrape_interval: 15s
    static_configs:
      - targets: ['<host>:9090']
```

## 8. 注意事项

- 若覆盖 `zlog` panic hook，请用 `AppendPanicHook`，避免覆盖 metrics hook
- 大规模场景注意标签基数（尤其 per-handler 与 monitor snapshot）
- 文档与代码不一致时，以 `zmetrics/*.go` 与 `zgate/net_metrics.go` 实现为准
