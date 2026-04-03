# im_single_demo_bench

单机压测版示例（Gate + IM Actor），用于 **连接/QPS/延迟调优**。  
与 `examples/im_single_demo` 不同：本示例偏压测与观测，默认不提供完整聊天室广播语义。

## 启动

```bash
go run ./examples/im_single_demo_bench
```

默认 Gate 地址：`127.0.0.1:8001`。

## 常用参数

- `--codec json|msgpack`：协议编解码
- `--benchMode business|framework`：业务路径或框架路径（固定轻回包）
- `--reactor`：开启 TCP reactor 模式
- `--sharedSendWorker`：开启共享发送 worker
- `--pprofAddr 127.0.0.1:6060`：开启 pprof
- `--pyroscopeAddr http://127.0.0.1:4040`：开启持续 profiling
- `--watchdogMs`：开启处理阻塞 watchdog
- `--lowLatencyPreset`：启用低延迟预设调参

发送路径调参（用于尾延迟实验）：
- `--sendBatchMin` / `--sendBatchMax` / `--sendBatchTargetMeanMs`
- `--sendBackoffFirst` / `--sendBackoffSecond` / `--sendBackoffSleepUs`
- `--reactorMaxQueuedMsgs` / `--reactorFlushBatchesPerTurn`

## 常用命令

基础压测：

```bash
go run ./examples/im_single_demo_bench --reactor --sharedSendWorker --benchMode framework --codec msgpack
```

压 P99（仅调参示例）：

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

## 配套客户端

- 交互式客户端：`go run ./examples/im_single_client`
- 并发压测客户端：`go run ./examples/im_multi_client_load`

## 使用建议

- 需要完整聊天室语义（`room_notify`/广播/签名等）请使用 `examples/im_single_demo`
- 需要做连接/QPS/尾延迟实验请使用 `examples/im_single_demo_bench`
