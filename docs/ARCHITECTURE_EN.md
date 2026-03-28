# zhenyi Architecture

This document describes the core layers, key data flows, and extension points of `zhenyi` as a "real-time application solution".

## 1. Layered Architecture

- **Solution Layer (zhenyi)**: Takes Actor runtime as the core engine, overlaid with gateway, routing, monitoring, script, and message bus adapters
- **Base Capability Layer (zhenyi-base)**: Network protocols, connection management, utility libraries, common interfaces

`zhenyi` collaborates with `zhenyi-base` through interfaces, providing deployable real-time application engineering capabilities while maintaining high-performance hot paths.

## 2. Core Components

- `zgate`: Unified entry point, handling TCP/WS/KCP long connections
- `zactor`: Single Actor with single mailbox, handles messages and Tick
- `zmsg` / `zmodel`: Message carrier, object pool, and model definitions
- `zroute` + `zdiscovery`: Local/remote routing and discovery
- `zmetrics` + `zmonitor` + `ztrace`: Metrics, monitoring snapshots, and tracing; optional `zpyroscope` continuous profiling (decoupled from Prometheus metrics)
- `znats` / `zbus`: Cross-process message bus

## 3. Typical Message Flow (Long Connection)

1. Client message enters `zgate`
2. `zgate` converts wire protocol message to `ActorCmd`
3. Message enters target Actor mailbox
4. Handler processes and generates response
5. `zgate` writes response back to client based on session mapping

## 4. Routing Order (Gate)

Default routing order in `zgate`:

1. Gate's own handler
2. Local process LocalRouter
3. Remote candidates (discover + bus)
4. No-route fallback (OnNoRoute)

This order ensures local-first, remote-fallback, and observable failures.

## 5. Extension Points

- Routing strategy: `SetRemoteRouteStrategy`
- No-route handling: `OnNoRoute`
- Trace injection: `SetTraceHook`
- Gateway TLS/GM-TLS: `SetTLSConfig` / `SetStandardTLS` / `SetGMTLS`
- Script engines: `zscript` + `zjs/zlua/zstarlark/ztengo`

## 6. Observability Paths

- Metrics export: `zmetrics.Enable(...)->/metrics`
- Health probes: `/healthz`, `/readyz`
- Gate connection and routing metrics: bridged from metrics injected by `zgate`
- Continuous profiling (optional): `zpyroscope` (not Prometheus metrics; complements `/metrics`; see [MONITORING_OVERVIEW.md](MONITORING_OVERVIEW_EN.md) section 4)

See [MONITORING_METRICS.md](MONITORING_METRICS.md) for detailed metrics.

## 7. Codec and Message Adaptation

- Business messages are ultimately carried in `zmsg.Message.Data` (`[]byte`).
- Codec adaptation is done through implementing `ziface.IMessage` adapters (the framework calls `MarshalToVT` on the send path and writes the result to `Message.Data`).
- Recommended reference: [`docs/CODEC_ADAPTERS.md`](CODEC_ADAPTERS.md).
