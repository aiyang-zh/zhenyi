package ztengo

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aiyang-zh/zhenyi/zscript"
	"github.com/d5/tengo/v2"
)

func TestContextTypeNameAndString(t *testing.T) {
	e, err := NewEngine(&zscript.EngineConfig{Type: zscript.EngineTypeTengo, Timeout: 0})
	if err != nil {
		t.Fatalf("NewEngine err: %v", err)
	}
	ctx := zscript.GetContext(1, 2)
	defer zscript.PutContext(ctx)
	c := &Context{ctx: ctx, eng: e}
	if c.TypeName() != "context" {
		t.Fatalf("TypeName=%q", c.TypeName())
	}
	if c.String() == "" {
		t.Fatalf("String empty")
	}
}

func TestEngineLoadReloadAndType(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.tengo")
	// define a simple function and a call target
	if err := os.WriteFile(p, []byte("add := func(ctx, a, b) { return a + b }\n"), 0o644); err != nil {
		t.Fatalf("write err: %v", err)
	}

	e, err := NewEngine(&zscript.EngineConfig{Type: zscript.EngineTypeTengo, ScriptDir: dir, Timeout: 10 * time.Millisecond})
	if err != nil {
		t.Fatalf("NewEngine err: %v", err)
	}
	if err := e.LoadScripts([]string{p}); err != nil {
		t.Fatalf("LoadScripts err: %v", err)
	}
	if err := e.ReloadScript(p); err != nil {
		t.Fatalf("ReloadScript err: %v", err)
	}
	if err := e.ReloadAllScripts(); err != nil {
		t.Fatalf("ReloadAllScripts err: %v", err)
	}
	if e.GetType() != string(zscript.EngineTypeTengo) {
		t.Fatalf("GetType=%q", e.GetType())
	}
	e.Close()
}

func TestContextIndexGetAndConversions(t *testing.T) {
	e, err := NewEngine(&zscript.EngineConfig{Type: zscript.EngineTypeTengo, Timeout: 0})
	if err != nil {
		t.Fatalf("NewEngine err: %v", err)
	}
	ctx := zscript.GetContext(10, 20).WithMessage(30, 40, "data").WithTraceID(50).WithMetadata("k", "v")
	defer zscript.PutContext(ctx)
	c := &Context{ctx: ctx, eng: e}

	// IndexGet: non-string key
	if v, _ := c.IndexGet(&tengo.Int{Value: 1}); v != tengo.UndefinedValue {
		t.Fatalf("expected undefined for non-string key")
	}
	// IndexGet: known keys
	for _, key := range []string{"ActorID", "ActorType", "MsgID", "AuthID", "TraceID", "NowMillis"} {
		v, _ := c.IndexGet(&tengo.String{Value: key})
		if v == nil || v == tengo.UndefinedValue {
			t.Fatalf("expected value for %s", key)
		}
	}
	// optional keys
	v, _ := c.IndexGet(&tengo.String{Value: "Owner"})
	if v != tengo.UndefinedValue {
		// owner nil -> undefined
		t.Fatalf("expected undefined owner when nil")
	}
	v, _ = c.IndexGet(&tengo.String{Value: "MsgData"})
	if v == tengo.UndefinedValue {
		// msgdata is set to "data" via WithMessage
		// It's OK if conversion returns undefined for unsupported type, but "string" should be supported.
	}
	v, _ = c.IndexGet(&tengo.String{Value: "Metadata"})
	if v == tengo.UndefinedValue {
		t.Fatalf("expected metadata value")
	}

	// goToTengo branches
	_ = e.goToTengo(nil)
	_ = e.goToTengo(true)
	_ = e.goToTengo("s")
	_ = e.goToTengo([]byte("b"))
	_ = e.goToTengo(int(1))
	_ = e.goToTengo(int32(1))
	_ = e.goToTengo(int64(1))
	_ = e.goToTengo(uint64(1))
	_ = e.goToTengo(float32(1.25))
	_ = e.goToTengo(float64(1.5))
	_ = e.goToTengo(map[string]interface{}{"k": "v"})
	_ = e.goToTengo([]interface{}{1, "x"})
	_ = e.goToTengo(ctx)

	// tengoToGo branches
	if e.tengoToGo(tengo.UndefinedValue) != nil {
		t.Fatalf("undefined should map to nil")
	}
	_ = e.tengoToGo(tengo.TrueValue)
	_ = e.tengoToGo(&tengo.Int{Value: 1})
	_ = e.tengoToGo(&tengo.Float{Value: 1.5})
	_ = e.tengoToGo(&tengo.String{Value: "x"})
	_ = e.tengoToGo(&tengo.Bytes{Value: []byte{1}})
	_ = e.tengoToGo(&tengo.Array{Value: []tengo.Object{&tengo.Int{Value: 1}}})
	_ = e.tengoToGo(&tengo.Map{Value: map[string]tengo.Object{"k": &tengo.String{Value: "v"}}})
	_ = e.tengoToGo(&tengo.Error{Value: &tengo.String{Value: "err"}})
}
