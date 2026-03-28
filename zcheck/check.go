// Package zcheck provides startup dependency self-checks aligned with docs/GLOBALS_AND_HOOKS.md.
// Package zcheck 提供进程启动阶段的依赖自检，与 docs/GLOBALS_AND_HOOKS.md 对齐。
package zcheck

import (
	"errors"
	"fmt"

	"github.com/aiyang-zh/zhenyi-base/zerrs"
	"github.com/aiyang-zh/zhenyi/zbus"
	"github.com/aiyang-zh/zhenyi/zmetrics"
	"github.com/aiyang-zh/zhenyi/znats"
)

// Config describes global dependencies to validate at process startup.
// Config 描述本进程在启动时要校验的全局依赖。
type Config struct {
	// RequireRemoteBus 为 true 时要求 zbus.DefaultBus 已注入。
	RequireRemoteBus bool

	// RequireNatsPool 为 true 时要求 znats.DefaultNatsClient 非 nil。
	RequireNatsPool bool

	// RequireNatsConnected 为 true 时要求默认 NATS 池内各连接已 IsConnected（需先 Connect）。
	// 若 RequireNatsPool 为 false 但此项为 true，则仍会检查池存在且已连接。
	RequireNatsConnected bool

	// TouchMetricsRegistry 为 true 时调用 zmetrics.Global()，确保指标注册表已初始化（正常 init 恒成立）。
	TouchMetricsRegistry bool
}

// Validate runs self-check and returns nil on success, otherwise joined errors.
// Validate 执行自检，全部通过返回 nil；否则返回合并后的错误（可用 errors.Unwrap 遍历）。
func Validate(cfg Config) error {
	var errs []error

	if cfg.RequireRemoteBus && zbus.DefaultBus == nil {
		errs = append(errs, zerrs.New(zerrs.ErrTypeConfig, "zbus.DefaultBus is nil (跨进程路由 / SendMsg 远程需要注入 TopicBus，例如 znats.NewDefaultNats)"))
	}

	if cfg.RequireNatsPool && znats.DefaultNatsClient == nil {
		errs = append(errs, zerrs.New(zerrs.ErrTypeConfig, "znats.DefaultNatsClient is nil (请先调用 znats.NewDefaultNats)"))
	}

	if cfg.RequireNatsConnected {
		p := znats.DefaultNatsClient
		if p == nil {
			errs = append(errs, zerrs.New(zerrs.ErrTypeConfig, "znats.DefaultNatsClient is nil (无法检查连接，请先 NewDefaultNats 并 Connect)"))
		} else if !p.IsConnected() {
			errs = append(errs, zerrs.New(zerrs.ErrTypeNetwork, "znats.DefaultNatsClient is not connected (请先对池执行 Connect)"))
		}
	}

	if cfg.TouchMetricsRegistry {
		_ = zmetrics.Global()
	}

	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}

// ValidateOrPanic behaves like Validate but panics on failure (for must-succeed entrypoints).
// ValidateOrPanic 与 Validate 相同，失败时 panic（仅用于必须成功的进程入口）。
func ValidateOrPanic(cfg Config) {
	if err := Validate(cfg); err != nil {
		panic(fmt.Sprintf("zcheck: startup validation failed: %v", err))
	}
}
