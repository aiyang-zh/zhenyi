# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

## [0.1.1] - 2026-04-02

### Added

- **zgate**: **`SetReactorMode(bool)`** — when **TCP**, **no transport TLS/GM-TLS**, and the underlying server is **`*ztcp.Server`**, run **`ztcp.ServerReactor`** (Linux epoll / macOS kqueue); otherwise keep the existing **`Server(ctx)`** path.
- **zgate**: **`SetSharedSendWorkerMode(bool)`** — toggle **shared send workers** on the underlying long-lived **`IServer`** (**ztcp / zws / zkcp** via `znet.BaseServer`); **default off** (preserves historical behavior).
- **Examples**: **`im_single_demo`** / **`im_multi_demo`** add **`--reactor`** and **`--sharedSendWorker`** and wire the Gate APIs above.
- **`examples/mmo_web_demo`**: minimal combat loop (attack, HP, death, delayed respawn, cooldown, range checks).
- **`examples/mmo_web_demo`**: **`world_snapshot`** / **`combat_event`** broadcast filtered by **`zaoi`** (**`WorldManager` + `Zone` + `StaticAoi`**, nine-grid + view distance).
- **zactor**: **`SendToClient`** logs **Warn** when total time exceeds **`SlowLogThreshold`** in **`zmodel`** framework tuning, and records a split between **pre-send processing** and **`SendMsg`** latency.
- **Tests (fuzz)**: **`go test -fuzz`** entry points in **`zcodec`**, **`zroute`**, **`zaoi`**, **`zactor`**, **`ztrace`**, **`zdiscovery`**, **`zgate`**, **`zmsg`**, **`zscript`**, etc. (no panic; key assertions).

### Changed

- **`examples/im_multi_client_load`**: batch-flush **recv** counter updates to reduce global atomic contention under load (still reflects received replies count).

### Fixed

- **`zgate` / `zmsg`**: fuzz-related tests compile and run correctly.

### Documentation

- **Documentation synchronized**: **`docs/EXAMPLES.md`**, **`docs/EXAMPLES_EN.md`**, **`docs/MODULE_API.md`**, **`docs/MODULE_API_EN.md`**, and **`zgate/README.md`** updated with **`--reactor` / `--sharedSendWorker`** and **`mmo_web_demo`** usage guidance.

## [0.1.0] - 2026-03-27

### Added
- **Actor Runtime (zactor)**
  - Single Actor with single mailbox (MPSC lock-free queue)
  - Message handling, Tick, RPC, Dispatcher extensions
  - CircuitBreaker
  - Watchdog monitoring

- **Unified Gateway (zgate)**
  - TCP / WebSocket / KCP long connection support
  - Optional HTTP service (zhttp)
  - TLS / GM-TLS (National Secret) support
  - Session management and routing strategies

- **Service Discovery (zdiscovery)**
  - Etcd implementation
  - Noop implementation (single machine/testing)

- **Routing (zroute)**
  - FirstCandidateStrategy
  - RoundRobinStrategy
  - RendezvousHashStrategy

- **Message Bus (znats / zbus)**
  - NATS connection pool and broadcast
  - Bus abstraction

- **Observability**
  - zmetrics: Prometheus metrics export
  - zmonitor: Runtime monitoring data
  - ztrace: W3C traceparent distributed tracing
  - zpyroscope: optional Grafana Pyroscope continuous profiling (decoupled from `zmetrics`); re-exports `Config`, `Profiler`, and related symbols so apps only import this package, not `github.com/grafana/pyroscope-go` directly

- **Script Engines (5 types)**
  - zjs: JavaScript engine
  - ztengo: Tengo script
  - zlua: Lua engine
  - zstarlark: Starlark script
  - zscript: Generic script interface

- **Other Modules**
  - zaoi: 9-grid AOI (spatial proximity)
  - zcheck: Health checks
  - zconfig: Configuration management
  - zmodel / zmsg: Message models and serialization

### Documentation
- Documentation index: `docs/DOCS_INDEX_EN.md` (architecture, monitoring, module API, examples, book, etc.)

### Performance
- Single-machine benchmark: 500 connections / 10K QPS
- RTT P50: ~5.5ms
- RTT P99: ~34ms
- Memory usage: ~28MB
- GC pause ratio: <0.1%

### License
- AGPL-3.0 + Commercial Dual License
- Dependency zhenyi-base is MIT licensed

---

## Version History

| Version | Date | Description |
|---------|------|-------------|
| 0.1.1 | 2026-04-02 | Gateway reactor/shared-send switches, MMO AOI+combat sample, expanded fuzz coverage |
| 0.1.0 | 2026-03-27 | Initial open source release |
