# zhenyi 架构说明

本文档描述 `zhenyi` 作为"实时应用解决方案"的核心分层、关键数据流和扩展点。

## 1. 分层关系

- **解决方案层（zhenyi）**：以 Actor 运行时为核心引擎，叠加网关、路由、监控、脚本、消息总线适配
- **基础能力层（zhenyi-base）**：网络协议、连接管理、工具库、通用接口

`zhenyi` 通过接口与 `zhenyi-base` 协作，在保持高性能热路径的同时提供可落地的实时应用工程能力。

## 2. 核心组件

- `zgate`：统一入口，承接 TCP/WS/KCP 长连接
- `zactor`：单 Actor 单 mailbox，处理消息与 Tick
- `zmsg` / `zmodel`：消息载体、对象池与模型定义
- `zroute` + `zdiscovery`：本地/远程路由与发现
- `zmetrics` + `zmonitor` + `ztrace`：指标、监控快照与链路追踪；可选 `zpyroscope` 持续剖析（与 Prometheus 指标解耦）
- `znats` / `zbus`：跨进程消息总线

## 3. 典型消息流（长连接）

1. 客户端消息进入 `zgate`
2. `zgate` 将线协议消息转换为 `ActorCmd`
3. 消息进入目标 Actor mailbox
4. Handler 处理后产生响应
5. `zgate` 根据会话映射回写客户端

## 4. 路由顺序（Gate）

`zgate` 的默认路由顺序为：

1. Gate 自身 handler
2. 本进程 LocalRouter
3. 远程候选（discover + bus）
4. 无路由回退（OnNoRoute）

这个顺序保证了本地优先、远程兜底、失败可观测。

## 5. 可扩展点

- 路由策略：`SetRemoteRouteStrategy`
- 无路由处理：`OnNoRoute`
- Trace 注入：`SetTraceHook`
- 网关 TLS/GM-TLS：`SetTLSConfig` / `SetStandardTLS` / `SetGMTLS`
- 脚本引擎：`zscript` + `zjs/zlua/zstarlark/ztengo`

## 6. 可观测性路径

- 指标导出：`zmetrics.Enable(...)->/metrics`
- 健康探针：`/healthz`、`/readyz`
- Gate 连接和路由指标：由 `zgate` 注入底层网络指标桥接
- 持续剖析（可选）：`zpyroscope`（非 Prometheus 指标，与 `/metrics` 互补；见 [MONITORING_OVERVIEW.md](MONITORING_OVERVIEW.md) 第 4 节）

详细指标清单见 [MONITORING_METRICS.md](MONITORING_METRICS.md)。

## 7. 编解码与消息适配

- 业务消息最终承载在 `zmsg.Message.Data`（`[]byte`）中。
- 编解码通过实现 `ziface.IMessage` 的适配器完成（框架在发送路径上调用 `MarshalToVT`，并把结果写入 `Message.Data`）。
- 推荐参考：[`docs/CODEC_ADAPTERS.md`](CODEC_ADAPTERS.md)。
