# im_multi_client_load

Chat room concurrent load test client example (multi-connection auto login/join/send messages).

## Start

From the **zhenyi repository root** (after clone):

```bash
go run ./examples/im_multi_client_load
```

## Common Parameters

```bash
go run ./examples/im_multi_client_load \
  -addr 127.0.0.1:8001 \
  -room lobby \
  -clients 100 \
  -intervalMs 200 \
  -durationS 43200 \
  -prefix bot \
  -msgLogin 1 \
  -msgJoin 2 \
  -msgSend 4 \
  -codec msgpack \
  -benchMode framework
```

- `-addr`: Gate address
- `-room`: Load test room name
- `-clients`: Concurrent connections
- `-intervalMs`: Message sending interval per client (milliseconds)
- `-durationS`: Load test duration (seconds)
- `-prefix`: Nickname prefix (example: `bot_1`, `bot_2`)
- `-msgLogin`: Login request message ID
- `-msgJoin`: Join room request message ID
- `-msgSend`: Send room message request ID
- `-codec`: Message codec (`json` or `msgpack`, default `json`)
- `-benchMode`: Load test mode (`business` or `framework`, default `business`)

## Recommended Framework Load Test Commands

Goal: Maximize framework chain capability, reduce business logic interference.

### 1) Server (both processes use same codec/benchMode)

```bash
# Process 1: Gate
go run ./examples/im_multi_demo \
  -process 1 \
  -addr 127.0.0.1:8001 \
  -nats nats://127.0.0.1:4222 \
  -etcd 127.0.0.1:2379 \
  -codec msgpack \
  -benchMode framework
```

```bash
# Process 2: IM
go run ./examples/im_multi_demo \
  -process 2 \
  -nats nats://127.0.0.1:4222 \
  -etcd 127.0.0.1:2379 \
  -codec msgpack \
  -benchMode framework
```

### 2) Client (your 500 concurrent 12-hour parameters)

```bash
go run ./examples/im_multi_client_load \
  -addr 127.0.0.1:8001 \
  -room lobby \
  -clients 500 \
  -intervalMs 200 \
  -durationS 43200 \
  -prefix bot \
  -msgLogin 1 \
  -msgJoin 2 \
  -msgSend 4 \
  -codec msgpack \
  -benchMode framework
```

## Control Experiment Commands (Recommended)

After fixing concurrency parameters, run in order of the following four groups, compare `RTT_P99` and slow warn count in `Gate Monitor`:

1. `-codec json -benchMode business`
2. `-codec msgpack -benchMode business`
3. `-codec json -benchMode framework`
4. `-codec msgpack -benchMode framework`

## Output Metrics

The program periodically outputs:

- `sent`: Cumulative sent messages
- `recv`: Cumulative received messages
- `avg_send_qps`: Average send QPS

Used for quickly verifying throughput performance of cross-Actor chat room routing and broadcast chain.
