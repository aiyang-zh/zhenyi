package zmetrics

// Pre-register framework-level metrics for direct business usage.
// 预注册框架级指标，业务层直接使用即可。
//
// Note: ActorPanicCount is registered by zlog.AppendPanicHook in zmetrics.Enable/EnableWithOptions,
// 注意：ActorPanicCount 由 zlog.AppendPanicHook 在 zmetrics.Enable/EnableWithOptions 时注册，
// 与 WorkerPool 的 WithPanicHandler 内显式 Inc 共同覆盖 actor 相关 panic 恢复路径。

var (
	// ---- Network 层 ----
	ConnActive           = Global().Gauge("zhenyi_conn_active", "Current active connections")
	ConnAccepted         = Global().Counter("zhenyi_conn_accepted_total", "Total accepted connections")
	ConnRejected         = Global().Counter("zhenyi_conn_rejected_total", "Total rejected connections (limit)")
	BytesRecv            = Global().Counter("zhenyi_bytes_recv_total", "Total bytes received")
	BytesSent            = Global().Counter("zhenyi_bytes_sent_total", "Total bytes sent")
	ConnErrors           = Global().Counter("zhenyi_conn_errors_total", "Total connection errors (read/write/parse)")
	ConnHeartbeatTimeout = Global().Counter("zhenyi_conn_heartbeat_timeout_total", "Total connections closed by heartbeat timeout")

	// ---- Actor 层 ----
	ActorMsgRecv      = Global().Counter("zhenyi_actor_msg_recv_total", "Total actor messages received (Push)")
	ActorMsgHandled   = Global().Counter("zhenyi_actor_msg_handled_total", "Total actor messages handled")
	ActorMsgDropped   = Global().Counter("zhenyi_actor_msg_dropped_total", "Total actor messages dropped")
	ActorTickCount    = Global().Counter("zhenyi_actor_tick_total", "Total actor tick invocations")
	ActorTickLatency  = Global().Histogram("zhenyi_actor_tick_latency_ms", "Actor tick handling latency in ms", DefaultLatencyBounds)
	ActorPanicCount   = Global().Counter("zhenyi_actor_panic_total", "Total actor panic recoveries")
	ActorRestarts     = Global().Counter("zhenyi_actor_restarts_total", "Total actor restarts by supervisor")
	ActorQueueDepth   = Global().Gauge("zhenyi_actor_queue_depth", "Current actor mailbox queue depth (sampled)")
	ActorMsgLatency   = Global().Histogram("zhenyi_actor_msg_latency_ms", "Actor message handling latency in ms", DefaultLatencyBounds)
	ActorPoolRunning  = Global().Gauge("zhenyi_actor_workerpool_running", "Current running workers in actor pool")
	ActorPoolCapacity = Global().Gauge("zhenyi_actor_workerpool_capacity", "Actor worker pool capacity")
	ActorBlockedCount = Global().Counter("zhenyi_actor_blocked_total", "Total blocked handler detections by watchdog")

	// ---- RPC 层 ----
	RPCSent      = Global().Counter("zhenyi_rpc_sent_total", "Total RPC requests sent")
	RPCSuccess   = Global().Counter("zhenyi_rpc_success_total", "Total RPC successes")
	RPCTimeout   = Global().Counter("zhenyi_rpc_timeout_total", "Total RPC timeouts")
	RPCCBTripped = Global().Counter("zhenyi_rpc_circuit_breaker_tripped_total", "Total circuit breaker trips")
	RPCLatency   = Global().Histogram("zhenyi_rpc_latency_ms", "RPC round-trip latency in ms", DefaultLatencyBounds)

	// ---- GateServer 层 ----
	GateOnlineUsers = Global().Gauge("zhenyi_gate_online_users", "Current online user count")
	GateRecvQPS     = Global().Gauge("zhenyi_gate_recv_qps", "Gate receive QPS (sampled)")
	GateSentQPS     = Global().Gauge("zhenyi_gate_sent_qps", "Gate send QPS (sampled)")
	GateRTTAvg      = Global().Histogram("zhenyi_gate_rtt_ms", "Gate client RTT in ms", DefaultLatencyBounds)

	// ---- Gate 路由（P1-4 观测） ----
	// Route-destination counters for hit-rate analysis.
	// 按去向计数，便于算命中率：gate_self=Gate 自身 handler，local=本进程其他 Actor，remote=跨进程发出，no_route=无路由回错，remote_fail=远程发出失败
	GateRouteGateSelf         = Global().Counter("zhenyi_gate_route_gate_self_total", "Gate route: handled by Gate itself")
	GateRouteLocal            = Global().Counter("zhenyi_gate_route_local_total", "Gate route: routed to local actor")
	GateRouteRemote           = Global().Counter("zhenyi_gate_route_remote_total", "Gate route: routed to remote actor (broadcast ok)")
	GateRouteNoRoute          = Global().Counter("zhenyi_gate_route_no_route_total", "Gate route: no route, sent error reply")
	GateRouteRemoteFail       = Global().Counter("zhenyi_gate_route_remote_fail_total", "Gate route: remote broadcast failed")
	GateRouteRemoteCandidates = Global().Gauge("zhenyi_gate_route_remote_candidates", "Gate route: remote candidate count (last sample)")
	GateRouteRemoteTry        = Global().Counter("zhenyi_gate_route_remote_try_total", "Gate route: total remote broadcast attempts (including retries)")
	GateRouteRemoteFallback   = Global().Counter("zhenyi_gate_route_remote_fallback_total", "Gate route: retry due to first remote candidate failure")

	// ---- MsgPool 层 ----
	MsgPoolDoubleRelease = Global().Counter("zhenyi_msgpool_double_release_total", "Total double-release detections")

	// ---- NATS 层 ----
	NatsPublishTotal   = Global().Counter("zhenyi_nats_publish_total", "Total NATS publish operations")
	NatsPublishErrors  = Global().Counter("zhenyi_nats_publish_errors_total", "Total NATS publish errors")
	NatsRequestTotal   = Global().Counter("zhenyi_nats_request_total", "Total NATS request operations")
	NatsRequestErrors  = Global().Counter("zhenyi_nats_request_errors_total", "Total NATS request errors")
	NatsRequestLatency = Global().Histogram("zhenyi_nats_request_latency_ms", "NATS request round-trip latency in ms", DefaultLatencyBounds)
)
