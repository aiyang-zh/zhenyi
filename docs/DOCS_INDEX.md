# 文档索引（Docs Index）

本页用于快速导航 `zhenyi` 的核心文档，按“上手 -> 架构 -> 能力 -> 观测 -> 运维”组织。

根目录 [README.md](../README.md) 仅为简要入口；**完整文档目录、模块导航与外链见本页及下文各节**。**文档说明、CI 与许可条款**见根 README 页脚及 [LICENSE](../LICENSE)。文档与可复现实现不一致时，以源码与测试为准，欢迎 Issue/PR。

## 1. 入门与总览

- [图书：Go Actor 模型与实时应用（`docs/books/go-actor-realtime`）](books/go-actor-realtime/README.md)（与仓库实现对照阅读；许可证见该书 README）
- [商业授权说明](../COMMERCIAL_LICENSE.md)（对外网络服务等完整口径）
- [支持（Support）](../SUPPORT.md)
- [安全策略与加固说明](../SECURITY.md)
- [新手教程](BEGINNER_GUIDE.md)
- [架构说明](ARCHITECTURE.md)
- [模块 API 导航](MODULE_API.md)
- [示例总览](EXAMPLES.md)
- [信创适配分析与初步验证](XINCHUANG.md)（国产化环境：Linux/arm64、无 CGO、交叉构建建议）

## 2. 核心能力文档

- [全局变量与钩子 / 启动检查](GLOBALS_AND_HOOKS.md)
- [第三方依赖与许可证提示](../THIRD_PARTY_LICENSES.md)
- [监控与可观测性总览](MONITORING_OVERVIEW.md)（含 **第 4 节可选 Pyroscope** 持续剖析）
- [监控指标明细](MONITORING_METRICS.md)
- [编解码适配器（IMessage）](CODEC_ADAPTERS.md)
- [TLS/GM-TLS（国密）](../zgate/README.md)（网关加密接入：`SetTLSConfig` / `SetGMTLS`）
- [发布终检清单](RELEASE_CHECKLIST.md)
- [首发说明（v0）](RELEASE_NOTES_v0.md)

## 3. 模块 README 导航（完整列表）

与 [MODULE_API.md](MODULE_API.md) 分工：MODULE_API 偏「能力地图与阅读顺序」，下表偏「直达各包 README」。

| 包 | 说明 |
|------|------|
| [zmodel](../zmodel/README.md) | 消息与模型 |
| [ziface](../ziface/README.md) | 接口定义 |
| [zmsg](../zmsg/README.md) | 线协议与对象池 |
| [zactor](../zactor/README.md) | Actor 运行时 |
| [zgate](../zgate/README.md) | 统一网关 |
| [zhttp](../zhttp/README.md) | HTTP 服务 |
| [zstartup](../zstartup/README.md) | 启动编排 |
| [zmetrics](../zmetrics/README.md)、[zmonitor](../zmonitor/README.md)、[ztrace](../ztrace/README.md)、[zpyroscope](../zpyroscope/README.md) | 可观测性（指标 / 快照 / trace / 可选剖析） |
| [zbus](../zbus/README.md)、[znats](../znats/README.md) | 消息总线 |
| [zconfig](../zconfig/README.md)、[zstream](../zstream/README.md) | 配置与业务 Actor 封装 |
| [zaoi](../zaoi/README.md) | 空间邻近 AOI |
| [zroute](../zroute/README.md)、[zdiscovery](../zdiscovery/README.md) | 路由与发现 |
| [zscript](../zscript/README.md)、[zjs](../zjs/README.md)、[zlua](../zlua/README.md)、[zstarlark](../zstarlark/README.md)、[ztengo](../ztengo/README.md) | 脚本引擎 |
| [zcodec](../zcodec/README.md) | 编解码辅助 |
| [zcheck](../zcheck/README.md) | 启动自检（全局依赖） |

## 4. 构建、测试与覆盖率

```bash
make test          # 单测
make bench         # 基准测试
make coverage      # 覆盖率
make cover-html    # 生成 coverage.html
make docs-check    # 相对链接断链检查（docs/**、examples/**、根与各包 README*.md）
make release-check # 发布前终检（docs-check + go test -count=1 + bug-check）
go test ./... -count=1
go test ./... -bench=. -benchmem
```

## 5. 维护建议

- 代码接口变化后，同步更新对应模块 README 与本索引
- 新增模块时，在 `README.md` 与本索引同时补充入口
- 以代码行为为准，避免保留过时示例签名
