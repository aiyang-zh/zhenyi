package zjs

import (
	"testing"

	"github.com/aiyang-zh/zhenyi-base/zlo
	"github.com/aiyang-zh/zhenyi/zscript"
	"github.com/dop251/goja"
)

func TestConsoleAndProgramMapsBranches(t *testing.T) {
	e, err := NewEngine(&zscript.EngineConfig{Type: zscript.EngineTypeJS})
	if err != nil {
		t.Fatalf("NewEngine err: %v", err)
	}

	// registerConsole coverage
	vm := goja.New()
	e.registerConsole(vm)
	if _, err := vm.RunString(`console.log(1, "a", {k:"v"});`); err != nil {
		t.Fatalf("console.log run err: %v", err)
	}

	// getProgramsMap nil / bad type (use fresh Engine to avoid atomic.Value type conflicts)
	enil := &Engine{logger: zlog.GetDefaultLog()}
	_ = enil.getProgramsMap()
	ebad := &Engine{logger: zlog.GetDefaultLog()}
	ebad.programs.Store(123)
	_ = ebad.getProgramsMap()

	// getModulePrograms nil / bad type
	_ = enil.getModulePrograms()
	ebad.modulePrograms.Store(123)
	_ = ebad.getModulePrograms()

	// clearModuleProgram branch
	e.modulePrograms.Store(map[string]*goja.Program{"a": nil, "b": nil})
	e.clearModuleProgram("a")
	m := e.getModulePrograms()
	if _, ok := m["a"]; ok {
		t.Fatalf("expected a removed")
	}
}

func TestSafeCreateVMClosed(t *testing.T) {
	e, err := NewEngine(&zscript.EngineConfig{Type: zscript.EngineTypeJS})
	if err != nil {
		t.Fatalf("NewEngine err: %v", err)
	}
	e.closed.Store(true)
	if w := e.safeCreateVM(); w != nil {
		t.Fatalf("expected nil wrapper when closed")
	}
}

func TestNewEngineAppliesDefaults(t *testing.T) {
	e, err := NewEngine(&zscript.EngineConfig{
		Type:          zscript.EngineTypeJS,
		MaxVMUseCount: 0,
		MaxVMAge:      0,
	})
	if err != nil {
		t.Fatalf("NewEngine err: %v", err)
	}
	if e.maxVMUseCount <= 0 {
		t.Fatalf("expected default maxVMUseCount applied")
	}
	if e.maxVMAge <= 0 {
		t.Fatalf("expected default maxVMAge applied")
	}
}

func TestSafeCreateVM_RetriesOnPreloadFailure(t *testing.T) {
	e, err := NewEngine(&zscript.EngineConfig{Type: zscript.EngineTypeJS})
	if err != nil {
		t.Fatalf("NewEngine err: %v", err)
	}
	// preload program that always fails -> createVM returns nil -> safeCreateVM retries then nil
	bad, err := goja.Compile("bad.js", `throw new Error("boom")`, true)
	if err != nil {
		t.Fatalf("compile err: %v", err)
	}
	e.programs.Store(map[string]*goja.Program{"bad": bad})
	if w := e.safeCreateVM(); w != nil {
		t.Fatalf("expected nil after retries")
	}
}
