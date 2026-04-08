# Examples Overview

This document summarizes the runnable examples in the repository and their purposes.

## 1. Single Node Minimal

### `examples/echo_demo`

- Purpose: Minimal runnable example (Gate access + one business Actor handling one request/response)
- Suitable for: Running the full path in 5 minutes, troubleshooting environment issues, understanding the message handling model

Run server:

```bash
go run ./examples/echo_demo -conn tcp -addr 127.0.0.1:8021
```

In another terminal, run client:

```bash
go run ./examples/echo_client -addr 127.0.0.1:8021 -text "hello zhenyi"
```

### `examples/im_single_demo`

- Purpose: Single-node Gate + Actor running example
- Suitable for: Quickly verifying local environment and message flow
- Common flags:
  - `--reactor`: enable TCP reactor read path (TCP only, no TLS/GM-TLS)
  - `--sharedSendWorker`: enable shared send workers on the Gate (ztcp/zws/zkcp; compatible with or without reactor)

Run:

```bash
go run ./examples/im_single_demo
```

Shared send + reactor:

```bash
go run ./examples/im_single_demo --reactor --sharedSendWorker
```

### `examples/im_single_demo_bench`

- Purpose: single-node benchmark/tuning example (connections, QPS, tail latency)
- Suitable for: framework-path benchmarking, reactor/shared-send tuning, pprof/pyroscope observation
- Note: benchmark-oriented; not equivalent to full chat-room semantics

Run:

```bash
go run ./examples/im_single_demo_bench --reactor --sharedSendWorker --benchMode framework --codec msgpack
```

### `examples/im_single_client`

- Purpose: Minimal client paired with single-node demo
- Suitable for: Verifying server send/receive and response

Run:

```bash
go run ./examples/im_single_client
```

## 2. Multi-Process Examples

### `examples/im_multi_demo`

- Purpose: Multi-process Actor/Gate collaboration example
- Dependencies: Typically requires Etcd + NATS
- Suitable for: Verifying remote routing and cross-process messaging
- Common flags (Gate process): `--reactor`, `--sharedSendWorker` (same semantics as `im_single_demo`)

It is recommended to verify external dependencies are reachable before starting:

```bash
go run ./examples/im_multi_demo
```

### `examples/im_multi_client_load`

- Purpose: Multi-client load testing / concurrent request example
- Suitable for: Basic throughput and stability verification

Run:

```bash
go run ./examples/im_multi_client_load
```

## 3. Browser MMO Example

### `examples/mmo_web_demo`

- Purpose: Minimal MMO-style sample (position sync + simple combat/respawn + **`zaoi` AOI-filtered broadcast**)
- Components: Go server + HTML/JS WebSocket client
- Suitable for: Verifying browser access and in-room state fan-out

Server:

```bash
go run ./examples/mmo_web_demo -conn ws -addr 127.0.0.1:8001
```

Open in browser:

`http://127.0.0.1:8080/mmo_web_demo/web/`

Notes:

- The example starts a built-in static file server by default (`-web 127.0.0.1:8080`), no Python required.
- The static server uses `./examples` as web root by default (`-webRoot` can override), so demos can share the browser SDK at `/_shared/web/zhenyi-ws-sdk.js`.

## 4. Example Usage Suggestions

- First run single-node (`echo_demo` or `im_single_demo` + `im_single_client`)
- Then try browser access (`mmo_web_demo`)
- Then switch to multi-process (`im_multi_demo`)
- Finally use `im_multi_client_load` for stress testing

## 5. Common Issues

- If cross-process messages don't work, first check `znats` / `zbus` initialization and connection status
- If routing is abnormal, check `ActorType`, routing registration, and discoverer injection order
- If no metrics, confirm whether `zmetrics.Enable(...)` has been called
