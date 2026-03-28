# ztrace

**链路追踪工具模块**：提供 W3C `traceparent` 解析、生成与日志字段辅助。

## 模块定位

- 统一 trace id / span id 处理逻辑
- 为 `zgate` 与业务 handler 提供透传基础
- 在不强依赖完整 tracing SDK 的场景下提供轻量能力

## 核心 API

- `ParseTraceparent` / `ParseOrGenerate`
- `ExtractFromHTTPRequest` / `ExtractOrGenerateFromHTTPRequest`
- `GenerateTraceID` / `GenerateSpanID`
- `TraceIDToHex` / `SpanIDToHex`
- `LogFields`

## 使用

- 配合 Gate 的 `SetTraceHook` 在入包时注入 trace
- Handler 内可通过 msg 的 TraceIdHi/TraceIdLo/SpanId 透传

## 相关文档

- 架构说明：`../docs/ARCHITECTURE.md`
- 模块 API 导航：`../docs/MODULE_API.md`
