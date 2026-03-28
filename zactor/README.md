# zactor

**Actor 运行时核心模块**：提供单 Actor 单 mailbox 的消息处理模型，以及 Group 编排、RPC、Tick、Watchdog 等能力。

## 模块定位

- `zactor` 是 `zhenyi` 的执行核心，负责 Actor 生命周期与消息处理主循环。
- 典型链路：`zgate` 收包后转 `ActorCmd`，由 `zactor` 投递和执行 Handler。
- 适用于高并发、低延迟、需要清晰状态边界的业务处理场景。

## 核心能力

- **Actor 生命周期**：`NewActor`、`Init`、`Close`
- **消息处理**：`RegisterHandle`、`Push`、`SendMsg`
- **异步任务**：`AsyncRun`、`AsyncRunWithMsg`
- **定时任务**：`RegisterTickFn`
- **组编排**：`NewGroup`、`AddActor`、`RegisterRoutes`、`Run`
- **可观测与保护**：Watchdog 阻塞检测、RPC 熔断与超时统计

## 最小用法

```go
cfg := zmodel.ActorConfig{Id: 1, Name: "im", ActorType: 2, Index: 0}
actor := zactor.NewActor(cfg)
actor.SetIActor(actor)
actor.GetHandleMgr().RegisterHandle(100, func(ctx context.Context, msg *zmsg.Message) {
    // 业务处理
})
if err := actor.Init(ctx); err != nil {
    return err
}
```

## 使用建议

- Handler/Tick 保持短小，阻塞操作下沉到 `AsyncRun`
- 涉及跨 Actor 调用时必须使用带超时的 `context.Context`
- 所有池化消息按约定 `Release`，避免对象泄漏

## 路由快路径扩展

- `IGroup` 默认通过 `LookupActorsByMsgID` 返回候选副本（可安全修改）。
- 若实现了 `ziface.IGroupRouteTableView`，路由层可走 `LookupActorsByMsgIDView` 的只读视图快路径，避免热路径 slice `make+copy` 分配。
- 约束：`LookupActorsByMsgIDView` 的返回切片必须视为只读，调用方不得修改；需要可变结果时仍使用 `LookupActorsByMsgID`。

## 相关文档

- 总体架构：`../docs/ARCHITECTURE.md`
- 模块导航：`../docs/MODULE_API.md`
- 示例总览：`../docs/EXAMPLES.md`
- 网关协作：`zgate/README.md`
