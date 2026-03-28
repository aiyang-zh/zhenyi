package zcodec

// zcodec provides standard message adapters for zhenyi payload encoding/decoding.
// zcodec 提供“标准消息适配器”，用于把你的业务 payload 编解码接入 zhenyi。
//
// Core contract: implement `ziface.IMessage`; framework uses these on send path:
// 核心契约：实现 `ziface.IMessage`，框架在发送路径上会调用：
// - `SizeVT()` 决定输出字节大小
// - `MarshalToVT(dst)` 零/低分配地把编码结果写入 `zmsg.Message.Data`
//
// You can also implement `ziface.IMessage` yourself instead of built-in adapters.
// 你如果不习惯内置适配器，可以自行实现 `ziface.IMessage`，不影响框架本身与既有调用方式。
