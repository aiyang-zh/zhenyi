# 全局变量、钩子与启动检查

本文档汇总 zhenyi 各包中的**进程级全局状态**、**可注入钩子**，以及按场景**建议在启动前或 Init 前完成的检查**。实现以仓库当前代码为准。

---

## 一、按场景：必须检查项

| 场景 | 检查项 | 未满足时的典型表现 |
|------|--------|-------------------|
| **Gate 跨进程路由**（`routeToRemoteActor`） | `zbus.DefaultBus != nil` | 打日志 `remote bus is not configured`，走无路由回退 |
| **Actor `SendMsg` / `Broadcast` 到远程** | `zbus.DefaultBus != nil` | 返回「bus 未配置」类错误 |
| **Actor 订阅跨进程 topic**（`pubsub`） | `zbus.DefaultBus != nil` | 订阅跳过并打 warn |
| **使用 znats 默认连接池**（与 `NewDefaultNats` 配套） | `znats.DefaultNatsClient != nil` 且已 `Connect` | Publish/Subscribe 失败 |
| **多进程发现（Etcd 等）** | `Group.SetDiscoverer(d)` 在路由前完成 | `Find*` 无数据、远程路由失败 |
| **zstartup 拉起多 Actor** | 每个 `ActorType` 均已 `RegisterActorFactory` | `InitActors: no ActorFactory registered` |

**建议**：在 `main` 或统一 `Init` 中，对「本进程一定会用到的全局依赖」做显式判断并 `log.Fatal` / 返回错误，避免运行期才失败。跨进程场景下典型顺序示例：

1. `znats.NewDefaultNats(url, poolSize)`（内部会设置 `DefaultNatsClient` 与 `zbus.DefaultBus`）
2. `DefaultNatsClient.Connect(ctx)`
3. **`zcheck.Validate(zcheck.Config{...})`**（见包 [zcheck](../zcheck/README.md)）
4. 创建 Gate / Group，`SetDiscoverer`（若需要）
5. `gate.Init` / `RunServer`

---

## 二、全局变量（按包）

### zbus

| 符号 | 类型 | 说明 |
|------|------|------|
| `DefaultBus` | `TopicBus` | 跨进程消息总线实现；**默认 nil**，需注入（常见方式：`znats.NewDefaultNats(...)` 自动注入，或手动 `zbus.DefaultBus = znats.NewNatsBus(pool)`） |

### znats

| 符号 | 类型 | 说明 |
|------|------|------|
| `DefaultNatsClient` | `*NatsPool` | 由 `NewDefaultNats` 内 `sync.Once` 初始化；同时设置 `zbus.DefaultBus` |
| `DefaultMaxRetries` / `DefaultRetryDelay` | 常量 | 连接重试参数 |
| `DefaultURL` | 常量 | 默认 NATS 地址字符串 |

### zmetrics

| 符号 | 类型 | 说明 |
|------|------|------|
| `globalRegistry` | `*Registry` | 包 init 时创建；`Global()` 始终非 nil |
| `DefaultLatencyBounds` | `[]float64` | 默认直方图桶（毫秒） |
| `framework.go` 中各 `Counter`/`Gauge`/`Histogram` | 预注册指标 | 通过 `Global()` 懒注册到 `globalRegistry` |
| `runtime.go` 中 `GoMem*` / `GoGC*` 等 | 预注册指标 | 需 `StartRuntimeCollector` 才会被周期更新 |
| `HandlerSlowLogThreshold` | `time.Duration` | Handler 慢调用阈值，启动期可覆盖 |

### zmodel

| 符号 | 类型 | 说明 |
|------|------|------|
| `DefaultFrameworkTuning` | `FrameworkTuning` | Actor 批处理、默认 WorkSize 等；运行期只读语义 |
| `frameworkTuningValue` | `atomic.Value` | `SetFrameworkTuning` 写入 |

### zpoolobs

| 符号 | 类型 | 说明 |
|------|------|------|
| 全局 `atomic.Pointer[observerHolder]` | 内部 | `SetObserver(nil)` 关闭观测 |
| `GetObserver` / `SetObserver` | 函数 | 对象池 `IPoolObserver`（默认由 `zmetrics.Enable` 装 `GlobalPoolObserver`）；**可选**自定义覆盖 |

### zactor（包级，非 Actor 实例）

| 符号 | 说明 |
|------|------|
| `traceEnabled` 等 | 由 `SetTraceHooks` 注入，默认全链路关闭 |
| `asyncTaskPool` / `asyncTaskPoolOnce` | `handlemsg.go`，异步任务池 |
| `DefaultMaxPendingRPCs` 等 | Sender 默认参数 |

