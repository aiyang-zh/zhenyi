# zactor

**Actor 运行时**：zhenyi 的核心执行单元，提供基于 MPSC 邮箱的轻量级 Actor 模型。

## 简介

zactor 是 zhenyi 框架的核心模块，实现了高性能 Actor 模型：

- **单 Actor 单邮箱**：每个 Actor 拥有独立的 MPSC 队列
- **协程池执行**：异步任务通过预分配的协程池处理
- **批处理优化**：自适应批量处理，减少上下文切换
- **优雅退出**：关闭时自动排空队列，不丢失消息

## 核心概念

### Actor

Actor 是最小的执行单元，每个 Actor 拥有：
- 独立的 MPSC 邮箱队列
- 专属的协程池
- 注册的消息处理器
- 可选的定时任务（Tick）

### Group

Group 管理多个 Actor，提供：
- Actor 注册与发现
- 消息路由表（msgId → Actor）
- 统一生命周期管理
- Watchdog 阻塞检测

### 消息流程

```
客户端消息 → Gate → ActorCmd → Actor 邮箱 → 批处理 → Handler → 回包
```

## 核心 API

### Actor 创建与初始化

#### `NewActor(cfg ActorConfig) *Actor`

创建 Actor 实例。

```go
cfg := zmodel.ActorConfig{
    Id:       1,
    Name:     "gate",
    ActorType: 1,
    Index:    0,
    WorkSize: 10, // 协程池大小，默认从 FrameworkTuning 读取
}
actor := zactor.NewActor(cfg)
```

**参数说明**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| Id | uint64 | 是 | Actor 唯一标识 |
| Name | string | 是 | Actor 名称（用于日志）|
| ActorType | uint32 | 是 | Actor 类型，用于路由 |
| Index | uint32 | 否 | 同类型 Actor 的序号 |
| WorkSize | int | 否 | 异步协程池大小，默认 0 |

#### `actor.SetIActor(iActor ziface.IActor)`

设置 Actor 的业务实现接口。

```go
actor.SetIActor(myActor)
```

#### `actor.Init(ctx context.Context) error`

初始化 Actor（必须在 Group.Run 之前调用）。

```go
if err := actor.Init(ctx); err != nil {
    log.Fatal(err)
}
```

**注意事项**：
- ctx 必须来自 Group.Run，由 Group 管理生命周期
- Init 会创建 Sender、订阅总线消息

---

### 消息处理器

#### `actor.GetHandleMgr().RegisterHandle(msgId int32, handle Handle)`

注册消息处理器。

```go
actor.GetHandleMgr().RegisterHandle(100, func(ctx context.Context, msg *zmsg.Message) {
    // 处理 msgId=100 的消息
    data := msg.Data
    // ...
    // 回包
    gate.SendToClient(msg, replyData)
})
```

**参数说明**：

| 参数 | 类型 | 说明 |
|------|------|------|
| msgId | int32 | 消息 ID |
| handle | Handle | 消息处理函数 |

**注意事项**：
- 同一个 msgId 不允许重复注册
- Handler 在 Actor 主循环中顺序执行，禁止阻塞操作

#### 禁止在 Handler 中使用的操作

```go
// ❌ 禁止：time.Sleep、同步 I/O、channel 阻塞操作
actor.GetHandleMgr().RegisterHandle(100, func(ctx context.Context, msg *zmsg.Message) {
    time.Sleep(time.Second) // 会阻塞 Actor！
    result := db.Query()     // 同步 I/O 会阻塞！
    <-ch                      // channel 阻塞会阻塞 Actor！
})

// ✅ 正确：使用 AsyncRun 或 AsyncRunWithMsg
actor.GetHandleMgr().RegisterHandle(100, func(ctx context.Context, msg *zmsg.Message) {
    actor.AsyncRun(func() interface{} {
        // 异步执行，不阻塞 Actor
        return db.Query()
    }, nil)
})
```

---

### 异步执行

#### `actor.AsyncRun(f func() interface{}, callBackFunc func(interface{}), validators ...func() bool)`

在协程池中异步执行函数。

```go
actor.AsyncRun(func() interface{} {
    // 异步执行，不阻塞 Actor 主循环
    return heavyWork()
}, func(result interface{}) {
    // 回到 Actor 主线程处理结果（可选，传 nil 表示不需要）
    useResult(result)
})
```

#### `actor.AsyncRunWithMsg(msg *zmsg.Message, f func(*zmsg.Message) interface{}, callBackFunc func(interface{}), validators ...func() bool)`

带消息上下文的异步执行：`f` 在 worker 协程执行，`callBackFunc` 回到 Actor 主线程执行。

