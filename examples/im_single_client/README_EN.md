# im_single_client

Interactive chat room client example (single connection, manual message input). Pairs with **`examples/im_single_demo`**.

Run the commands below from the **zhenyi module root** (`go run ./examples/...`).

## Start

```bash
go run ./examples/im_single_client
```

## Three layers of “encryption” vs server

When the server changes how traffic is protected, align the client **by layer** (layers are independent):

| Layer | Meaning | Client |
|------|---------|--------|
| Transport | GM-TLS or plain TCP | If the server uses `-gmCert`/`-gmKey` (or dual certs), add **`-gmtls`**; otherwise plain TCP without `-gmtls` |
| GM-TLS cipher suites | ECDHE vs ECC negotiation | **`-gmCipherSuite`**: `default` (library default, ECDHE first) \| `ecdhe` \| `ecc` \| `both` — keep in sync with **`im_single_demo -gmCipherSuite`** when the server narrows suites |
| Wire payload | SM4-GCM on message bodies | **`-payloadEncKey`** must **exactly match** the server (orthogonal to TLS suite choice) |

Certificate verification (self-signed / dev):

- **`-gmInsecure`**: skip verification (local demo only).
- **`-gmRoot <PEM>`**: trust anchor; demo self-signed example:  
  `-gmRoot examples/im_single_demo/testdata/server.pem`
- **`-gmServerName`**: hostname used for TLS verification / SNI (default **`im-single-demo-local`**, matching the CN from `gengmtestcert`). When dialing an **IP** like `127.0.0.1` and the cert has **no IP SAN**, this must match the cert CN or you get `doesn't contain any IP SANs`.

## Common parameters

```bash
go run ./examples/im_single_client \
  -addr 127.0.0.1:8001 \
  -user 10001 \
  -nick alice \
  -room lobby \
  -msgLogin 1 \
  -msgJoin 2 \
  -msgLeave 3 \
  -msgSend 4
```

- `-addr`: Gate address  
- `-user` / `-nick` / `-room`: user and room  
- `-msgLogin` / `-msgJoin` / `-msgLeave` / `-msgSend`: message IDs (must match server handlers)  

GM-TLS and wire encryption:

- `-gmtls`: use GM-TLS (Gate must be configured with GM certs)  
- `-gmInsecure`: skip server certificate verification (self-signed / demo)  
- `-gmRoot`: PEM trust bundle (prefer over `-gmInsecure` when possible)  
- `-gmServerName`: hostname for cert verification / SNI (default `im-single-demo-local`; required to match CN when dialing an IP without IP SANs)  
- `-gmCipherSuite`: `default|ecdhe|ecc|both`, same meaning as `im_single_demo`; must intersect server’s offered suites  
- `-gmInfo`: print negotiated cipher suite after connect (default `true`; use `-gmInfo=false` to disable)  
- `-payloadEncKey`: wire payload SM4-GCM key string; must match server `-payloadEncKey`  

## Pairing with im_single_demo

**1) Plain TCP (no GM-TLS)**

```bash
# Terminal 1
go run ./examples/im_single_demo -addr 127.0.0.1:8001

# Terminal 2
go run ./examples/im_single_client -addr 127.0.0.1:8001
```

**2) GM-TLS + demo self-signed cert (verify server cert)**

```bash
# Terminal 1 (certs under im_single_demo/testdata/; regenerate with cmd/gengmtestcert)
go run ./examples/im_single_demo \
  -addr 127.0.0.1:8001 \
  -gmCert examples/im_single_demo/testdata/server.pem \
  -gmKey examples/im_single_demo/testdata/server.key

# Terminal 2
go run ./examples/im_single_client \
  -addr 127.0.0.1:8001 \
  -gmtls \
  -gmRoot examples/im_single_demo/testdata/server.pem
```

(Default `-gmServerName=im-single-demo-local` matches the demo cert CN; usually no extra flag.)

**3) GM-TLS + skip verify + matching suite / payload key**

```bash
go run ./examples/im_single_client \
  -addr 127.0.0.1:8001 \
  -gmtls -gmInsecure \
  -gmCipherSuite ecdhe \
  -payloadEncKey "your-shared-secret"
```

(`-gmCipherSuite` and `-payloadEncKey` must match `im_single_demo` where applicable.)

## Interactive commands

- Normal line: send chat message  
- `/join <room>`: switch and join room  
- `/leave`: leave current room  
- `/quit`: exit client  

## Web UI (direct Gate WebSocket)

1. Start the server with WebSocket on path `/`:

   ```bash
   go run ./examples/im_single_demo -conn ws -addr 127.0.0.1:8001
   ```

2. Open `examples/im_single_client/web/index.html` in a browser and set WS URL to `ws://127.0.0.1:8001/`.

Framing matches the TCP client: 12-byte big-endian header + JSON body. If the server enables **GM-TLS**, the browser’s WebSocket cannot complete a GM handshake; use this **Go client with `-gmtls`** or plain TCP for GM demos.

## Description

After the client starts, it automatically sends:

- `login`: `{"userId":..., "nickname":...}`
- `join`: `{"room":"..."}`
