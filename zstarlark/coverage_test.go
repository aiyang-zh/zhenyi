package zstarlark

import (
	"testing"

	"github.com/aiyang-zh/zhenyi/zscript"
	"go.starlark.net/starlark"
)

func TestLazyContextValueAndAttrs(t *testing.T) {
	e, err := NewEngine(nil)
	if err != nil {
		t.Fatalf("NewEngine err: %v", err)
	}
	ctx := zscript.GetContext(1, 2).WithMessage(3, 4, "x").WithTraceID(5).WithMetadata("k", "v")
	defer zscript.PutContext(ctx)

	lc := &lazyContext{engine: e, ctx: ctx}
	if lc.Type() != "context" || lc.String() == "" {
		t.Fatalf("unexpected value methods")
	}
	lc.Freeze()
	if lc.Truth() != starlark.True {
		t.Fatalf("expected truthy")
	}
	if _, err := lc.Hash(); err == nil {
		t.Fatalf("expected unhashable error")
	}

	v, ok, err := lc.Get(starlark.String("ActorID"))
	if err != nil || !ok || v == nil {
		t.Fatalf("Get ActorID failed")
	}
	if _, ok, _ := lc.Get(starlark.String("Nope")); ok {
		t.Fatalf("expected not found")
	}
	if _, _, err := lc.Get(starlark.MakeInt(1)); err == nil {
		t.Fatalf("expected error for non-string key")
	}

	if _, err := lc.Attr("ActorType"); err != nil {
		t.Fatalf("Attr err: %v", err)
	}
	if _, err := lc.Attr("Missing"); err == nil {
		t.Fatalf("expected NoSuchAttrError")
	}
	if names := lc.AttrNames(); len(names) == 0 {
		t.Fatalf("expected attr names")
	}
}

func TestPrintHandler(t *testing.T) {
	e, err := NewEngine(nil)
	if err != nil {
		t.Fatalf("NewEngine err: %v", err)
	}
	ctx := zscript.GetContext(1, 2).WithMessage(3, 4, map[string]any{"a": 1}).WithMetadata("k", "v")
	defer zscript.PutContext(ctx)
	e.printHandler(nil, "hi")
}

func TestGoToStarlark_CoversTypes(t *testing.T) {
	e, err := NewEngine(nil)
	if err != nil {
		t.Fatalf("NewEngine err: %v", err)
	}
	_ = e.goToStarlark(true)
	_ = e.goToStarlark("s")
	_ = e.goToStarlark([]byte("b"))
	_ = e.goToStarlark(float64(1.5))
	_ = e.goToStarlark(float32(1.25))
	_ = e.goToStarlark(int(1))
	_ = e.goToStarlark(int8(1))
	_ = e.goToStarlark(int16(1))
	_ = e.goToStarlark(int32(1))
	_ = e.goToStarlark(int64(1))
	_ = e.goToStarlark(uint(1))
	_ = e.goToStarlark(uint8(1))
	_ = e.goToStarlark(uint16(1))
	_ = e.goToStarlark(uint32(1))
	_ = e.goToStarlark(uint64(1))
	_ = e.goToStarlark(map[string]any{"k": "v"})
	_ = e.goToStarlark([]any{1, "x"})
	_ = e.goToStarlark(struct{}{}) // default branch
}
