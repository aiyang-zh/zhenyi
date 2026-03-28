# 编解码适配器（实现 `ziface.IMessage`）

`zhenyi` 的业务消息最终都会落到 `zmsg.Message.Data`（`[]byte`）上；因此“编解码”在架构上应当通过 **`ziface.IMessage` 适配器**来接入。

建议优先使用仓库内置的标准适配器实现：`zcodec.JSONMessage` / `zcodec.BytesMessage` / `zcodec.MsgpackMessage`（均实现 `ziface.IMessage`）。  
若你需要自定义编码方式或组织结构，本页仍提供一套“标准适配器模板”（JSON / bytes / msgpack），你可以按同一契约实现自己的类型，不会影响框架本身与既有调用方式。

> 说明：以下为示例模板，字段与类型可按你的业务改造；是否追求零拷贝/对象池属于性能优化选择。
>
> 建议组织方式：将三种适配器分别放在独立文件，例如：
> `messages/json_message.go`、`messages/bytes_message.go`、`messages/msgpack_message.go`（与 demo 的“业务自定义消息适配器”写法一致）。

## 适配契约

业务消息类型需要实现 `ziface.IMessage`（见 `ziface/imessage.go`）：

- `GetMsgId() int32`
- `MarshalVT() ([]byte, error)`
- `UnmarshalVT([]byte) error`
- `MarshalToVT(dst []byte) (int, error)`（零/低分配序列化到外部 buffer）
- `SizeVT() int`

框架在 `SendActor/CallActor/SendToClient` 等路径上会调用 `MarshalToVT`，并把结果写入 `zmsg.Message.Data`。

## 标准实现模板（JSON）

把下面代码复制到你的业务工程任意位置即可（建议放到 `codec/` 或 `messages/` 目录）。

```go
package messages

import "encoding/json"

// JSONMessage 使用 json 编解码实现 ziface.IMessage。
// 典型用法：用于 SendActor/SendToClient 的请求/响应消息体。
type JSONMessage struct {
	msgID int32
	data  []byte
}

func NewJSONMessage(msgID int32, v any) (*JSONMessage, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return &JSONMessage{
		msgID: msgID,
		data:  b,
	}, nil
}

func (m *JSONMessage) GetMsgId() int32 { return m.msgID }

func (m *JSONMessage) UnmarshalVT(b []byte) error {
	// 保存一份数据，避免调用方复用底层切片导致内容被改写。
	m.data = append(m.data[:0], b...)
	return nil
}

func (m *JSONMessage) MarshalVT() ([]byte, error) {
	// MarshalVT 语义上返回新切片；更高性能可只在需要时用 MarshalToVT。
	return append([]byte(nil), m.data...), nil
}

func (m *JSONMessage) MarshalToVT(dst []byte) (int, error) {
	return copy(dst, m.data), nil
}

func (m *JSONMessage) SizeVT() int { return len(m.data) }

// Decode 把当前 data 反序列化到 out（便于业务侧直接解码）。
func (m *JSONMessage) Decode(out any) error {
	return json.Unmarshal(m.data, out)
}
```

## 标准实现模板（bytes）

当你的 payload 本身就是 `[]byte`（例如你已经有自定义二进制格式）时，用 bytes adapter 最省事。

```go
package messages

// BytesMessage 透明透传 []byte，实现 ziface.IMessage。
type BytesMessage struct {
	msgID int32
	data  []byte
}

func NewBytesMessage(msgID int32, b []byte) *BytesMessage {
	// 复制一份，避免外部 b 被复用导致消息内容变化。
	c := append([]byte(nil), b...)
	return &BytesMessage{msgID: msgID, data: c}
}

func (m *BytesMessage) GetMsgId() int32 { return m.msgID }
func (m *BytesMessage) SizeVT() int    { return len(m.data) }

func (m *BytesMessage) UnmarshalVT(b []byte) error {
	m.data = append(m.data[:0], b...)
	return nil
}

func (m *BytesMessage) MarshalVT() ([]byte, error) {
	return append([]byte(nil), m.data...), nil
}

func (m *BytesMessage) MarshalToVT(dst []byte) (int, error) {
	return copy(dst, m.data), nil
}
```

## 标准实现模板（msgpack）

msgpack 适用于需要比 JSON 更紧凑的二进制编码场景。

```go
package messages

import "github.com/vmihailenco/msgpack/v5"

type MsgpackMessage struct {
	msgID int32
	data  []byte
}

func NewMsgpackMessage(msgID int32, v any) (*MsgpackMessage, error) {
	b, err := msgpack.Marshal(v)
	if err != nil {
		return nil, err
	}
	return &MsgpackMessage{
		msgID: msgID,
		data:  b,
	}, nil
}

func (m *MsgpackMessage) GetMsgId() int32 { return m.msgID }
func (m *MsgpackMessage) SizeVT() int    { return len(m.data) }

func (m *MsgpackMessage) UnmarshalVT(b []byte) error {
	m.data = append(m.data[:0], b...)
	return nil
}

func (m *MsgpackMessage) MarshalVT() ([]byte, error) {
	return append([]byte(nil), m.data...), nil
}

func (m *MsgpackMessage) MarshalToVT(dst []byte) (int, error) {
	return copy(dst, m.data), nil
}

func (m *MsgpackMessage) Decode(out any) error {
	return msgpack.Unmarshal(m.data, out)
}
```

## 如何接入你的业务（不影响框架）

1. **发送给其他 Actor / 请求-响应（RPC）**：把请求/响应消息实现为 `ziface.IMessage`，然后直接传给 `CallActor`、`SendActor`、`SendToClient` 等 API。
2. **在 Handler 中接收**：框架把 payload 放在 `msg.Data`（`[]byte`）。你可以按你的协议使用 `json.Unmarshal/msgpack.Unmarshal` 等进行解码。

如果你不想使用上述模板，按同一契约实现你自己的适配器即可；框架与 `ziface.IMessage` 契约不变。

