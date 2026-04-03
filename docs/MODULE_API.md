# 模块 API 导航

本文档提供模块级 API 入口索引，帮助快速定位“去哪里用”。

## 1. 启动与编排

- `zstartup`：应用启动编排、Actor 工厂注册
- `zcheck`：启动前依赖自检（bus/nats/metrics 等）

## 2. 网关与网络入口

- `zgate`：统一网关（TCP/WS/KCP），会话映射、收发链路
  - 常用：`NewServer`、`Init`、`RunServer`、`SetHTTPAddr`、`SetTLSConfig`
  - 性能/模型：`SetReactorMode`（TCP 无 TLS 时可选 reactor 读）、`SetSharedSendWorkerMode`（共享写 worker，默认关）、`WithNetServerHook`（底层 net server 创建后回调，用于 `SetHeartbeatTimeout` 等扩展调参）
- `zhttp`：可选 HTTP 服务能力

## 3. Actor 与消息

- `zactor`：Actor 生命周期、消息处理、Tick、RPC、Group
- `zmodel`：`ActorConfig`、`ActorCmd`、框架调优配置
- `zmsg`：消息结构体与对象池（`GetMessage/Retain/Release`）
- `ziface`：核心接口定义（跨模块契约）

## 4. 路由、发现、总线

- `zroute`：本地/远程路由策略
- `zdiscovery`：服务发现
- `zbus`：总线抽象
- `znats`：NATS 实现与默认总线接入

## 5. 可观测性

- `zmetrics`：Prometheus 指标、健康探针
- `zmonitor`：结构化监控快照与管理器
- `ztrace`：trace 透传与解析
- `zpyroscope`：可选 Pyroscope 持续剖析（与 `zmetrics` 解耦，见 `zpyroscope/README.md`）

## 6. 脚本与扩展

- `zscript`：脚本统一抽象与上下文
- `zjs` / `zlua` / `zstarlark` / `ztengo`：具体脚本引擎
- `zcodec`：编解码辅助

## 7. 其他基础能力

- `zaoi`：AOI 能力
- `zstream`：业务 Actor Server 轻封装（`zactor.Actor` 包装层）
- `zconfig`：配置管理

## 8. 深入阅读顺序（推荐）

1. `README.md`
2. `BEGINNER_GUIDE.md`
3. `ARCHITECTURE.md`
4. 目标模块 `README.md`
5. `MONITORING_OVERVIEW.md` 与 `GLOBALS_AND_HOOKS.md`
