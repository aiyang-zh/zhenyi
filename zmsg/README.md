# zmsg

**消息与对象池模块**：定义 `Message`、序列化能力与引用计数生命周期，用于高频消息场景。

## 模块定位

- 提供统一消息载体 `Message`
- 通过对象池与引用计数降低分配与 GC 压力
- 与网关、Actor、总线链路共享同一消息语义

## 核心 API（常用）

| 类型/函数 | 说明 |
|----------|------|
| `Message` | 线协议消息：MsgId、SeqId、AuthId、SrcActor、TarActor、Data、ToClient 等 |
| `GetMessage()` | 从池获取消息（refCount=1） |
| `Retain()` | 引用计数 +1 |
| `Release()` | 引用计数 -1，归零时回收到池 |
| `Marshal` / `MarshalTo` / `MarshalPooled` | 序列化为字节流 |
| `Unmarshal` | 反序列化 |

## 最小用法

```go
m := zmsg.GetMessage()
defer m.Release()
m.MsgId = 100
m.Data = append(m.Data[:0], []byte("hello")...)
```

## 生命周期约定

- 拿到消息对象后，必须在生命周期末尾 `Release`
- 传递给异步/跨协程消费者前先 `Retain`
- 谁 `Retain`，谁负责对应的 `Release`

## 相关文档

- 模型定义：`zmodel/README.md`
- 模块 API 导航：`../docs/MODULE_API.md`
- 监控与对象池指标：`../docs/MONITORING_METRICS.md`
