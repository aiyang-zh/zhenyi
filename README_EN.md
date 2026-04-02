<div align="center">

# zhenyi

**Distributed real-time applications (Actor engine)**

*Built on [zhenyi-base](https://github.com/aiyang-zh/zhenyi-base) (MIT); zhenyi is licensed under AGPL-3.0 + commercial dual license · long-lived connections · low latency*

[![Tests](https://github.com/aiyang-zh/zhenyi/actions/workflows/test.yml/badge.svg)](https://github.com/aiyang-zh/zhenyi/actions)
[![Go Reference](https://pkg.go.dev/badge/github.com/aiyang-zh/zhenyi.svg)](https://pkg.go.dev/github.com/aiyang-zh/zhenyi)
[![Go Report Card](https://img.shields.io/badge/go%20report-A+-brightgreen?style=flat-square)](https://goreportcard.com/report/github.com/aiyang-zh/zhenyi)
[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square&logo=go)](https://go.dev/dl/)
[![License](https://img.shields.io/badge/License-AGPL--3.0%20%2B%20Commercial-red.svg?style=flat-square)](COMMERCIAL_LICENSE_EN.md)

**[中文](README.md)** · **[Documentation index](docs/DOCS_INDEX_EN.md)** · [Xinchuang](docs/XINCHUANG_EN.md) · [Actions](https://github.com/aiyang-zh/zhenyi/actions) · [zhenyi-base](https://github.com/aiyang-zh/zhenyi-base)

</div>

---

## Introduction

zhenyi builds on **zhenyi-base** with an Actor runtime, unified gateway (TCP/WebSocket/KCP), cross-process bus & discovery, metrics and tracing for long-lived connections and real-time backends. For the module map and architecture, see the **[documentation index](docs/DOCS_INDEX_EN.md)** (large tables are maintained there, not here).

## Highlights

- **Actor runtime**: service-level Actors, MPSC mailbox, Tick/RPC, goroutine pool; optional circuit breaking and rate limiting.
- **Unified gateway**: `zgate` supports TCP / WebSocket / KCP, optional HTTP, TLS / GM-TLS, application-layer payload encryption.
- **Distributed**: Etcd discovery, NATS cross-process bus, remote routing (including RendezvousHash).
- **Observability**: Prometheus, health probes, tracing, monitoring snapshots; see [Monitoring overview](docs/MONITORING_OVERVIEW_EN.md).
- **Continuous profiling (optional)**: [`zpyroscope`](zpyroscope/README.md), decoupled from `zmetrics`; see [Monitoring overview](docs/MONITORING_OVERVIEW_EN.md) section 4.
- **Performance**: message pooling, refcounting and zero-copy-oriented hot paths (verify against source and benchmarks).
- **Xinchuang / domestic IT**: build and run notes in [Xinchuang](docs/XINCHUANG_EN.md).

## Quick start

```bash
go test ./... -count=1
go run ./examples/im_single_demo
# Second terminal: go run ./examples/im_single_client
```

The single-process demo **does not** require Etcd/NATS; multi-process examples (e.g. `im_multi_demo`) need Etcd + NATS → **[Examples overview](docs/EXAMPLES_EN.md)** (including **`--reactor` / `--sharedSendWorker`** and **`mmo_web_demo`**).

## Documentation

| Doc | Link |
|-----|------|
| Full index (Support, security, monitoring, releases, etc.) | [DOCS_INDEX_EN.md](docs/DOCS_INDEX_EN.md) |
| Beginner's guide | [BEGINNER_GUIDE_EN.md](docs/BEGINNER_GUIDE_EN.md) |
| Architecture | [ARCHITECTURE_EN.md](docs/ARCHITECTURE_EN.md) |
| Module API | [MODULE_API_EN.md](docs/MODULE_API_EN.md) |
| Xinchuang | [XINCHUANG_EN.md](docs/XINCHUANG_EN.md) |
| Book (aligned with code) | [go-actor-realtime](docs/books/go-actor-realtime/README.md) |

## License & compliance

**AGPL-3.0 + commercial dual license.** Open-source use must follow [LICENSE](LICENSE). For commercial use, closed distribution, or network services to third parties, read **[COMMERCIAL_LICENSE_EN.md](COMMERCIAL_LICENSE_EN.md)** first.  
Commercial contact: `1093993119@qq.com`

## Development & CI

```bash
make test           # unit tests
make release-check  # pre-release: doc links + tests, etc.
```

For more Make targets and benchmarks, see each `Makefile` and [Examples](docs/EXAMPLES_EN.md).

---

> This README is only an entry point; per-package docs live under each directory (`README.md` / `README_EN.md`), with a single hub at **[DOCS_INDEX_EN](docs/DOCS_INDEX_EN.md)**.  
> Some documentation may be drafted with AI assistance and reviewed by maintainers. The repo runs **CI** (tests, race, coverage, `go vet`, doc link checks, etc.—see [`.github/workflows`](.github/workflows/)). Legal terms: **[LICENSE](LICENSE)**. If docs and behavior disagree, defer to reproducible source and tests. Issues and PRs welcome.
