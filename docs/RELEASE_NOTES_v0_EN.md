# v0.1.0 Release: zhenyi Solution Layer First Open Source

> From base library to complete solution, one command to run distributed IM.

## One-Line Positioning

**zhenyi** is a high-performance Go solution for real-time business (games, IM, IoT, etc.), providing **Actor Model + Unified Gateway (`zgate`) + Distributed Collaboration capabilities**, helping you build observable, scalable real-time backends faster.

## What's in This Release?

### ✅ Actor Runtime (`zactor`)
- Single Actor with single mailbox (concurrency-oriented message-driven processing)
- Message handling, Tick, RPC, Dispatcher extensions
- Provides stable message send/receipt paths for high-throughput scenarios

### ✅ Unified Access Layer (`zgate`, optional `zhttp`)
- Supports TCP/WS/KCP long connections
- Attachable processing chain to Gate, business code reuse
- Optional HTTP service capability (corresponds to `zhttp`)

### ✅ Service Discovery & Routing (`zdiscovery` + `zroute`)
- Etcd / Noop discovery implementations
- Remote routing strategies include consistent hashing (HRW / RendezvousHash), etc.

### ✅ Observability (`zmetrics` / `zmonitor` / `ztrace`)
- Metrics export and health probes
- Structured monitoring snapshots, traceparent parsing and trace pass-through

### ✅ External Bus Integration (`znats` / `zbus`)
- NATS / bus abstraction for cross-process message collaboration

## Stability Verification (Observed Phase, Stress Testing In Progress)

> Note: The following data comes from an observation window of a local stress test (sample data for explaining metric meaning and scale; different environments will vary).

- Scenario: 500 connections / 10K QPS (single-machine)
- Latency (RTT): P50 ~`5.5ms`, P99 ~`34ms`
- Memory: ~`28MB`
- GC: pause ratio `<0.1%`

Complete long-term statistics, and finer P99/P999 percentile and fluctuation details, suggest adding a final report after stress testing completes.

## Quick Start (Minutes)

### 1) Start Server (Gate + IM Actor)
```bash
go run ./examples/im_single_demo
```

### 2) Start Client (Interactive Chat)
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

After starting, enter text in the client console and press Enter to send messages; supports `/join`, `/leave`, `/quit` and other interactive commands.

## Applicable Scenarios

- Game servers: rooms, matching, battle, and other real-time interactions
- IoT platform: device connection, command delivery and receipt
- Real-time IM / social applications: sessions, rooms, broadcasting, and online status
- Engineering / compliance projects: controllable delivery under AGPL + Commercial Dual License

## Known Limitations

- Current documentation and examples are still continuously improving; some advanced scenarios follow module README and source code behavior
- Different transport protocols (TCP/WS/KCP) performance under extreme network conditions needs verification by business scenario stress testing
- Distributed deployment depends on external components (like discovery services, message buses); need to plan high-availability topology yourself

## Upgrade / Migration Suggestions

- When migrating from base layer capabilities, prioritize keeping `ziface` interface contracts unchanged, replace module by module
- First complete message model and routing rules alignment, then introduce cross-process discovery and bus capabilities
- Before release, execute minimum regression: connection establishment, message send/receive, routing fallback, monitoring exposure

## License Guidance

- `zhenyi`: AGPL-3.0 + Commercial Dual License
- `zhenyi-base`: MIT (base capability layer)
- Commercial use should follow formal commercial license terms; specific to repository license files
