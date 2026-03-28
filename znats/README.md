# znats

**NATS 总线适配模块**：实现 `zbus.TopicBus`，提供默认总线接入与连接池能力。

## 模块定位

- 将 NATS 能力适配为 `zbus` 抽象，供 Gate/Actor 复用
- 支持默认客户端初始化（`DefaultNatsClient`）
- 为跨进程消息路由提供基础设施

## 最小用法

```go
znats.NewDefaultNats(natsURL, poolSize)
if err := znats.DefaultNatsClient.Connect(ctx); err != nil {
    return err
}
// NewDefaultNats 会设置 DefaultNatsClient，并注入默认 zbus
```

## 使用建议

- 启动期先完成 `NewDefaultNats + Connect`，再启动 Gate/Group
- 生产环境建议显式配置连接参数与重连策略
- 与 `zcheck` 组合，可提前发现“总线未初始化/未连接”问题

## 相关文档

- 模块 API 导航：`../docs/MODULE_API.md`
- 全局变量与启动检查：`../docs/GLOBALS_AND_HOOKS.md`
