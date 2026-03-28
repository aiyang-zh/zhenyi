# ziface

**接口契约层**：定义方案核心抽象与扩展边界，不包含具体实现。

## 模块定位

- 统一 `Actor/Gate/Group/Discovery/Dispatcher` 等核心接口
- 降低模块耦合，便于替换实现与单元测试
- 为扩展点（路由策略、脚本、发现等）提供稳定边界

## 核心接口（常用）

| 接口 | 说明 |
|------|------|
| `IActor` | Actor 能力：Push、Init、Run、SendMsg、CallActor、GetDispatcher、GetGroup 等 |
| `IServerActor` | 扩展 IActor，增加 RunServer（服务启动阶段） |
| `IGroup` | Group 能力：AddActor、Run、RegisterRoutes、LookupActorsByMsgID、SetDiscoverer |
| `IMessage` | 业务 proto 消息：MarshalVT/UnmarshalVT/GetMsgId |
| `IHttpServer` | HTTP：SetActor、GET/POST/Group、Run |
| `IDiscovery` | 服务发现：Register、FindAllByPrefix、Watch |
| `IDispatcher` | 消息分发：Dispatch |
| `LocalRouter` | 进程内路由：RouteLocal |
| `RemoteRouteStrategy` | 跨进程路由选址策略 |
| `IGroupRouteTableView` | 进程内路由只读视图（零分配） |
| `IGroupRemoteRouteTableView` | 跨进程候选只读视图（零分配） |
| `Handle` | 客户端消息处理函数 |

## 使用

- 业务与方案组件通过接口交互，方便 mock 和扩展；
- 新 Actor 实现 `IActor`（或 `IServerActor`）即可接入 Group/Gate。

## 相关文档

- 总体架构：`../docs/ARCHITECTURE.md`
- 模块 API 导航：`../docs/MODULE_API.md`
- 模型定义：`../zmodel/README.md`
