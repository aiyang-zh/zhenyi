# zgate

**统一网关**：嵌 Actor，支持长连接（TCP/WS/KCP）+ 可选 HTTP，将客户端消息路由到后端 Actor。

## 模块定位

- 承接客户端连接与收包
- 统一转为 `ActorCmd` 进入 Actor 处理链
- 负责 `ToClient` 回包下发与路由失败兜底
- 可选启用 HTTP 与 TLS/GM-TLS

## 核心类型

| 类型 | 说明 |
|------|------|
| `Server` | 网关主体：嵌 `*zactor.Actor`，持底层 `IServer` |
| `SessionManager` | authId -> actorType -> actorId 映射（粘性路由） |

## 路由顺序

1. Gate 自身 handler（如登录）
2. 进程内 `LocalRouter.RouteLocal`（Group 路由表）
3. 跨进程 `routeToRemoteActor`（NATS + discovery）
4. 无路由 → `sendNoRouteError` 或 `OnNoRoute` 钩子

## 最小用法

```go
gate := zgate.NewServer(cfg, znet.TCP)
gate.SetHTTPAddr(":8080") // 可选
if err := gate.SetStandardTLS("server.crt", "server.key"); err != nil {
    return err
}
if err := gate.Init(ctx); err != nil {
    return err
}
if err := gate.RunServer(ctx); err != nil {
    return err
}
```

## 常用扩展点

- `SetRemoteRouteStrategy`：远程首选选址（`PickOne`，失败后 fallback）
- 默认优先走 `ziface.IGroupRemoteRouteTableView`（远程候选只读视图）以实现路由热路径零分配
- `OnNoRoute`：无路由兜底处理
- `SetTraceHook`：收包路径注入链路字段
- `SetHTTPAddr`：开启 HTTP 服务
- `SetTLSConfig` / `SetStandardTLS` / `SetGMTLS`：加密接入
- **`SetReactorMode(bool)`**：TCP、且**未**配置传输层 TLS/GM-TLS、底层为 `*ztcp.Server` 时，使用 **`ztcp.ServerReactor`**（Linux/macOS 上 zhenyi-base 的 epoll/kqueue 单循环读）；与 TLS 互斥。
- **`SetSharedSendWorkerMode(bool)`**：底层 **`ztcp` / `zws` / `zkcp`**（`znet.BaseServer`）是否启用 **共享写 worker**（默认 **false**，与历史行为一致）。

## 相关文档

- 总体架构：`../docs/ARCHITECTURE.md`
- 模块导航：`../docs/MODULE_API.md`
- 监控与指标：`../docs/MONITORING_METRICS.md`
