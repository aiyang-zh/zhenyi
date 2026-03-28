# Global Variables, Hooks, and Startup Checks

This document summarizes **process-level global state**, **injectable hooks**, and **recommended checks before startup or Init** for each package in zhenyi. Implementation is based on the current repository code.

---

## 1. By Scenario: Required Checks

| Scenario | Check Item | Typical Behavior When Not Met |
|----------|------------|------------------------------|
| **Gate cross-process routing** (`routeToRemoteActor`) | `zbus.DefaultBus != nil` | Logs "remote bus is not configured", falls back to no-route |
| **Actor `SendMsg` / `Broadcast` to remote** | `zbus.DefaultBus != nil` | Returns "bus not configured" error |
| **Actor subscribes cross-process topic** (`pubsub`) | `zbus.DefaultBus != nil` | Skips subscription and logs warning |
| **Using znats default connection pool** (paired with `NewDefaultNats`) | `znats.DefaultNatsClient != nil` and already `Connect` | Publish/Subscribe fails |
| **Multi-process discovery (Etcd, etc.)** | `Group.SetDiscoverer(d)` completed before routing | `Find*` returns no data, remote routing fails |
| **zstartup starts multiple Actors** | Each `ActorType` has `RegisterActorFactory` registered | `InitActors: no ActorFactory registered` |

**Recommendation**: In `main` or unified `Init`, explicitly check "global dependencies that this process will definitely use" and `log.Fatal` / return error to avoid runtime failures. Typical cross-process startup sequence:

1. `znats.NewDefaultNats(url, poolSize)` (internally sets `DefaultNatsClient` and `zbus.DefaultBus`)
2. `DefaultNatsClient.Connect(ctx)`
3. **`zcheck.Validate(zcheck.Config{...})`** (see package [zcheck](../zcheck/README.md))
4. Create Gate / Group, `SetDiscoverer` (if needed)
5. `gate.Init` / `RunServer`

---

## 2. Global Variables (By Package)

### zbus

| Symbol | Type | Description |
|--------|------|-------------|
| `DefaultBus` | `TopicBus` | Cross-process message bus implementation; **default nil**, needs injection (common: `znats.NewDefaultNats(...)` auto-injects, or manually `zbus.DefaultBus = znats.NewNatsBus(pool)`) |

### znats

| Symbol | Type | Description |
|--------|------|-------------|
| `DefaultNatsClient` | `*NatsPool` | Initialized by `NewDefaultNats` via `sync.Once`; also sets `zbus.DefaultBus` |
| `DefaultMaxRetries` / `DefaultRetryDelay` | constants | Connection retry parameters |
| `DefaultURL` | constant | Default NATS address string |

### zmetrics

| Symbol | Type | Description |
|--------|------|-------------|
| `globalRegistry` | `*Registry` | Created in package init; `Global()` is always non-nil |
| `DefaultLatencyBounds` | `[]float64` | Default histogram buckets (milliseconds) |
| Counters/Gauges/Histograms in `framework.go` | pre-registered metrics | Lazily registered to `globalRegistry` via `Global()` |
| `GoMem*` / `GoGC*` etc. in `runtime.go` | pre-registered metrics | Only periodically updated when `StartRuntimeCollector` is called |
| `HandlerSlowLogThreshold` | `time.Duration` | Handler slow call threshold; can be overridden at startup |

### zmodel

| Symbol | Type | Description |
|--------|------|-------------|
| `DefaultFrameworkTuning` | `FrameworkTuning` | Actor batching, default WorkSize, etc.; read-only semantics at runtime |
| `frameworkTuningValue` | `atomic.Value` | Written by `SetFrameworkTuning` |

### zpoolobs

| Symbol | Type | Description |
|--------|------|-------------|
| Global `atomic.Pointer[observerHolder]` | internal | `SetObserver(nil)` to disable observation |
| `GetObserver` / `SetObserver` | functions | Object pool `IPoolObserver` (default: `zmetrics.Enable` installs `GlobalPoolObserver`); **optional** custom override |

### zactor (package-level, not Actor instance)

| Symbol | Description |
|--------|-------------|
| `traceEnabled` etc. | Injected by `SetTraceHooks`, all traces disabled by default |
| `asyncTaskPool` / `asyncTaskPoolOnce` | in `handlemsg.go`, async task pool |
| `DefaultMaxPendingRPCs` etc. | Sender default parameters |

