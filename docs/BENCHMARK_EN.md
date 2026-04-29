# zhenyi Benchmark Analysis Report

> Data source: four raw log files (`client_out.log`, `out.log`, `client_out_hour.log`, `out_hour.log`), archived in `zhenyi-site/docs/bench/` as scenario-based `logs.zip` bundles.  
> Hardware: Alibaba Cloud 2 vCPU / 4 GiB, client and server on the same host with one core each.  
> Server architecture: 2 Actors (Gate + IM), worker pool size 500 each.  
> Test path: **Client -> Gate Actor -> IM Actor -> Gate Actor -> Client (echo round-trip)**.  
> Each message goes through 4 actor-mailbox hops.

---

## Raw Materials

- **Scenario A logs (2000 conns x 100ms x 7 days)**: `zhenyi-site/docs/bench/scenario-a_2000c_100ms_7d/logs.zip`
- **Scenario B logs (500 conns x 1ms x 1 hour)**: `zhenyi-site/docs/bench/scenario-b_500c_1ms_1h/logs.zip`
- **Environment/result screenshots**: `zhenyi-site/docs/bench/assets/`
  - `WechatIMG105.jpg`
  - `WX20260419-192439@2x.png`
  - `WX20260419-192520@2x.png`
  - `WX20260419-192533@2x.png`

## Repro Entry Index

- **Benchmark server (single-process)**: `examples/im_single_demo_bench/main.go`
- **Load generator client**: `examples/im_multi_client_load/main.go`

Standard benchmark commands (framework + msgpack):

```bash
# Server
go run examples/im_single_demo_bench/main.go -reactor -codec msgpack -benchMode framework

# Client
go run ./examples/im_multi_client_load -addr 127.0.0.1:8001 -room lobby -clients 500 -intervalMs 1 -durationS 3600 -msgLogin 1 -msgJoin 2 -msgSend 4 -codec msgpack -benchMode framework
```

## Overview

| Metric | Scenario A (`client_out.log` + `out.log`) | Scenario B (`client_out_hour.log` + `out_hour.log`) |
|---|---:|---:|
| Time range | 2026-04-03 15:48 -> 2026-04-10 15:49 | 2026-04-10 16:36 -> 2026-04-10 17:37 |
| Connections | 2000 | 500 |
| Send interval | 100ms | 1ms |
| Duration | 604,797s (~7 days) | 3,597.6s (~1 hour) |
| Total sent | 12,095,967,155 | 300,514,579 |
| Total recv | 12,095,967,132 | 300,508,719 |
| Client-side unreturned count (`sent-recv`) | 23 (0.000000019%) | 5,860 (0.00195%) |

> Note: "unreturned/loss" in this report is **client-side `sent-recv`** from the load generator logs.  
> Since client `recv` is batch-flushed in read callbacks and the test exits at deadline, this value may include tail responses not yet counted at shutdown. It is not equivalent to confirmed server-side message loss.

---

## Scenario A (2000 connections, 7-day steady run)

- Client steady send QPS: ~20,000 (`avg_send_qps=20000.05` at final line)
- Server global QPS (end): ~39,999.62
- RTT P50: ~0.11-0.19ms
- RTT P99: ~1.27-1.52ms
- Peak RTT Max: 142ms (rare tail, correlated with scheduled system tasks)
- GC total pause over 7 days: ~45,966ms (~46s), around 0.0076% of total time
- Goroutines: stable at 25 during the run

## Scenario B (500 connections, 1-hour high pressure)

- Client average send QPS: ~83,532
- Server global QPS (end): ~166,738.74
- RTT P50: ~8.7-13ms
- RTT P99: ~47-88ms
- GC cadence: higher than Scenario A under pressure, still within expected bounds

---

## Key Takeaways

1. **Actor runtime is stable under long-running load**: 7-day run stays near 20k client echo QPS / 40k server message QPS.
2. **Latency scales with load as expected**: high-pressure mode trades tail latency for throughput.
3. **Resource profile is controlled**: no goroutine leak signal; memory and GC remain bounded.
4. **Tail spikes are mostly environmental**: rare high RTT spikes correlate with system-level scheduled tasks.

---

## Recommended Public Statement

Under the tested environment (2 vCPU / 4 GiB, same-host client+server, both `GOMAXPROCS=1`), zhenyi shows two validated operating points:

- **Low-latency mode**: ~40k server QPS, P99 around 1-2ms
- **High-throughput mode**: ~166k server QPS, P99 around 50-100ms

Use these as baseline capacity references and re-validate on your target production topology.