```go
actor.AsyncRunWithMsg(msg, func(msg *zmsg.Message) interface{} {
    // 在 worker 协程中执行（不要直接改 Actor 内部状态）
    return doWork(msg.Data)
}, func(result interface{}) {
    // 回到 Actor 主线程，安全更新状态
    applyResult(result)
}, func() bool {
    // 可选：前置/回调前条件检查
    return canProcess(msg)
})
```

---

### 定时任务

#### `actor.RegisterTickFn(name string, interval time.Duration, f func(ctx, nowTs))`

注册定时任务。

```go
actor.RegisterTickFn("heartbeat", 5*time.Second, func(ctx context.Context, nowTs int64) {
    // 每 5 秒执行一次
    checkConnections()
})
```

**参数说明**：

| 参数 | 类型 | 说明 |
|------|------|------|
| name | string | 任务名称（唯一标识）|
| interval | time.Duration | 执行间隔 |
| f | func | 定时执行函数 |

**注意事项**：
- 同一个 name 只允许注册一次
- Tick 在 Actor 主循环中执行，禁止阻塞

---

### RPC 调用

#### `actor.CallActor(ctx, targetActorId, msg) (*zmsg.Message, error)`

同步调用其他 Actor。

```go
// 调用其他 Actor 并等待响应
reply, err := actor.CallActor(ctx, targetActorId, reqMsg)
if err != nil {
    // 处理错误（超时/熔断/网络）
    return
}
defer reply.Release()
```

**参数说明**：

| 参数 | 类型 | 说明 |
|------|------|------|
| ctx | context.Context | 上下文，包含超时 |
| targetActorId | uint64 | 目标 Actor ID |
| msg | *zmsg.Message | 请求消息 |

**返回值**：

| 类型 | 说明 |
|------|------|
| *zmsg.Message | 响应消息，需调用 Release() |
| error | 错误（超时/熔断/未找到）|

#### `actor.SendMsg(msg *zmsg.Message)`

发送异步消息（不等待响应）。

```go
msg := zmsg.GetMessage()
defer msg.Release()
msg.MsgId = 100
actor.SendMsg(msg)
```

---

### 熔断器

#### `actor.GetCircuitBreaker(targetActorId uint64) *circuitBreaker`

获取对目标 Actor 的熔断器。

```go
cb := actor.GetCircuitBreaker(targetActorId)
if !cb.Allow() {
    // 熔断开启，快速失败
    return
}
```

**熔断策略**：
- 连续失败 5 次后触发熔断
- 熔断持续 10 秒
- 10 秒后进入半开状态，允许一个请求试探

---

### 热更新

#### `actor.UpdateWorkerPoolSize(newSize int)`

运行时调整协程池大小。

```go
// 将协程池从 10 调整为 20
actor.UpdateWorkerPoolSize(20)
```

#### `actor.UpdateRateLimit(rate, burst int)`

运行时调整限流参数。

```go
actor.UpdateRateLimit(100, 200)
```

---

### Watchdog（阻塞检测）

#### `group.EnableWatchdog(threshold time.Duration)`

启用 Actor 阻塞检测。

```go
group := zactor.NewGroup(1, true)
group.EnableWatchdog(100 * time.Millisecond) // 检测超过 100ms 的 Handler
```

**效果**：
- Handler 执行超过阈值时，捕获 goroutine stack trace
- 输出警告日志，便于定位阻塞

---

### Group 管理

#### `NewGroup(process uint, isSingle bool) *Group`

创建 Group。

```go
group := zactor.NewGroup(1, false) // process=1, 非单机模式
```

| 参数 | 类型 | 说明 |
|------|------|------|
| process | uint | 进程编号 |
| isSingle | bool | 是否单机模式（单机模式不使用服务发现）|

#### `group.AddActor(iActor ziface.IActor)`

添加 Actor 到 Group。

```go
group.AddActor(actor)
```

#### `group.RegisterRoutes(actor, msgIDs)`

注册 Actor 支持的消息列表。

```go
group.RegisterRoutes(actor, []int32{100, 101, 102})
```

#### 路由快路径扩展（可选）

`IGroup` 默认通过 `LookupActorsByMsgID(msgID)` 返回候选副本，调用方可安全修改。  
若 Group 实现了 `ziface.IGroupRouteTableView`，路由层会优先调用：

```go
LookupActorsByMsgIDView(msgID int32) []ziface.IActor
```

该方法用于热路径零分配读取，返回切片必须视为**只读视图**，调用方不得修改；需要可变结果时仍应使用 `LookupActorsByMsgID`。

#### `group.Run(ctx context.Context) error`

启动 Group，运行所有 Actor。

```go
if err := group.Run(ctx); err != nil {
    log.Fatal(err)
}
```

#### `group.WaitForDrain()`

等待所有 Actor 排空退出。

```go
group.WaitForDrain() // 阻塞直到所有 Actor 优雅退出
```

---

## 使用示例

