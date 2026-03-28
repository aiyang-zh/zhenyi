package zlua

import (
	"sync/atomic"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestSafeCreateVMClosed(t *testing.T) {
	e, err := NewEngine(nil)
	if err != nil {
		t.Fatalf("NewEngine err: %v", err)
	}
	e.closed.Store(true)
	if w := e.safeCreateVM(); w != nil {
		t.Fatalf("expected nil vm when closed")
	}
}

func TestProgramMapBranchesAndSandbox(t *testing.T) {
	e, err := NewEngine(nil)
	if err != nil {
		t.Fatalf("NewEngine err: %v", err)
	}
	// getProgramsMap nil/bad type
	e.programs = atomic.Value{} // zero value -> Load nil
	_ = e.getProgramsMap()
	e.programs.Store(123)
	_ = e.getProgramsMap()

	L := lua.NewState()
	defer L.Close()
	e.setupSafeEnv(L)
	// dangerous globals disabled
	for _, k := range []string{"loadfile", "dofile", "require", "load", "loadstring"} {
		if v := L.GetGlobal(k); v != lua.LNil {
			t.Fatalf("expected %s disabled", k)
		}
	}
}

func TestRegisterContextType_IndexBranches(t *testing.T) {
	e, err := NewEngine(nil)
	if err != nil {
		t.Fatalf("NewEngine err: %v", err)
	}
	L := lua.NewState()
	defer L.Close()
	e.setupSafeEnv(L)
	e.registerContextType(L)

	// create userdata with wrong type to hit !ok branch
	ud := L.NewUserData()
	ud.Value = "not_ctx"
	mt := L.GetTypeMetatable("ScriptContext")
	L.SetMetatable(ud, mt)
	L.SetGlobal("ctx", ud)

	if err := L.DoString(`return ctx.ActorID`); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// unknown key -> nil
	if err := L.DoString(`return ctx.Nope`); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}
