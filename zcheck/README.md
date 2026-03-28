# zcheck

启动阶段**全局依赖自检**：`zbus.DefaultBus`、`znats.DefaultNatsClient`、NATS 是否已连接、可选触碰 `zmetrics` 注册表。

## 模块定位

- 在应用启动前快速暴露全局依赖缺失
- 将“运行时才失败”的问题前移到启动期
- 支持返回聚合错误（`errors.Join`），便于一次性看到全部缺口

## 最小用法

```go
import "github.com/aiyang-zh/zhenyi/zcheck"

if err := zcheck.Validate(zcheck.Config{
	RequireRemoteBus:      true,
	RequireNatsPool:       true,
	RequireNatsConnected:  true, // 在 DefaultNatsClient.Connect(ctx) 之后调用
	TouchMetricsRegistry:  true,
}); err != nil {
	log.Fatal(err)
}
```

单机 Gate、不做跨进程时可关闭 `RequireRemoteBus` / `RequireNatsPool` / `RequireNatsConnected`。

## 核心 API

- `Validate(cfg Config) error`
- `ValidateOrPanic(cfg Config)`

## 相关文档

- 全局变量与启动检查：`../docs/GLOBALS_AND_HOOKS.md`
- 启动编排：`../zstartup/README.md`
