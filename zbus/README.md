# zbus

**消息总线抽象模块**：定义跨进程 Topic 广播/订阅契约，不绑定具体中间件实现。

## 模块定位

- 提供统一 `TopicBus` 接口，隔离上层与底层中间件差异
- 通过全局 `DefaultBus` 供 Gate/Actor 快速接入
- 允许业务在测试场景替换为内存实现

## 核心接口

```go
type TopicBus interface {
    Broadcast(topic string, data []byte) error
    Subscribe(topic string, handler Handler) (Subscription, error)
}
```

## 使用建议

- 生产环境通常注入 `znats` 实现到 `zbus.DefaultBus`
- 单机/测试可替换为 in-memory 实现
- zactor 跨进程发送依赖 `DefaultBus`，启动前需注入

## 相关文档

- 模块 API 导航：`../docs/MODULE_API.md`
- NATS 适配：`../znats/README.md`
