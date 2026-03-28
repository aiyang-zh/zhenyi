# zjs

**JavaScript 脚本引擎**：实现 `zscript.IScriptEngine`。

## 使用

```go
engine := zjs.NewEngine(cfg)
group.SetScriptEngine(ziface.ScriptEngineJS, engine)
```

## 安全默认

- 生产环境请设置 `cfg.ScriptDir`，使 `require()` 仅能加载该目录下的模块。
- `ScriptDir` 为空时 **默认禁止 `require()`**；仅当明确需要历史行为时设置 `cfg.AllowRequireWithoutScriptDir = true`（无目录沙箱）。详见根目录 `SECURITY.md`。
