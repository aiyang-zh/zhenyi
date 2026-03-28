# zhenyi

**Distributed real-time applications (Actor engine)**

*Built on [zhenyi-base](https://github.com/aiyang-zh/zhenyi-base) (MIT) · AGPL-3.0 + dual license · long-lived connections · low latency*

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![zhenyi-base](https://img.shields.io/badge/base-zhenyi--base%20(MIT)-blue?style=flat-square)](https://github.com/aiyang-zh/zhenyi-base)
[![License](https://img.shields.io/badge/License-AGPL--3.0%20%2B%20Commercial-red.svg?style=flat-square)](COMMERCIAL_LICENSE_EN.md)
[![Commercial](https://img.shields.io/badge/Commercial-Dual_License-gold?style=flat-square)](COMMERCIAL_LICENSE_EN.md)
[![Tests](https://github.com/aiyang-zh/zhenyi/actions/workflows/test.yml/badge.svg?branch=main)](https://github.com/aiyang-zh/zhenyi/actions/workflows/test.yml)
[![Bug Check](https://github.com/aiyang-zh/zhenyi/actions/workflows/bug-check.yml/badge.svg?branch=main)](https://github.com/aiyang-zh/zhenyi/actions/workflows/bug-check.yml)
[![Docs Link Check](https://github.com/aiyang-zh/zhenyi/actions/workflows/docs-link-check.yml/badge.svg?branch=main)](https://github.com/aiyang-zh/zhenyi/actions/workflows/docs-link-check.yml)
[![Xinchuang](https://img.shields.io/badge/Xinchuang-Adaptation-026e00?style=flat-square)](docs/XINCHUANG_EN.md)

zhenyi builds on **zhenyi-base** with an **Actor runtime, unified gateway (TCP/WebSocket/KCP), cross-process bus & discovery, metrics and tracing** for long-lived connections and real-time backends. For full architecture, module map and package docs, see **[Documentation Index](docs/DOCS_INDEX_EN.md)**—avoid duplicating large tables in this README.

## Highlights

- **Actor runtime**: service-level Actors, MPSC mailbox, Tick/RPC, goroutine pool for async work; optional circuit breaking and rate limiting.  
- **Unified gateway**: `zgate` supports TCP / WebSocket / KCP, optional HTTP, TLS / GM-TLS, application-layer payload encryption; sessions and routing connect to business Actors.  
- **Distributed**: Etcd discovery, NATS cross-process bus, remote routing strategies (including RendezvousHash).  
- **Observability**: Prometheus metrics, health probes, tracing hooks and monitoring snapshots; see [Monitoring overview](docs/MONITORING_OVERVIEW_EN.md).  
- **Continuous profiling (optional)**: [`zpyroscope`](zpyroscope/README.md) wraps Pyroscope, decoupled from `zmetrics`; see [Monitoring overview](docs/MONITORING_OVERVIEW_EN.md) section 4.  
- **Performance**: message pooling, refcounting and zero-copy-oriented hot paths (verify against source and benchmarks).  
- **Xinchuang / domestic IT**: adaptation notes in [Xinchuang](docs/XINCHUANG_EN.md), maintained with CI and docs.  

## Quick start

```bash
go test ./... -count=1
go run ./examples/im_single_demo
# Second terminal: go run ./examples/im_single_client
```

The single-process demo **does not** require Etcd/NATS; multi-process examples (e.g. `im_multi_demo`) need Etcd + NATS—see **[Examples overview](docs/EXAMPLES_EN.md)**.

## Documentation

| Topic | Link |
|------|------|
| Full index (Support, security, third-party licenses, monitoring, releases, module README table, Make targets, etc.) | [docs/DOCS_INDEX_EN.md](docs/DOCS_INDEX_EN.md) |
| Beginner's guide | [docs/BEGINNER_GUIDE_EN.md](docs/BEGINNER_GUIDE_EN.md) |
| Architecture | [docs/ARCHITECTURE_EN.md](docs/ARCHITECTURE_EN.md) |
| Module API | [docs/MODULE_API_EN.md](docs/MODULE_API_EN.md) |
| Xinchuang | [docs/XINCHUANG_EN.md](docs/XINCHUANG_EN.md) |

Book (aligned with implementation): [docs/books/go-actor-realtime](docs/books/go-actor-realtime/README.md)

## License & compliance

**AGPL-3.0 + commercial dual license.** Open-source use must follow [LICENSE](LICENSE). For commercial use, closed distribution, or network services to third parties, read **[COMMERCIAL_LICENSE_EN.md](COMMERCIAL_LICENSE_EN.md)** first. Commercial contact: `1093993119@qq.com`.

## Development & CI

```bash
make test           # unit tests
make release-check  # pre-release: doc links + tests, etc.
```

More Make targets, benchmarks and coverage: see `Makefile` and [docs/EXAMPLES_EN.md](docs/EXAMPLES_EN.md).

---

*This README stays short; per-package docs live under each directory’s `README.md` / `README_EN.md`, with a single entry point in [DOCS_INDEX_EN](docs/DOCS_INDEX_EN.md).*  
*Much of this project was built with AI assistance (docs and code), then reviewed and tested; **untested paths, bugs, or doc drift may still exist—this is not a warranty of any kind**. **Validate thoroughly before production use**; issues and PRs welcome.*
