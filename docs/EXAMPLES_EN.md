# Examples Overview

This document summarizes the runnable examples in the repository and their purposes.

## 1. Single Node Minimal

### `examples/im_single_demo`

- Purpose: Single-node Gate + Actor running example
- Suitable for: Quickly verifying local environment and message flow

Run:

```bash
go run ./examples/im_single_demo
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

## 3. Example Usage Suggestions

- First run single-node (`im_single_demo` + `im_single_client`)
- Then switch to multi-process (`im_multi_demo`)
- Finally use `im_multi_client_load` for stress testing

## 4. Common Issues

- If cross-process messages don't work, first check `znats` / `zbus` initialization and connection status
- If routing is abnormal, check `ActorType`, routing registration, and discoverer injection order
- If no metrics, confirm whether `zmetrics.Enable(...)` has been called
