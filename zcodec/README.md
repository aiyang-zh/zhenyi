# zcodec

**消息编解码适配器**：为业务 payload 提供标准 `ziface.IMessage` 实现，方便直接接入 zhenyi 的发送/接收路径。

## 标准实现（建议优先使用）

| 类型 | 编码方式 | 适用场景 |
|------|----------|------------|
| `JSONMessage` | JSON | 调试友好、轻量协议 |
| `BytesMessage` | 原始 bytes | 自定义二进制协议 |
| `MsgpackMessage` | msgpack | 更紧凑的二进制协议 |

这些类型都实现 `ziface.IMessage`，从而在 `SendActor/CallActor/SendToClient` 等路径上由框架自动调用 `MarshalToVT/SizeVT` 写入 `zmsg.Message.Data`。

如果你不习惯内置适配器，可以自行实现自己的 `ziface.IMessage`（契约不变），不会影响框架本体。

更多适配模板与扩展说明见 [`docs/CODEC_ADAPTERS.md`](../docs/CODEC_ADAPTERS.md)。
