package zconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_NoFile(t *testing.T) {
	var cfg struct{ X int }
	err := Load("/nonexistent/path.toml", &cfg)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `
addr = ":9001"
workSize = 256
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var cfg struct {
		Addr     string `toml:"addr"`
		WorkSize int    `toml:"workSize"`
	}
	if err := Load(path, &cfg); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Addr != ":9001" {
		t.Errorf("Addr: got %q, want :9001", cfg.Addr)
	}
	if cfg.WorkSize != 256 {
		t.Errorf("WorkSize: got %d, want 256", cfg.WorkSize)
	}
}

func TestLoader_Load_NoFile(t *testing.T) {
	loader := NewLoader("/nonexistent/app.toml")
	var cfg struct{ Name string }
	err := loader.Load(&cfg)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoad_InvalidTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.toml")
	os.WriteFile(path, []byte("x = [invalid"), 0644)
	var cfg struct{ X int }
	err := Load(path, &cfg)
	if err == nil {
		t.Fatal("expected error for invalid TOML")
	}
}

func TestLoader_Load(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.toml")
	content := `name = "test"`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	loader := NewLoader(path)
	var cfg struct {
		Name string `toml:"name"`
	}
	if err := loader.Load(&cfg); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Name != "test" {
		t.Errorf("Name: got %q, want test", cfg.Name)
	}
}

func TestLoader_OnReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "reload.toml")
	if err := os.WriteFile(path, []byte("x = 1"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	loader := NewLoader(path)
	var x int
	loader.OnReload(func() {
		var cfg struct {
			X int `toml:"x"`
		}
		if err := loader.Load(&cfg); err == nil {
			x = cfg.X
		}
	})
	loader.reload()
	if x != 1 {
		t.Errorf("reload x: got %d, want 1", x)
	}
}
