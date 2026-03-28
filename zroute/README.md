# zroute

**路由策略模块**：定义本地路由与远程首选选址策略，供 `zgate` 路由链路使用。

## 模块定位

- 约束“如何选本地 Actor / 如何排序远程候选”
- 将策略从网关主逻辑解耦，便于按场景替换
- 默认策略可直接开箱，复杂场景可自定义实现接口

## 核心类型

| 类型 | 说明 |
|------|------|
| `LocalRouter` | 进程内按 msgId 路由，实现 `RouteLocal` |
| `RemoteRouteStrategy` | 跨进程首选选址，实现 `PickOne`（零分配） |
| `FirstCandidateStrategy` | 默认：保持发现顺序 |
| `RoundRobinStrategy` | 轮询 |
| `RendezvousHashStrategy` | HRW 一致性哈希（扩缩容友好） |

## 最小用法

```go
gate.SetRemoteRouteStrategy(&zroute.RendezvousHashStrategy{})
```

## 使用建议

- 简单场景可用默认 `FirstCandidateStrategy`
- 多实例且希望会话稳定命中时，优先 `RendezvousHashStrategy`
- 自定义策略时只做“选址”职责，不在策略里耦合网络 I/O

## 相关文档

- 架构说明：`../docs/ARCHITECTURE.md`
- 模块 API 导航：`../docs/MODULE_API.md`
- 网关协作：`../zgate/README.md`
