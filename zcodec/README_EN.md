# zcodec

**Message Codec Adapter**: Provides standard `ziface.IMessage` implementations for business payloads, convenient for direct integration into zhenyi's send/receive paths.

## Standard Implementations (Recommended Priority Use)

| Type | Encoding | Applicable Scenario |
|------|----------|---------------------|
| `JSONMessage` | JSON | Debug-friendly, lightweight protocols |
| `BytesMessage` | Raw bytes | Custom binary protocols |
| `MsgpackMessage` | msgpack | More compact binary protocols |

These types all implement `ziface.IMessage`, so the framework automatically calls `MarshalToVT/SizeVT` on paths like `SendActor/CallActor/SendToClient` and writes to `zmsg.Message.Data`.

If you don't like the built-in adapters, you can implement your own `ziface.IMessage` (contract unchanged) without affecting the framework body.

More adapter templates and extension instructions see [`docs/CODEC_ADAPTERS.md`](../docs/CODEC_ADAPTERS.md).
