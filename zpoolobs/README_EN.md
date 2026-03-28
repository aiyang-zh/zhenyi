# zpoolobs

`zpoolobs` provides unified entry for zhenyi layer object pool observation, solving the "object pool created first, monitoring enabled later" observation gap problem.

## Module Responsibilities

- Unified management of global object pool observer: `SetObserver` / `GetObserver`
- Optional observer resolution: `Resolve`
- Unified pool creation entry with pool name: `NewObservedPool`
- zhenyi layer pool name constants: `PoolName*`

## Usage

```go
// 1) Register global observer (usually done internally by zmetrics.Enable)
zpoolobs.SetObserver(myObserver)

// 2) Create object pool with unified entry (automatically connects to global observer relay)
pool := zpoolobs.NewObservedPool(zpoolobs.PoolNameZActorAsyncTask, func() *Task {
	return &Task{}
})

// 3) Use object pool
t := pool.Get()
pool.Put(t)
```

## Design Points

- `poolRelay` as fixed observer attached to pool, relays to `GetObserver()` at runtime
- Even if `SetObserver` is later than pool creation, subsequent pool events can still be observed
- `Resolve` supports "caller explicitly passes observer takes priority, otherwise falls back to global observer"
