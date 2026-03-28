package zscript

import "testing"

func TestContextPoolLifecycle(t *testing.T) {
	ctx := GetContext(1, 2).
		WithTraceID(3).
		WithOwner(struct{}{}).
		WithMessage(10, 11, "data").
		WithMetadata("k", "v")

	if ctx.ActorID != 1 || ctx.ActorType != 2 {
		t.Fatalf("unexpected actor fields: %+v", ctx)
	}
	if ctx.TraceID != 3 || ctx.MsgID != 10 || ctx.AuthID != 11 {
		t.Fatalf("unexpected message fields: %+v", ctx)
	}
	if ctx.Metadata == nil || ctx.Metadata["k"] != "v" {
		t.Fatalf("expected metadata to be set")
	}

	PutContext(ctx)

	if ctx.ActorID != 0 || ctx.ActorType != 0 || ctx.TraceID != 0 || ctx.Metadata != nil {
		t.Fatalf("expected context reset, got: %+v", ctx)
	}
}

func BenchmarkGetPutContext(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := GetContext(1, 2)
		PutContext(ctx)
	}
}
