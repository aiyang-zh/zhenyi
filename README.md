<div align="center">

# zhenyi

**分布式实时应用（Actor 引擎）**

*基于 [zhenyi-base](https://github.com/aiyang-zh/zhenyi-base)（MIT）；zhenyi 采用 AGPL-3.0 + 商业双授权 · 长连接 · 低延迟*

[![Tests](https://github.com/aiyang-zh/zhenyi/actions/workflows/test.yml/badge.svg)](https://github.com/aiyang-zh/zhenyi/actions)
[![Go Reference](https://pkg.go.dev/badge/github.com/aiyang-zh/zhenyi.svg)](https://pkg.go.dev/github.com/aiyang-zh/zhenyi)
[![Go Report Card](https://img.shields.io/badge/go%20report-A+-brightgreen?style=flat-square)](https://goreportcard.com/report/github.com/aiyang-zh/zhenyi)
[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square&logo=go)](https://go.dev/dl/)
[![License](https://img.shields.io/badge/License-AGPL--3.0%20%2B%20Commercial-red.svg?style=flat-square)](COMMERCIAL_LICENSE.md)

**[English](README_EN.md)** · **[文档索引](docs/DOCS_INDEX.md)** · [信创](docs/XINCHUANG.md) · [Actions](https://github.com/aiyang-zh/zhenyi/actions) · [zhenyi-base](https://github.com/aiyang-zh/zhenyi-base)

</div>

---

## 简介

zhenyi 在 **zhenyi-base** 之上提供 Actor 运行时、统一网关（TCP/WS/KCP）、跨进程总线/发现、指标与追踪等能力，面向长连接与实时业务后台。模块清单与架构说明见 **[文档索引](docs/DOCS_INDEX.md)**（勿在本页重复维护大表）。
在 `2 vCPU / 4 GiB` 同机压测条件下，已验证两档实测结果：**7 天稳态约 4 万 QPS（P99 约 1~2ms）**，以及**1 小时高压约 16.7 万 QPS**；详见 **[压测报告](docs/BENCHMARK.md)**。

## 项目亮点

- **Actor 运行时**：服务级 Actor、MPSC 邮箱、Tick/RPC、协程池；可选熔断与限流。
- **统一网关**：`zgate` 支持 TCP / WebSocket / KCP，可选 HTTP、TLS/国密 GM-TLS、线协议载荷加密等。
- **分布式**：Etcd 服务发现、NATS 跨进程总线、远程路由（含 RendezvousHash 等）。
- **可观测**：Prometheus、健康探针、链路追踪、监控快照；详见 [监控总览](docs/MONITORING_OVERVIEW.md)。
- **持续剖析（可选）**：[`zpyroscope`](zpyroscope/README.md) 与 `zmetrics` 解耦；见 [监控总览](docs/MONITORING_OVERVIEW.md) 第 4 节。
- **性能取向**：消息对象池、引用计数与零拷贝取向（以源码与基准为准）。
- **信创**：国产化构建与运行要点见 [信创适配](docs/XINCHUANG.md)。

## 快速开始

```bash
go test ./... -count=1
go run ./examples/im_single_demo
# 另开终端：go run ./examples/im_single_client
```

单机示例 **不依赖** Etcd/NATS；多进程示例（如 `im_multi_demo`）需要 Etcd + NATS → **[示例总览](docs/EXAMPLES.md)**（含 **`--reactor` / `--sharedSendWorker`**、**`mmo_web_demo`** 等）。

## 文档导航

| 文档 | 链接 |
|------|------|
| 总目录（索引、Support、安全、监控、发布等） | [DOCS_INDEX.md](docs/DOCS_INDEX.md) |
| 新手教程 | [BEGINNER_GUIDE.md](docs/BEGINNER_GUIDE.md) |
| 架构 | [ARCHITECTURE.md](docs/ARCHITECTURE.md) |
| 模块与 API | [MODULE_API.md](docs/MODULE_API.md) |
| 压测报告 | [BENCHMARK.md](docs/BENCHMARK.md) |
| 信创适配 | [XINCHUANG.md](docs/XINCHUANG.md) |
| 图书（与实现对齐） | [go-actor-realtime](docs/books/go-actor-realtime/README.md) |

## 协议与合规

**AGPL-3.0 + 商业双授权。** 开源使用须遵守 [LICENSE](LICENSE)。商业、闭源或对外网络服务等场景请先阅读 **[COMMERCIAL_LICENSE.md](COMMERCIAL_LICENSE.md)**。  
商业联系：`1093993119@qq.com`

## 开发与 CI

```bash
make test           # 单测
make release-check  # 发布前：文档链接 + 测试等
```

更多 Make 目标与基准测试见各目录 `Makefile` 与 [示例说明](docs/EXAMPLES.md)。

---

> 本 README 仅作入口；各包文档见子目录 `README.md`，统一从 **[文档索引](docs/DOCS_INDEX.md)** 进入。  
> 部分文档在写作过程中使用 AI 辅助，并经人工校对；仓库持续运行 **CI**（测试、race、覆盖率、`go vet`、文档链接检查等，见 [`.github/workflows`](.github/workflows/)）。权利义务以 **[LICENSE](LICENSE)** 为准；文档与实现不一致时，以可复现的源码与测试结果为准。欢迎 Issue / PR。
