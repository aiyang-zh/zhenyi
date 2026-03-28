# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

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
| 0.1.0 | 2026-03-27 | Initial open source release |
