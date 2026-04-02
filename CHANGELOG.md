# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [0.1.1] - 2026-04-02

### Added

- **zgate**：**`SetReactorMode(bool)`** — 在 **TCP**、**无传输层 TLS/GM-TLS**、底层为 **`*ztcp.Server`** 时走 **`ztcp.ServerReactor`**（Linux epoll / macOS kqueue）；否则保持原 **`Server(ctx)`**。
- **zgate**：**`SetSharedSendWorkerMode(bool)`** — 将底层长连接 **`IServer`**（**ztcp / zws / zkcp**，经 `znet.BaseServer`）切到 **共享写 worker**（默认 **关闭**，与历史行为一致）。
- **示例**：**`im_single_demo`** / **`im_multi_demo`** 增加 **`--reactor`**、**`--sharedSendWorker`**，并调用上述 Gate API。
- **`examples/mmo_web_demo`**：浏览器 MMO 示例补充战斗循环（攻击、HP、阵亡、延迟重生、冷却与范围判定）。
- **`examples/mmo_web_demo`**：**`world_snapshot`** / **`combat_event`** 按 **`zaoi`**（**`WorldManager` + `Zone` + `StaticAoi`** 九宫格与视距）做 AOI 过滤下发。
- **zactor**：**`SendToClient`** 在总耗时超过 **`zmodel`** 框架调优中的 **`SlowLogThreshold`** 时打 **Warn**，并拆分记录 **前半段处理耗时** 与 **`SendMsg`** 耗时。
- **测试（fuzz）**：**`zcodec`** / **`zroute`** / **`zaoi`** / **`zactor`** / **`ztrace`** / **`zdiscovery`** / **`zgate`** / **`zmsg`** / **`zscript`** 等包增加 **`go test -fuzz`** 入口（不 panic、关键路径断言）。

### Changed

- **`examples/im_multi_client_load`**：收包回调内 **recv 计数**改为 **批量 flush** 到全局原子，降低高并发下 **`recv` 原子竞争**（统计语义仍为「收到回包数」量级）。

### Fixed

- **`zgate` / `zmsg`**：修复核心模块 fuzz 相关测试，使其可正确编译运行。

### Documentation

- **文档同步更新**：**`docs/EXAMPLES.md`**、**`docs/EXAMPLES_EN.md`**、**`docs/MODULE_API.md`**、**`docs/MODULE_API_EN.md`**、**`zgate/README.md`**，补充 **`--reactor` / `--sharedSendWorker`** 与 **`mmo_web_demo`** 的使用说明。

## [0.1.0] - 2026-03-27

### Added
- **Actor 运行时 (zactor)**
  - 单 Actor 单 mailbox（MPSC 无锁队列）
  - 消息处理、Tick、RPC、Dispatcher 扩展
  - CircuitBreaker 熔断器
  - Watchdog 看门狗监控

- **统一网关 (zgate)**
  - TCP / WebSocket / KCP 长连接支持
  - 可选 HTTP 服务 (zhttp)
  - TLS / GM-TLS (国密) 支持
  - Session 管理与路由策略

- **服务发现 (zdiscovery)**
  - Etcd 实现
  - Noop 空实现（单机/测试用）

- **路由 (zroute)**
  - FirstCandidateStrategy（首选候选）
  - RoundRobinStrategy（轮询）
  - RendezvousHashStrategy（一致性哈希）

- **消息总线 (znats / zbus)**
  - NATS 连接池与广播
  - 总线抽象

- **可观测性**
  - zmetrics: Prometheus 指标导出
  - zmonitor: 运行时监控数据
  - ztrace: W3C traceparent 链路追踪
  - zpyroscope: 可选 Grafana Pyroscope 持续剖析（与 `zmetrics` 解耦）；重导出 `Config` / `Profiler` 等，业务仅需 `import` 本包，无需直接依赖 `github.com/grafana/pyroscope-go`

- **脚本引擎（5种）**
  - zjs: JavaScript 引擎
  - ztengo: Tengo 脚本
  - zlua: Lua 引擎
  - zstarlark: Starlark 脚本
  - zscript: 通用脚本接口

- **其他模块**
  - zaoi: 九宫格 AOI（空间邻近）
  - zcheck: 健康检查
  - zconfig: 配置管理
  - zmodel / zmsg: 消息模型与序列化

### Documentation
- 文档索引：`docs/DOCS_INDEX.md`（架构、监控、模块 API、示例与图书等入口）

### Performance
- 单机压测数据：500 连接 / 10K QPS
- RTT P50: ~5.5ms
- RTT P99: ~34ms
- 内存占用: ~28MB
- GC 暂停占比: <0.1%

### License
- AGPL-3.0 + 商业双授权
- 依赖库 zhenyi-base 为 MIT 授权

---

## 完整版本历史

| 版本 | 日期 | 说明 |
|------|------|------|
| 0.1.1 | 2026-04-02 | 网关 reactor/共享写开关、MMO AOI+战斗示例、fuzz 覆盖扩展 |
| 0.1.0 | 2026-03-27 | 首次开源 |
