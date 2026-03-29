# zhenyi Beginner's Guide

This document is for developers who are new to `zhenyi`, with the goal of getting a working minimal real-time application service running in the shortest path.

## 1. Prerequisites

- Go version: follow root `go.mod` (see badge in `README.md`)
- Git installed
- Local available port (default `9001` in examples)

## 2. Get Code and Basic Verification

```bash
git clone https://github.com/aiyang-zh/zhenyi.git
cd zhenyi
go test ./... -count=1
```

If tests pass, dependencies and environment are ready.

## 3. Run Minimal Example (Single Node)

```bash
go run ./examples/im_single_demo
```

You will get a single-node real-time application example service based on `zgate + zactor`.

To verify with a client, open another terminal and run:

```bash
go run ./examples/im_single_client
```

## 4. Minimal Code Skeleton

Core flow: Create Gate -> Initialize -> Run.

```go
cfg := zmodel.ActorConfig{
    Id: 1, Name: "gate", ActorType: 1, Index: 0, Host: "0.0.0.0", Port: 9001,
}
gate := zgate.NewServer(cfg, znet.TCP)
_ = gate.Init(ctx)
_ = gate.RunServer(ctx)
```

Recommended to complete before `Init/Run`:

- `RegisterHandle` for business message handling
- Optional `SetHTTPAddr` to enable HTTP
- Optional `SetTLSConfig`/`SetGMTLS` to enable TLS/GM-TLS

## 5. Cross-Process Basics

Cross-process routing depends on message bus and service discovery; common combinations are:

- `znats` as default bus
- `zdiscovery` (e.g., etcd) as discovery layer

Before starting, you can use `zcheck.Validate(...)` for dependency self-check to avoid runtime failures.

## 6. Next Steps

- Architecture: [`ARCHITECTURE.md`](ARCHITECTURE.md)
- Module API Navigation: [`MODULE_API.md`](MODULE_API.md)
- Examples Overview: [`EXAMPLES.md`](EXAMPLES.md)
- Monitoring: [`MONITORING_OVERVIEW.md`](MONITORING_OVERVIEW.md)
