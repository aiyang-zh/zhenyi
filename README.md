# zhenyi

**分布式实时应用（Actor 引擎）**

*基于 [zhenyi-base](https://github.com/aiyang-zh/zhenyi-base)（MIT）· AGPL-3.0 + 双授权 · 长连接 · 低延迟*

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![zhenyi-base](https://img.shields.io/badge/base-zhenyi--base%20(MIT)-blue?style=flat-square)](https://github.com/aiyang-zh/zhenyi-base)
[![License](https://img.shields.io/badge/License-AGPL--3.0%20%2B%20Commercial-red.svg?style=flat-square)](COMMERCIAL_LICENSE.md)
[![Commercial](https://img.shields.io/badge/Commercial-Dual_License-gold?style=flat-square)](COMMERCIAL_LICENSE.md)
[![Tests](https://github.com/aiyang-zh/zhenyi/actions/workflows/test.yml/badge.svg?branch=main)](https://github.com/aiyang-zh/zhenyi/actions/workflows/test.yml)
[![Bug Check](https://github.com/aiyang-zh/zhenyi/actions/workflows/bug-check.yml/badge.svg?branch=main)](https://github.com/aiyang-zh/zhenyi/actions/workflows/bug-check.yml)
[![Docs Link Check](https://github.com/aiyang-zh/zhenyi/actions/workflows/docs-link-check.yml/badge.svg?branch=main)](https://github.com/aiyang-zh/zhenyi/actions/workflows/docs-link-check.yml)
[![信创](https://img.shields.io/badge/%E4%BF%A1%E5%88%9B-%E9%80%82%E9%85%8D%E5%88%86%E6%9E%90-026e00?style=flat-square)](docs/XINCHUANG.md)

zhenyi 在 **zhenyi-base** 之上提供 **Actor 运行时、统一网关（TCP/WS/KCP）、跨进程总线/发现、指标与追踪** 等能力，面向长连接与实时业务后台。详细架构、模块清单与各包说明见 **[文档索引](docs/DOCS_INDEX.md)**（勿在根 README 重复维护大表）。

## 项目亮点

- **Actor 运行时**：服务级 Actor、MPSC 邮箱、Tick/RPC、协程池承载异步任务；可选熔断与限流。  
- **统一网关**：`zgate` 支持 TCP / WebSocket / KCP，可选 HTTP、TLS/国密 GM-TLS、线协议层载荷加密等；会话与路由对接业务 Actor。  
- **分布式**：Etcd 服务发现、NATS 跨进程总线、远程路由策略（含 RendezvousHash 等）。  
- **可观测**：Prometheus 指标、健康探针、链路追踪钩子与监控快照；详见 [监控总览](docs/MONITORING_OVERVIEW.md)。  
- **持续剖析（可选）**：[`zpyroscope`](zpyroscope/README.md) 封装 Pyroscope，与 `zmetrics` 解耦；见 [监控总览](docs/MONITORING_OVERVIEW.md) 第 4 节。  
- **性能取向**：消息对象池、引用计数与零拷贝取向（热路径以源码与基准为准）。  
- **信创**：国产化环境适配与构建要点见 [信创](docs/XINCHUANG.md)（与仓库 CI/文档一并维护）。  

## 快速开始

```bash
go test ./... -count=1
go run ./examples/im_single_demo
# 另开终端：go run ./examples/im_single_client
```

单机示例 **不依赖** Etcd/NATS；多进程示例（如 `im_multi_demo`）需要 Etcd + NATS，见 **[示例总览](docs/EXAMPLES.md)**。

## 读文档

| 目的 | 链接 |
|------|------|
| 总目录（完整索引：Support/安全/第三方许可/监控/发布/各包 README/构建命令等） | [docs/DOCS_INDEX.md](docs/DOCS_INDEX.md) |
| 新手 | [docs/BEGINNER_GUIDE.md](docs/BEGINNER_GUIDE.md) |
| 架构 | [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) |
| 模块与 API | [docs/MODULE_API.md](docs/MODULE_API.md) |
| 信创适配 | [docs/XINCHUANG.md](docs/XINCHUANG.md) |

图书（与实现对照）：[docs/books/go-actor-realtime](docs/books/go-actor-realtime/README.md)

## 协议与合规

**AGPL-3.0 + 商业双授权**。开源使用须遵守 [LICENSE](LICENSE)；商业、闭源或对外网络服务等场景请先阅读 **[COMMERCIAL_LICENSE.md](COMMERCIAL_LICENSE.md)**，商业联系：`1093993119@qq.com`。

## 开发与 CI

```bash
make test           # 单测
make release-check  # 发布前：文档链接 + 测试等
```

更多 Makefile 目标、基准测试与覆盖率见各 `Makefile` / [docs/EXAMPLES.md](docs/EXAMPLES.md)。

---

*根 README 刻意保持简短；模块级说明在各子目录 `README.md`，统一入口在 [DOCS_INDEX](docs/DOCS_INDEX.md)。*  
*本仓库大量借助 AI 辅助完成文档与代码，虽经人工审阅与测试，**仍可能有未覆盖场景、缺陷或与说明不一致之处**，不构成任何明示或默示担保。**用于生产前请自行充分验证**；发现问题欢迎 Issue/PR。*
