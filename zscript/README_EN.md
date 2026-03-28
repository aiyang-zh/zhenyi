# zscript

**Script Engine Abstraction Layer**: Unifies script engine interface, invocation parameters, and statistics capability, shields specific language implementation differences.

## Module Positioning

- Defines `IScriptEngine` contract (load, call, hot-reload, close)
- Provides unified `CallParams` and `EngineConfig`
- Serves as common abstraction layer for `zjs/zlua/zstarlark/ztengo`

## Core Interfaces (Commonly Used)

- `LoadScript` / `LoadScripts` / `ReloadScript` / `ReloadAllScripts`
- `Call(params, function, args...)` executes script function
- `GetStats` / `GetType` / `Close`

## Engine Implementations

- `zjs`: JavaScript
- `zlua`: Lua
- `zstarlark`: Starlark
- `ztengo`: Tengo

## Minimal Integration

Inject via `Group.SetScriptEngine(engineType, engine)`, use within Actor via `GetGroup().GetScriptEngine()`.

## Usage Suggestions

- **JavaScript (`zjs`)**: set `EngineConfig.ScriptDir` in production so `require` stays under a fixed root. If `ScriptDir` is empty, **`require()` is rejected by default**. Set `AllowRequireWithoutScriptDir` only if you accept legacy unsandboxed behavior (see root `SECURITY_EN.md`).
- Uniformly pass messages and context via `CallParams` to avoid parameter drift between engines
- Compile/load verify before hot-reload to avoid runtime failure
- Pay attention to `GetStats()` metrics, combine with `zmetrics` for script execution observability

## Related Documentation

- Module API navigation: `../docs/MODULE_API.md`
- Architecture: `../docs/ARCHITECTURE.md`
