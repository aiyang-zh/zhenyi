# zmsg

**Message and Object Pool Module**: Defines `Message`, serialization capability, and reference counting lifecycle, used in high-frequency message scenarios.

## Module Positioning

- Provides unified message carrier `Message`
- Reduces allocation and GC pressure through object pooling and reference counting
- Shares same message semantics with gateway, Actor, and bus链路

## Core API (Commonly Used)

| Type/Function | Description |
|---------------|-------------|
| `Message` | Wire protocol message: MsgId, SeqId, AuthId, SrcActor, TarActor, Data, ToClient, etc. |
| `GetMessage()` | Get message from pool (refCount=1) |
| `Retain()` | Reference count +1 |
| `Release()` | Reference count -1, returns to pool when zero |
| `Marshal` / `MarshalTo` / `MarshalPooled` | Serialize to byte stream |
| `Unmarshal` | Deserialize |

## Minimal Usage

```go
m := zmsg.GetMessage()
defer m.Release()
m.MsgId = 100
m.Data = append(m.Data[:0], []byte("hello")...)
```

## Lifecycle Conventions

- After getting message object, must `Release` at lifecycle end
- Before passing to async/cross-goroutine consumers, `Retain` first
- Who `Retain`s is responsible for corresponding `Release`

## Related Documentation

- Model definitions: `zmodel/README.md`
- Module API navigation: `../docs/MODULE_API.md`
- Monitoring and object pool metrics: `../docs/MONITORING_METRICS.md`
