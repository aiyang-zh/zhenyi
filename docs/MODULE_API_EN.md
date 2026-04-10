# Module API Navigation

This document provides module-level API entry index to help quickly locate "where to use what".

## 1. Startup & Orchestration

- `zstartup`: Application startup orchestration, Actor factory registration
  - Common: `NewApp`, `RegisterActorFactory`, `Run`
  - Lifecycle: `App` uses a unified shutdown path that calls `IGroup.Close(ctx)`
- `zcheck`: Pre-startup dependency self-check (bus/nats/metrics, etc.)

## 2. Gateway & Network Entry

- `zgate`: Unified gateway (TCP/WS/KCP), session mapping, send/receive pipeline
  - Common: `NewServer`, `Init`, `RunServer`, `SetHTTPAddr`, `SetTLSConfig`
  - Performance/model: `SetReactorMode` (optional reactor read path for TCP without TLS), `SetSharedSendWorkerMode` (shared send worker, default off), `WithNetServerHook` (callback after underlying net server is created; e.g. `SetHeartbeatTimeout`)
- `zhttp`: Optional HTTP service capability

## 3. Actor & Messaging

- `zactor`: Actor lifecycle, message handling, Tick, RPC, Group
- `zmodel`: `ActorConfig`, `ActorCmd`, framework tuning configuration
- `zmsg`: Message struct and object pool (`GetMessage/Retain/Release`)
- `ziface`: Core interface definitions (cross-module contracts)
  - `IGroup` lifecycle: `Run(ctx)` and `Close(ctx)` (single close entry)

## 4. Routing, Discovery, Bus

- `zroute`: Local/remote routing strategies
- `zdiscovery`: Service discovery
- `zbus`: Bus abstraction
- `znats`: NATS implementation and default bus access

## 5. Observability

- `zmetrics`: Prometheus metrics, health probes
- `zmonitor`: Structured monitoring snapshots and manager
- `ztrace`: Trace pass-through and parsing
- `zpyroscope`: Optional Pyroscope continuous profiling (decoupled from `zmetrics`; see `zpyroscope/README.md`)

## 6. Scripts & Extensions

- `zscript`: Unified script abstraction and context
- `zjs` / `zlua` / `zstarlark` / `ztengo`: Specific script engines
- `zcodec`: Codec helpers

## 7. Other Base Capabilities

- `zaoi`: AOI capability
- `zstream`: Lightweight business Actor Server wrapper (`zactor.Actor` wrapper)
- `zconfig`: Configuration management

## 8. Recommended Deep Reading Order

1. `README.md`
2. `BEGINNER_GUIDE.md`
3. `ARCHITECTURE.md`
4. Target module `README.md`
5. `MONITORING_OVERVIEW.md` and `GLOBALS_AND_HOOKS.md`
