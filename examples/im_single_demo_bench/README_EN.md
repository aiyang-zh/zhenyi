# im_single_demo_bench

Single-node benchmark-oriented example (Gate + IM Actor) for **connections/QPS/latency tuning**.  
Unlike `examples/im_single_demo`, this example is optimized for benchmarking and observability, not full chat-room semantics.

## Start

```bash
go run ./examples/im_single_demo_bench
```

Default Gate address: `127.0.0.1:8001`.

## Common Flags

- `--codec json|msgpack`: payload codec
- `--benchMode business|framework`: business path or framework-only fast reply path
- `--reactor`: enable TCP reactor mode
- `--sharedSendWorker`: enable shared send workers
- `--pprofAddr 127.0.0.1:6060`: enable pprof
- `--pyroscopeAddr http://127.0.0.1:4040`: enable continuous profiling
- `--watchdogMs`: enable handler-block watchdog
- `--lowLatencyPreset`: apply low-latency tuning preset

Send-path tuning flags:
- `--sendBatchMin` / `--sendBatchMax` / `--sendBatchTargetMeanMs`
- `--sendBackoffFirst` / `--sendBackoffSecond` / `--sendBackoffSleepUs`
- `--reactorMaxQueuedMsgs` / `--reactorFlushBatchesPerTurn`

## Typical Commands

Baseline benchmark:

```bash
go run ./examples/im_single_demo_bench --reactor --sharedSendWorker --benchMode framework --codec msgpack
```

P99 tuning example:

```bash
go run ./examples/im_single_demo_bench \
  --reactor \
  --sharedSendWorker \
  --benchMode framework \
  --codec msgpack \
  --sendBatchMax 16 \
  --sendBackoffFirst 6 \
  --sendBackoffSecond 18 \
  --sendBackoffSleepUs 2 \
  --reactorFlushBatchesPerTurn 4
```

## Companion Clients

- Interactive client: `go run ./examples/im_single_client`
- Concurrent load client: `go run ./examples/im_multi_client_load`

## Notes

- Use `examples/im_single_demo` if you need full chat semantics (`room_notify`/broadcast/signature).
- Use `examples/im_single_demo_bench` for load and tail-latency experiments.
