# Codec Adapters (Implementing `ziface.IMessage`)

All business messages in `zhenyi` ultimately reside in `zmsg.Message.Data` (`[]byte`); therefore, "codec" should be integrated through **`ziface.IMessage` adapters** in the architecture.

It is recommended to use the built-in standard adapter implementations: `zcodec.JSONMessage` / `zcodec.BytesMessage` / `zcodec.MsgpackMessage` (all implement `ziface.IMessage`).

If you need custom encoding or structure organization, this page provides a "standard adapter template" (JSON / bytes / msgpack). You can implement your own types following the same contract without affecting the framework or existing callers.

> Note: The following are template examples; fields and types can be modified for your business needs. Whether to pursue zero-copy/object pooling is a performance optimization choice.
>
> Recommended organization: Place the three adapters in separate files, e.g.:
> `messages/json_message.go`, `messages/bytes_message.go`, `messages/msgpack_message.go` (consistent with the "business custom message adapter" approach in demos).

## Adapter Contract

Business message types need to implement `ziface.IMessage` (see `ziface/imessage.go`):

- `GetMsgId() int32`
- `MarshalVT() ([]byte, error)`
- `UnmarshalVT([]byte) error`
- `MarshalToVT(dst []byte) (int, error)` (zero/low-allocation serialization to external buffer)
- `SizeVT() int`

The framework calls `MarshalToVT` on paths like `SendActor/CallActor/SendToClient`, and writes the result to `zmsg.Message.Data`.

## Standard Implementation Template (JSON)

Copy the code below to any location in your business project (recommended: `codec/` or `messages/` directory).

```go
package messages

import "encoding/json"

// JSONMessage uses JSON codec to implement ziface.IMessage.
// Typical usage: request/response message body for SendActor/SendToClient.
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
	// Save a copy to avoid caller reusing underlying slice causing content changes.
	m.data = append(m.data[:0], b...)
	return nil
}

func (m *JSONMessage) MarshalVT() ([]byte, error) {
	// MarshalVT semantically returns a new slice; higher performance can use MarshalToVT only when needed.
	return append([]byte(nil), m.data...), nil
}

func (m *JSONMessage) MarshalToVT(dst []byte) (int, error) {
	return copy(dst, m.data), nil
}

func (m *JSONMessage) SizeVT() int { return len(m.data) }

// Decode deserializes current data to out (for business side to decode directly).
func (m *JSONMessage) Decode(out any) error {
	return json.Unmarshal(m.data, out)
}
```

## Standard Implementation Template (bytes)

When your payload is already `[]byte` (e.g., you have a custom binary format), the bytes adapter is most convenient.

```go
package messages

// BytesMessage transparently passes through []byte, implementing ziface.IMessage.
type BytesMessage struct {
	msgID int32
	data  []byte
}

func NewBytesMessage(msgID int32, b []byte) *BytesMessage {
	// Make a copy to avoid external b being reused causing message content changes.
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

## Standard Implementation Template (msgpack)

msgpack is suitable for scenarios requiring more compact binary encoding than JSON.

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

## How to Integrate with Your Business (Without Affecting Framework)

1. **Sending to other Actors / Request-Response (RPC)**: Implement request/response messages as `ziface.IMessage`, then directly pass to APIs like `CallActor`, `SendActor`, `SendToClient`.
2. **Receiving in Handler**: Framework places payload in `msg.Data` (`[]byte`). You can decode using `json.Unmarshal`, `msgpack.Unmarshal`, etc., according to your protocol.

If you don't want to use the templates above, simply implement your own adapter following the same contract; the framework and `ziface.IMessage` contract remain unchanged.