### zmsg

| Symbol | Description |
|--------|-------------|
| `messagePool` / `messagePoolOnce` | `GetMessage` object pool |
| `DEBUG_LIFECYCLE` | Behavior differs when build tag `debug_lifecycle` is set (default off) |

### zmonitor

| Symbol | Description |
|--------|-------------|
| `systemMonitorCache` | `CollectSystemMonitor` cache; interval controlled by `SetSystemMonitorCacheInterval` |

---

## 3. Hooks and Injection Points (By Module)

### zgate.Server (`zgate`)

| Method | Timing | Purpose |
|--------|--------|---------|
| `SetRemoteRouteStrategy` | Before Run | Remote primary selection (`PickOne`, HRW / round-robin, etc.) |
| `SetHTTPAddr` | Before Run | Enable built-in HTTP |
| `OnNoRoute` | Before Run | Custom response or logging when no route |
| `SetTraceHook` | Before Run | Rewrite trace fields when Gate receives packets |
| `OnChannelClose` | Before Run | Connection close callback |
| `OnAccept` | Implemented in Server, can work with `channel.SetCloseCall`, etc. | Rate limiting, authentication, etc. (same path as `OnRead` in `gate.go`) |

`IServer`'s `OnAccept` / `OnRead` are bound to this `Server` when `RunServer` is called.

### zactor.Actor

| Method | Timing | Purpose |
|--------|--------|---------|
| `SetGroup` | After joining Group | Local/remote routing, `SendMsg` |
| `SetIActor` | As early as possible (e.g., in Gate `NewServer`) | Cache `IToClientFastPath`, etc. |
| `SetPoolObserver` | Before Sender initialization | Override default pool observation |
| `RegisterTickFn` | Before Run | Periodic Tick |
| `SetInitServer` | Before Run | Custom logic in `Init` phase |
| `GetHandleMgr().RegisterHandle` / Dispatcher | Before Run | Business message handling |

### zactor (package functions)

| Function | Timing | Description |
|----------|--------|-------------|
| `SetTraceHooks` | **Before any Actor Run, only takes effect once** (`sync.Once`) | Integration with `ztrace` |

### zpoolobs

| Function | Description |
|----------|-------------|
| `SetObserver` | Global pool observation (relay forwarding); `zmsg`/`zactor` etc. pools call current observer when created |

### zmodel

| Function | Description |
|----------|-------------|
| `SetFrameworkTuning` | Replace default tuning; recommended to call early in process |

### zmetrics

| Function | Description |
|----------|-------------|
| `StartRuntimeCollector(ctx, interval)` | Start Go MemStats/GC collection; **starts only once globally** (`sync.Once`) |

### zmonitor

| Function | Description |
|----------|-------------|
| `SetSystemMonitorCacheInterval` | Control `CollectSystemMonitor` cache refresh interval; `0` means fresh sample each time |

### zstartup.App

| Method | Description |
|--------|-------------|
| `RegisterActorFactory` | Each `ActorType` to be started must be registered, otherwise `initActors` fails |

---

## 4. Optional / Tunable Global Items

- **zmetrics**: Custom metrics register via `Global().Counter/Gauge/Histogram`; `WritePrometheus` exports full table.
- **Script engine packages** (`zjs` / `zlua` / `zstarlark` / `ztengo`): Each engine has default config and VM-level `SetGlobal`, see package README for usage.
- **zdiscovery**: No package-level singleton; each process holds its own `EtcdDiscovery` instance and injects into Group via `SetDiscoverer`.

---

## 5. zcheck Package (Recommended)

| API | Description |
|-----|-------------|
| `zcheck.Validate(cfg)` | Validates according to [Config](../zcheck/check.go), returns `errors.Join` merged errors on failure |
| `zcheck.ValidateOrPanic(cfg)` | Same as above, panics on failure |

---

## 6. Maintenance Notes

When adding new APIs of the following types, please sync update this document and `zcheck` implementation (if applicable):

- New `var Default*` or "process singleton"
- New `Set*` / `Register*` / `On*` hooks that are "must be called once at startup" by convention
