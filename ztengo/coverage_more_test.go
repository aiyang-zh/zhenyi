package ztengo

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aiyang-zh/zhenyi/zscript"
)

func TestNewEngine_NilConfigAndClose(t *testing.T) {
	e, err := NewEngine(nil)
	if err != nil {
		t.Fatalf("NewEngine err: %v", err)
	}
	if e.GetType() != string(zscript.EngineTypeTengo) {
		t.Fatalf("type=%s", e.GetType())
	}
	e.Close()
}

func TestLoadScriptsInternal_EdgeCases(t *testing.T) {
	e, err := NewEngine(&zscript.EngineConfig{Type: zscript.EngineTypeTengo})
	if err != nil {
		t.Fatalf("NewEngine err: %v", err)
	}
	// empty list no-op
	if err := e.LoadScripts(nil); err != nil {
		t.Fatalf("expected nil err: %v", err)
	}
	// file not found
	if err := e.LoadScript("/no/such/file.tengo"); err == nil {
		t.Fatalf("expected error for missing file")
	}
	// syntax error
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.tengo")
	_ = os.WriteFile(bad, []byte("x :="), 0o644)
	if err := e.LoadScript(bad); err == nil {
		t.Fatalf("expected compile error")
	}
}

func TestCall_ErrorsAndClosed(t *testing.T) {
	e, err := NewEngine(&zscript.EngineConfig{Type: zscript.EngineTypeTengo, Timeout: 0})
	if err != nil {
		t.Fatalf("NewEngine err: %v", err)
	}
	ctx := context.Background()
	// no scripts loaded -> function not found
	_, err = e.Call(ctx, &zscript.CallParams{ActorID: 1, ActorType: 2}, "nope")
	if err == nil {
		t.Fatalf("expected error")
	}

	// closed
	e.Close()
	_, err = e.Call(ctx, &zscript.CallParams{ActorID: 1, ActorType: 2}, "nope")
	if err == nil {
		t.Fatalf("expected error when closed")
	}
}

func TestCall_InvalidFunctionNameRejected(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.tengo")
	_ = os.WriteFile(p, []byte("ok := func(ctx){ return 1 }\n"), 0o644)
	e, err := NewEngine(&zscript.EngineConfig{Type: zscript.EngineTypeTengo, Timeout: 10 * time.Millisecond})
	if err != nil {
		t.Fatalf("NewEngine err: %v", err)
	}
	defer e.Close()
	if err := e.LoadScript(p); err != nil {
		t.Fatalf("load err: %v", err)
	}
	_, err = e.Call(context.Background(), &zscript.CallParams{ActorID: 1, ActorType: 2}, "ok);malicious(")
	if err == nil {
		t.Fatalf("expected injection rejected")
	}
}

func TestInternalGetters_NilAtomicValues(t *testing.T) {
	e, err := NewEngine(&zscript.EngineConfig{Type: zscript.EngineTypeTengo})
	if err != nil {
		t.Fatalf("NewEngine err: %v", err)
	}
	// force nil loads (atomic.Value zero value)
	e.sourceMap = atomic.Value{}
	e.sourceCode = atomic.Value{}
	e.validFunctions = atomic.Value{}
	e.globalVariables = atomic.Value{}

	_ = e.getSourceMap()
	_ = e.getSourceCode()
	_ = e.getValidFunctions()
	_ = e.getGlobalVariables()
}

func TestExtractAndDetectGlobalVariablesBranches(t *testing.T) {
	e, err := NewEngine(&zscript.EngineConfig{Type: zscript.EngineTypeTengo})
	if err != nil {
		t.Fatalf("NewEngine err: %v", err)
	}
	code := `
foo := func(ctx) { return 1 }
bar := func(ctx) { return 2 }
global_state := {"count": 0}
arr := [1,2,3]
foo := {"not_func": 1} // should be ignored because foo is a known func
`
	funcs := e.extractFunctionNames(code)
	if !funcs["foo"] || !funcs["bar"] {
		t.Fatalf("expected funcs extracted")
	}
	globals := e.detectGlobalVariables(code, funcs)
	// expect to see global_state and arr at least
	foundState, foundArr := false, false
	for _, g := range globals {
		if g == "global_state" {
			foundState = true
		}
		if g == "arr" {
			foundArr = true
		}
	}
	if !foundState || !foundArr {
		t.Fatalf("expected globals detected, got %v", globals)
	}
}
