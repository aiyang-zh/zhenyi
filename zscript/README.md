# zscript

**脚本引擎抽象层**：统一脚本引擎接口、调用参数与统计能力，屏蔽具体语言实现差异。

## 模块定位

- 定义 `IScriptEngine` 契约（加载、调用、热更、关闭）
- 提供统一 `CallParams` 与 `EngineConfig`
- 作为 `zjs/zlua/zstarlark/ztengo` 的共用抽象层

## 核心接口（常用）

- `LoadScript` / `LoadScripts` / `ReloadScript` / `ReloadAllScripts`
- `Call(params, function, args...)` 执行脚本函数
- `GetStats` / `GetType` / `Close`

## 引擎实现

- `zjs`：JavaScript
- `zlua`：Lua
- `zstarlark`：Starlark
- `ztengo`：Tengo

## 最小接入

通过 `Group.SetScriptEngine(engineType, engine)` 注入，Actor 内通过 `GetGroup().GetScriptEngine()` 使用。

## 使用建议

- **JavaScript（`zjs`）**：生产环境请设置 `EngineConfig.ScriptDir`，将 `require` 限制在固定根目录；`ScriptDir` 为空时默认 **禁止使用 `require()`**。仅当接受无沙箱风险时，可显式设置 `AllowRequireWithoutScriptDir`（见根目录 `SECURITY.md`）。
- 统一通过 `CallParams` 传递消息与上下文，避免引擎间参数漂移
- 热更前先编译/加载校验，避免运行期失败
- 关注 `GetStats()` 指标，结合 `zmetrics` 做脚本运行可观测

## 相关文档

- 模块 API 导航：`../docs/MODULE_API.md`
- 架构说明：`../docs/ARCHITECTURE.md`
