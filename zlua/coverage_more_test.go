package zlua

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aiyang-zh/zhenyi/zscript"
	
)

func TestGoToLua_CoversBranches(t *testing.T) {
	e, err := NewEngine(nil)
	if err != nil {
		t.Fatalf("NewEngine err: %v", err)
	}
	L := lua.NewState()
	defer L.Close()

	_ = e.goToLua(L, nil)
	_ = e.goToLua(L, true)
	_ = e.goToLua(L, "s")
	_ = e.goToLua(L, []byte("b"))
	_ = e.goToLua(L, float64(1.5))
	_ = e.goToLua(L, float32(1.25))
	_ = e.goToLua(L, int(1))
	_ = e.goToLua(L, int8(1))
	_ = e.goToLua(L, int16(1))
	_ = e.goToLua(L, int32(1))
	_ = e.goToLua(L, int64(1))
	// int64 big -> string
	v := e.goToLua(L, int64(9007199254740992))
	if _, ok := v.(lua.LString); !ok {
		t.Fatalf("expected string for big int64")
	}
	_ = e.goToLua(L, uint(1))
	_ = e.goToLua(L, uint8(1))
	_ = e.goToLua(L, uint16(1))
	_ = e.goToLua(L, uint32(1))
	// uint64 big -> string
	v = e.goToLua(L, uint64(9007199254740992))
	if _, ok := v.(lua.LString); !ok {
		t.Fatalf("expected string for big uint64")
	}
	_ = e.goToLua(L, map[string]interface{}{"k": "v", "n": int64(1)})
	_ = e.goToLua(L, []interface{}{1, "x", true})
	_ = e.goToLua(L, struct{}{}) // default
}

func TestSetupSafeEnv_PrintFunction(t *testing.T) {
	e, err := NewEngine(nil)
	if err != nil {
		t.Fatalf("NewEngine err: %v", err)
	}
	L := lua.NewState()
	defer L.Close()
	e.setupSafeEnv(L)
	if err := L.DoString(`print("hello", 123)`); err != nil {
		t.Fatalf("print err: %v", err)
	}
}

func TestSafeCreateVM_RetriesOnPreloadFailureAndCloseIdempotent(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "bad.lua")
	if err := os.WriteFile(p, []byte(`error("boom")`), 0o644); err != nil {
		t.Fatal(err)
	}

	e, err := NewEngine(&zscript.EngineConfig{Type: zscript.EngineTypeLua})
	if err != nil {
		t.Fatalf("NewEngine err: %v", err)
	}
	if err := e.LoadScript(p); err != nil {
		t.Fatalf("LoadScript err: %v", err)
	}

	// createVM will fail when preloading this proto
	if w := e.safeCreateVM(); w != nil {
		t.Fatalf("expected nil after retries")
	}

	e.Close()
	e.Close() // idempotent branch
}
