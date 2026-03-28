package zscript

import "github.com/aiyang-zh/zhenyi-base/zerrs"

// Error type definitions for script subsystem.
// 脚本系统错误类型定义。
const (
	ErrTypeScript zerrs.ErrorType = "SCRIPT" // Script-related errors / 脚本相关错误
	ErrTypeEngine zerrs.ErrorType = "ENGINE" // Engine-related errors / 引擎相关错误
)

// Common errors (based on zhenyi-base/zerrs).
// 常用错误（使用 zhenyi-base/zerrs）。
var (
	// ErrScriptNotFound means script file not found.
	// ErrScriptNotFound 脚本未找到。
	ErrScriptNotFound = zerrs.New(ErrTypeScript, "script not found")

	// ErrScriptCompile means script compile failed.
	// ErrScriptCompile 脚本编译失败。
	ErrScriptCompile = zerrs.New(ErrTypeScript, "script compile failed")

	// ErrScriptTimeout means script execution timed out.
	// ErrScriptTimeout 脚本执行超时。
	ErrScriptTimeout = zerrs.New(zerrs.ErrTypeTimeout, "script execution timeout")

	// ErrScriptPanic means script execution panicked.
	// ErrScriptPanic 脚本执行 panic。
	ErrScriptPanic = zerrs.New(ErrTypeScript, "script execution panic")

	// ErrFunctionNotFound means target function was not found.
	// ErrFunctionNotFound 函数未找到。
	ErrFunctionNotFound = zerrs.New(ErrTypeScript, "function not found")

	// ErrInvalidArgument means invalid function arguments.
	// ErrInvalidArgument 参数无效。
	ErrInvalidArgument = zerrs.New(zerrs.ErrTypeValidation, "invalid argument")

	// ErrEngineNotInitialized means engine has not been initialized.
	// ErrEngineNotInitialized 引擎未初始化。
	ErrEngineNotInitialized = zerrs.New(ErrTypeScript, "engine not initialized")
)
