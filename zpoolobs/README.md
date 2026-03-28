# zpoolobs

`zpoolobs` 提供 zhenyi 层对象池观测的统一入口，解决“对象池先创建、监控后启用”时观测断链的问题。

## 模块职责

- 统一管理全局对象池观测器：`SetObserver` / `GetObserver`
- 提供可选观测器解析：`Resolve`
- 提供带池名的统一建池入口：`NewObservedPool`
- 提供 zhenyi 层池名常量：`PoolName*`

## 使用方式

```go
// 1) 注册全局观测器（通常由 zmetrics.Enable 内部完成）
zpoolobs.SetObserver(myObserver)

// 2) 用统一入口创建对象池（会自动接入全局观测转发）
pool := zpoolobs.NewObservedPool(zpoolobs.PoolNameZActorAsyncTask, func() *Task {
	return &Task{}
})

// 3) 使用对象池
t := pool.Get()
pool.Put(t)
```

## 设计要点

- `poolRelay` 作为固定 observer 挂在池上，运行时转发到 `GetObserver()`
- 即使 `SetObserver` 晚于池创建，后续池事件仍可被观测到
- `Resolve` 支持“调用方显式传入 observer 优先，否则回退全局 observer”