### zmsg

| 符号 | 说明 |
|------|------|
| `messagePool` / `messagePoolOnce` | `GetMessage` 对象池 |
| `DEBUG_LIFECYCLE` | 构建标签 `debug_lifecycle` 时行为不同（默认 off） |

### zmonitor

| 符号 | 说明 |
|------|------|
| `systemMonitorCache` | `CollectSystemMonitor` 缓存；间隔由 `SetSystemMonitorCacheInterval` 控制 |

---

## 三、钩子与注入点（按模块）

### zgate.Server（`zgate`）

| 方法 | 时机 | 用途 |
|------|------|------|
| `SetRemoteRouteStrategy` | Run 前 | 远程首选选址（`PickOne`，HRW / 轮询等） |
| `SetHTTPAddr` | Run 前 | 启用内置 HTTP |
| `OnNoRoute` | Run 前 | 无路由时自定义回包或日志 |
| `SetTraceHook` | Run 前 | Gate 收包时改写 trace 字段 |
| `OnChannelClose` | Run 前 | 连接关闭回调 |
| `OnAccept` | 已实现于 Server，可配合 `channel.SetCloseCall` 等 | 限流、鉴权等（在 `gate.go` 内与 `OnRead` 同链路） |

底层 `IServer` 的 `OnAccept` / `OnRead` 在 `RunServer` 时绑定到本 `Server`。

### zactor.Actor

| 方法 | 时机 | 用途 |
|------|------|------|
| `SetGroup` | 加入 Group 后 | 本地/远程路由、`SendMsg` |
| `SetIActor` | 尽早（如 Gate `NewServer` 内） | 缓存 `IToClientFastPath` 等 |
| `SetPoolObserver` | Sender 等初始化前 | 覆盖默认池观测 |
| `RegisterTickFn` | Run 前 | 周期 Tick |
| `SetInitServer` | Run 前 | `Init` 阶段自定义逻辑 |
| `GetHandleMgr().RegisterHandle` / Dispatcher | Run 前 | 业务消息处理 |

### zactor（包函数）

| 函数 | 时机 | 说明 |
|------|------|------|
| `SetTraceHooks` | **任意 Actor Run 之前，且仅生效一次**（`sync.Once`） | 与 `ztrace` 集成 |

### zpoolobs

| 函数 | 说明 |
|------|------|
| `SetObserver` | 全局池观测（中继转发）；`zmsg`/`zactor` 等创建池时会经 relay 调当前 observer |

### zmodel

| 函数 | 说明 |
|------|------|
| `SetFrameworkTuning` | 替换默认调优；建议在进程早期调用 |

### zmetrics

| 函数 | 说明 |
|------|------|
| `StartRuntimeCollector(ctx, interval)` | 启动 Go MemStats/GC 等指标采集；**全局只启动一次**（`sync.Once`） |

### zmonitor

| 函数 | 说明 |
|------|------|
| `SetSystemMonitorCacheInterval` | 控制 `CollectSystemMonitor` 缓存刷新间隔；`0` 表示每次新鲜采样 |

### zstartup.App

| 方法 | 说明 |
|------|------|
| `RegisterActorFactory` | 每个要拉起的 `ActorType` 必须注册，否则 `initActors` 失败 |

---

## 四、可选 / 调参全局项

- **zmetrics**：自定义指标通过 `Global().Counter/Gauge/Histogram` 注册；`WritePrometheus` 导出全表。
- **脚本引擎包**（`zjs` / `zlua` / `zstarlark` / `ztengo`）：各引擎内部有默认配置与 VM 级 `SetGlobal`，按包 README 使用。
- **zdiscovery**：无包级单例；每个进程持有自己的 `EtcdDiscovery` 实例并 `SetDiscoverer` 注入 Group。

---

## 五、zcheck 包（推荐）

| API | 说明 |
|-----|------|
| `zcheck.Validate(cfg)` | 按 [Config](../zcheck/check.go) 校验，失败返回 `errors.Join` 合并错误 |
| `zcheck.ValidateOrPanic(cfg)` | 同上，失败 panic |

---

## 六、维护说明

新增以下类型 API 时，请同步更新本文档与 `zcheck` 实现（若适用）：

- 新的 `var Default*` 或「进程单例」
- 新的 `Set*` / `Register*` / `On*` 且约定「必须在启动期调用一次」的钩子
