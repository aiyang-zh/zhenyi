# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## 2026-04-10

### Changed

- **zactor/group**: `Group.Run` now uses "return error + rollback successful actors (Close + Unregister)" on startup failure, instead of `Fatal` exit; `watchActor` is started only after all local actors are initialized successfully.
- **zactor**: move `mailBoxQueue.Close` and `workerPool.Release` into `closeOnce` in `Actor.Close`, making repeated-close paths safer.
- **zstartup**: `App.Run` now uses a unified, idempotent shutdown path (shared by signal shutdown and startup-failure cleanup).
- **ziface**: `IGroup` now exposes a single lifecycle close entry `Close(ctx)` and no longer leaks implementation-detail methods such as `CloseScriptEngines`.
- **zdiscovery/noop**: make `Watch` channel buffered to reduce sender-side blocking risk under misuse (noop semantics unchanged).
- **zgate**: add debug logs in `sendClient` for "channel already closed" race cases to improve diagnostics.

## 2026-04-03

### Added

- **zgate**: **`WithNetServerHook(func(IServer))`** — a hook invoked after the underlying **ztcp/zws/zkcp** server is created and injected with TLS/encrypt/shared-send; useful for extra tuning on **`znet.BaseServer`** (e.g. **`SetHeartbeatTimeout`**). Multiple hooks are chained in registration order.
- **`examples/mmo_web_demo`**: broadcast **`world_snapshot`** to the new room (and the previous room on room switch) after entering, so peers see each other immediately.
- **`examples/mmo_web_demo`**: add **`attackRange`** to **`enter_ack`** / **`world_snapshot`**; the Web client draws a melee range ring consistent with the server.
- **Docs**: **`docs/EXAMPLES.md`** / **`docs/EXAMPLES_EN.md`** add `im_single_demo_bench` description and run example.

### Changed

- **`examples/mmo_web_demo`**: **`pickAttackTarget`** no longer falls back to "attack nearest" when an explicit **`targetId`** is invalid/out of range; **`flushRespawns`** now takes a unified timestamp passed in by the caller.
- **`examples/mmo_web_demo` (Web)**: Shift+Click uses an **AABB** hit test matching the sprite size, and picks the topmost target by iterating draw order in reverse.

### Fixed

- **`examples/mmo_web_demo`**: fix late-join synchronization where existing players couldn't see newcomers until a **`MSG_MOVE`** was received.
- **zactor**: fix high CPU usage when Actors are idle (adjust idle backoff sleep granularity to avoid busy spinning).

### Documentation

- **`docs/MODULE_API.md`** / **`docs/MODULE_API_EN.md`** and **`zgate/README.md`**: document **`WithNetServerHook`**.
- **`examples/im_single_demo/README.md`**: add TLS/GM-TLS notes and document **`--reactor`** / **`--sharedSendWorker`** flags.
- **`examples/mmo_web_demo/README.md`**: note that the demo disables idle read timeout via **`WithNetServerHook`** for local UX.

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
