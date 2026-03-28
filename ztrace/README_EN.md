# ztrace

**Distributed Tracing Tool Module**: Provides W3C `traceparent` parsing, generation, and log field assistance.

## Module Positioning

- Unifies trace id / span id processing logic
- Provides pass-through foundation for `zgate` and business handlers
- Provides lightweight capability in scenarios without strong dependency on full tracing SDK

## Core API

- `ParseTraceparent` / `ParseOrGenerate`
- `ExtractFromHTTPRequest` / `ExtractOrGenerateFromHTTPRequest`
- `GenerateTraceID` / `GenerateSpanID`
- `TraceIDToHex` / `SpanIDToHex`
- `LogFields`

## Usage

- Cooperate with Gate's `SetTraceHook` to inject trace when receiving packets
- Within Handler, can pass through via msg's TraceIdHi/TraceIdLo/SpanId

## Related Documentation

- Architecture: `../docs/ARCHITECTURE.md`
- Module API navigation: `../docs/MODULE_API.md`
