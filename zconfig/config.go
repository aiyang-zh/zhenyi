// Package zconfig provides TOML configuration loading with hot reload support.
// Package zconfig 提供 TOML 配置加载，支持热重载。
package zconfig

import (
	"github.com/pelletier/go-toml/v2"
	"os"
	"sync"
)

// Load parses TOML from path into dest (dest must be a pointer).
// Load 从 path 加载 TOML 到 dest，dest 需为指针。
func Load(path string, dest any) error {
	data, err := os.ReadFile(path) // #nosec G304 -- path provided by caller for config loading
	if err != nil {
		return err
	}
	return toml.Unmarshal(data, dest)
}

// Loader is a hot-reloadable config loader.
// Loader 可热重载的配置加载器。
type Loader struct {
	path string
	mu   sync.RWMutex
	cb   func() // Hot-reload callback / 热重载回调
}

// NewLoader creates a loader with a config file path.
// NewLoader 创建加载器，path 为配置文件路径。
func NewLoader(path string) *Loader {
	return &Loader{path: path}
}

// Load reads config into dest (pointer required). It is thread-safe.
// Load 加载配置到 dest，dest 需为指针。线程安全。
func (l *Loader) Load(dest any) error {
	data, err := os.ReadFile(l.path) // #nosec G304 -- loader path set once at construction
	if err != nil {
		return err
	}
	return toml.Unmarshal(data, dest)
}

// OnReload sets the hot-reload callback.
// OnReload 设置热重载回调。
// The callback can call Load again (or validate/atomic-swap), and trigger timing is caller-controlled.
// 你可以在回调中再次 Load（或做校验、原子替换等），由调用方决定触发时机。
func (l *Loader) OnReload(cb func()) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cb = cb
}

// Reload manually triggers one reload callback invocation.
// Reload 手动触发一次重载回调，触发时机由调用方决定（如配置中心推送/管理接口/定时任务）。
// Note: this only triggers callback; call Load in callback if parsing is needed.
// 说明：该方法只负责触发回调；如需重新解析配置，请在回调中调用 Load。
func (l *Loader) Reload() {
	l.reload()
}

// reload triggers one reload callback (internal helper).
// reload 触发一次重载回调（内部实现）。
func (l *Loader) reload() {
	l.mu.RLock()
	cb := l.cb
	l.mu.RUnlock()
	if cb != nil {
		cb()
	}
}
