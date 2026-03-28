# im_multi_demo

Multi-process version based on `im_single_demo`:
- `process=1` only starts Gate (listens for client connections)
- `process=2` only starts IM Actor (handles join/leave/send)
- Gate and IM use NATS + Etcd for cross-process routing
- Built-in Actor RPC example: Gate uses `AsyncRunWithMsg + CallActor(IM)` to asynchronously query IM node info during `login`, and returns `imNode` to client (to avoid blocking Gate main thread)

## Prerequisites

Requires locally available:
- NATS (default `nats://127.0.0.1:4222`)
- Etcd (default `127.0.0.1:2379`)

## How to Start

Start two processes separately in `zhenyi/examples/im_multi_demo` directory:

```bash
# Process 1: Gate
go run ./examples/im_multi_demo --process=1 --addr=127.0.0.1:8001 --nats=nats://127.0.0.1:4222 --etcd=127.0.0.1:2379
```

```bash
# Process 2: IM
go run ./examples/im_multi_demo --process=2 --nats=nats://127.0.0.1:4222 --etcd=127.0.0.1:2379
```

Clients can continue to use `im_single_client` to connect to `127.0.0.1:8001`.

## Load Test Related Parameters

- `-codec`: Message codec (`json` or `msgpack`, default `json`)
- `-benchMode`: Load test mode (`business` or `framework`, default `business`)

Example (recommended framework baseline load test):

```bash
# Gate
go run ./examples/im_multi_demo --process=1 --addr=127.0.0.1:8001 --nats=nats://127.0.0.1:4222 --etcd=127.0.0.1:2379 --codec=msgpack --benchMode=framework
```

```bash
# IM
go run ./examples/im_multi_demo --process=2 --nats=nats://127.0.0.1:4222 --etcd=127.0.0.1:2379 --codec=msgpack --benchMode=framework
```
