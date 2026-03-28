# im_single_demo

A **single-node chat room server** sample built from **Gate** (long-lived connections, optional TLS, routing) and **IM** (rooms and chat). It illustrates **`msgId` routing**, **room broadcast**, optional **GM/TLS**, and an optional **SM3 digest** field on chat payloads.

## Where to run

From the **zhenyi** source tree:

```bash
go run ./examples/im_single_demo
```

## What’s included

| Item | Description |
|------|-------------|
| `main.go` | Gate / IM logic; for chat (`msgId=4`) builds **`chat_broadcast`**, computes an **SM3** digest of the JSON **without** `sign`, then adds **`sign`** (hex) |
| `gencert/` | Generates local SM2 self-signed certs (single or dual) for **GM-TLS** trials |
| `certs/` | Default output for PEM files (or use `-out`); **test keys must not be used in production** |

## Start

```bash
go run ./examples/im_single_demo
```

Default listen **`127.0.0.1:8001`**, transport **TCP**. For the **Web** sample, use **WebSocket**:

```bash
go run ./examples/im_single_demo -conn ws -addr 127.0.0.1:8001
```

Companion CLI:

```bash
go run ./examples/im_single_client -addr 127.0.0.1:8001
```

After connect: login, join a room, then type chat text. Commands: `/join <room>`, `/leave`, `/quit`. See **`examples/im_single_client/README.md`** for client flags.

## Behavior

### Routing

Client messages are routed by **`msgId`**. In this sample, login goes through Gate and chat through IM. Pushes often use **`SeqId=0`**; use JSON **`type`** to tell payloads apart.

### Chat broadcast and SM3 digest (`msgId=4`)

The server builds **`chat_broadcast`** fields **without** `sign`, **`json.Marshal`s** that object, hashes the bytes with **SM3**, hex-encodes the digest into **`sign`**, then sends the full JSON.

**`im_single_client`** removes `sign`, re-marshals, recomputes SM3, and compares. This demonstrates **integrity / content-digest** ideas; it is **not** a PKI non-repudiation signature.

### Optional: wire payload encryption

With **`payloadEncKey`**, the body after the frame header is protected with **SM4-GCM** (key derived via SM3), independent of **GM-TLS**. Client and server must share the **same** key.

### Optional: GM-TLS

See **`certs/README.md`** for cert generation and flags. With `im_single_client -gmtls`, the negotiated suite is printed by default (use `-gmInfo=false` to disable).

---

## Quick try

### Plain TCP (no TLS)

Do not pass cert flags on the server; do not use `-gmtls` on the client.

### GM-TLS (self-signed SM2)

**Single cert:**

```bash
go run ./examples/im_single_demo/gencert
CERT=$(pwd)/examples/im_single_demo/certs/sm2.crt
KEY=$(pwd)/examples/im_single_demo/certs/sm2.key
go run ./examples/im_single_demo -conn tcp -addr 127.0.0.1:8001 -gmCert "$CERT" -gmKey "$KEY"
```

```bash
go run ./examples/im_single_client -gmtls -gmInsecure -addr 127.0.0.1:8001
```

**Dual cert:**

```bash
go run ./examples/im_single_demo/gencert -dual
D=$(pwd)/examples/im_single_demo/certs
go run ./examples/im_single_demo -conn tcp -addr 127.0.0.1:8001 \
  -gmSignCert "$D/sm2_sign.crt" -gmSignKey "$D/sm2_sign.key" \
  -gmEncCert "$D/sm2_enc.crt" -gmEncKey "$D/sm2_enc.key"
```

Do not mix single-cert and dual-cert options.

### Wire SM4-GCM (can combine with GM-TLS)

```bash
go run ./examples/im_single_demo -addr 127.0.0.1:8001 -payloadEncKey 'shared-secret'
go run ./examples/im_single_client -addr 127.0.0.1:8001 -payloadEncKey 'shared-secret'
```

---

## Common server flags

| Flag | Meaning |
|------|---------|
| `-addr` | Listen address (default `127.0.0.1:8001`) |
| `-conn` | `tcp` (default) or `ws` (for WebSocket clients such as `im_single_client/web`) |
| `-gmCert` / `-gmKey` | SM2 single PEM pair; enables **GM-TLS** on Gate |
| `-gmSignCert` … `-gmEncKey` | Dual SM2 PEMs; **all four** required together |
| `-payloadEncKey` | Shared passphrase for wire **SM4-GCM** (must match client) |

---

## Downstream JSON (`msgId` 1–4)

| msgId | Role |
|------|------|
| 1 | Login / `login_ack` |
| 2 | Join / `join_ack`, `room_notify` (join) |
| 3 | Leave / `leave_ack`, `room_notify` (leave) |
| 4 | Chat / **`chat_broadcast`** (`fromSessionId`, `nickname`, `text`, **`sign`** = SM3 hex of JSON without `sign`) |

Others in the room may receive pushes (`SeqId=0`); distinguish by **`type`**.

---

## What this sample is for

- A minimal runnable chat room server
- Gate vs business split, **`msgId`** routing, broadcast fan-out
- Optional **GM-TLS**, **payload SM4-GCM**, **SM3 digest** on broadcast JSON

## Other clients

- Interactive: `go run ./examples/im_single_client`
- Load test: `go run ./examples/im_multi_client_load`
