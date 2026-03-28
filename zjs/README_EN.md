# zjs

**JavaScript Script Engine**: Implements `zscript.IScriptEngine`.

## Usage

```go
engine := zjs.NewEngine(cfg)
group.SetScriptEngine(ziface.ScriptEngineJS, engine)
```

## Security defaults

- Set `cfg.ScriptDir` in production so `require()` can only load modules under that root.
- If `ScriptDir` is empty, **`require()` fails by default**; set `cfg.AllowRequireWithoutScriptDir = true` only for legacy unsandboxed behavior. See root `SECURITY_EN.md`.
