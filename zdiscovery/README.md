# zdiscovery

**服务发现模块**：提供 `Discoverer` 的 Etcd 与 Noop 实现，用于跨进程 Actor 发现与路由支撑。

## 模块定位

- 维护“Actor 注册信息 -> 可查询视图”
- 为 `zgate` 远程路由与 `zactor` 跨进程调用提供候选来源
- 在单机模式下可用 Noop 实现保持接口一致

## 当前实现

| 实现 | 说明 |
|------|------|
| `NewEtcdDiscovery(ctx, client)` | Etcd |
| `NewNoopDiscovery()` | 单机/测试用空实现 |

## 最小用法

```go
d, _ := zdiscovery.NewEtcdDiscovery(ctx, etcdClient)
group.SetDiscoverer(d)
```

## 使用建议

- 生产环境建议在启动期完成注册与 watch 预热，再开放流量
- 对 discovery 的失败要有降级策略（回退本地路由或快速失败）
- 单机/测试场景优先 `NewNoopDiscovery()`，避免引入外部依赖

## 相关文档

- 架构说明：`../docs/ARCHITECTURE.md`
- 模块 API 导航：`../docs/MODULE_API.md`
- 网关路由：`../zgate/README.md`