### 方式一：通过 zstartup.App（推荐）

这是推荐的使用方式，通过 App 统一管理 Actor 生命周期。

```go
package main

import (
    "context"
    "log"

    "github.com/aiyang-zh/zhenyi/zgate"
    "github.com/aiyang-zh/zhenyi/zstartup"
    "github.com/aiyang-zh/zhenyi/ziface"
    "github.com/aiyang-zh/zhenyi/zmodel"
    "github.com/aiyang-zh/zhenyi/zmsg"
    "github.com/aiyang-zh/zhenyi/znet"
)

const (
    ActorTypeGate uint32 = 1
    ActorTypeIM   uint32 = 2
)

const (
    MsgChatReq int32 = 100
)

func main() {
    ctx := context.Background()

    // 创建 App
    app := zstartup.NewApp(ctx, zstartup.AppConfig{
        Process:  1,
        IsSingle: true,  // 单机模式，不使用服务发现
        ConnType: 1,     // TCP 连接
        Actors: []zmodel.ActorConfig{
            {
                Id:        1,
                ActorType: ActorTypeGate,
                Name:      "gate",
                Index:     1,
                Addr:      "127.0.0.1:8001",
                Process:   1,
            },
        },
    })

    // 注册 Actor 工厂
    app.RegisterActorFactory(ActorTypeGate, func(a *zstartup.App, c zmodel.ActorConfig) ziface.IServerActor {
        s := zgate.NewServer(c, a.ConnType)
        s.GetHandleMgr().RegisterHandle(MsgChatReq, func(ctx context.Context, msg *zmsg.Message) {
            // 处理聊天消息
            log.Printf("收到消息: %s", string(msg.Data))
        })
        return s
    })

    // 启动 App
    app.Run()
}
```

### 方式二：直接使用 zgate

如果只需要一个网关 Actor，可以直接创建。

```go
cfg := zmodel.ActorConfig{
    Id:        1,
    Name:      "gate",
    ActorType: 1,
    Addr:      "127.0.0.1:8001",
}
gate := zgate.NewServer(cfg, znet.TCP)

gate.GetHandleMgr().RegisterHandle(100, func(ctx context.Context, msg *zmsg.Message) {
    // 处理消息
})

gate.Init(ctx)
gate.RunServer(ctx)
```

### 跨进程 RPC

```go
// 在 Actor 中调用远程 Actor
reply, err := actor.CallActor(ctx, remoteActorId, reqMsg)
if err != nil {
    log.Printf("RPC 失败: %v", err)
    return
}
defer reply.Release()

// 处理响应
processReply(reply)
```

---

## 生命周期

```
创建 Actor → SetIActor → RegisterHandle → AddActor → RegisterRoutes → Group.Run
                                                                         
                                                                         
关闭：Group.Close → Actor 排空退出 → 释放资源
```

### 优雅退出

1. 收到关闭信号（ctx.Done 或 closeCh）
2. 标记 shouldExit = true
3. 继续处理队列中的消息
4. 队列为空时退出
5. 释放协程池

---

## 性能特性

| 特性 | 说明 |
|------|------|
| **MPSC 队列** | 无锁队列，高性能 |
| **批处理** | 自适应批量，减少上下文切换 |
| **协程池** | 预分配，减少 goroutine 创建开销 |
| **零拷贝** | 消息对象池，复用减少分配 |

---

## 依赖

| 模块 | 说明 |
|------|------|
| zmodel | ActorConfig、ActorCmd、Message |
| ziface | 接口定义 |
| zmsg | 消息封装 |
| zmetrics | 监控指标 |
| zmonitor | 状态监控 |
| zbus/znats | 跨进程消息 |
| zlog | 日志（zhenyi-base）|
| ants | 协程池 |

---

## 注意事项

### 1. Handler 禁止阻塞

```go
// ❌ 错误
RegisterHandle(100, func(ctx, msg) {
    time.Sleep(time.Second) // 阻塞整个 Actor！
})

// ✅ 正确
RegisterHandle(100, func(ctx, msg) {
    actor.AsyncRun(func() {
        time.Sleep(time.Second) // 异步执行
    })
})
```

### 2. Tick 禁止阻塞

Tick 在 Actor 主循环中执行，与 Handler 相同约束。

### 3. 消息必须 Release

```go
msg := zmsg.GetMessage()
defer msg.Release() // 防止内存泄漏
```

### 4. RPC 需要设置超时

```go
ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()
reply, err := actor.CallActor(ctx, targetId, msg)
```

---

## 相关模块

| 模块 | 说明 |
|------|------|
| zgate | 网关，接收客户端连接 |
| zmsg | 消息格式与对象池 |
| zbus/znats | 跨进程消息总线 |
| zroute | 路由策略 |
| zdiscovery | 服务发现 |
