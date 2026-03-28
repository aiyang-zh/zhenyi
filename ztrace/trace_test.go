package ztrace

import (
	"net/http"
	"testing"

	"github.com/aiyang-zh/zhenyi-base/zid"
)

func TestParseTraceparent_Invalid(t *testing.T) {
	tid, sid := ParseTraceparent("")
	if !TraceIDIsZero(tid) || sid != 0 {
		t.Fatalf("expected zero, got tid=%v sid=%v", tid, sid)
	}

	tid, sid = ParseTraceparent("00-xyz-abc-01")
	if !TraceIDIsZero(tid) || sid != 0 {
		t.Fatalf("expected zero for invalid format, got tid=%v sid=%v", tid, sid)
	}
}

func TestParseTraceparent_Valid(t *testing.T) {
	// 00-{32hex traceId}-{16hex spanId}-01
	in := "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01"
	tid, sid := ParseTraceparent(in)
	if TraceIDIsZero(tid) || sid == 0 {
		t.Fatalf("expected non-zero, got tid=%v sid=%v", tid, sid)
	}
	if TraceIDToHex(tid) != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("unexpected trace hex: %s", TraceIDToHex(tid))
	}
	if SpanIDToHex(sid) != "0123456789abcdef" {
		t.Fatalf("unexpected span hex: %s", SpanIDToHex(sid))
	}
}

func TestParseTraceparentString(t *testing.T) {
	in := "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01"
	tidHex, sidHex := ParseTraceparentString(in)
	if tidHex != "0123456789abcdef0123456789abcdef" || sidHex != "0123456789abcdef" {
		t.Fatalf("unexpected hex: %q %q", tidHex, sidHex)
	}
	// invalid -> empty strings
	tidHex, sidHex = ParseTraceparentString("bad")
	if tidHex != "" || sidHex != "" {
		t.Fatalf("expected empty for invalid, got %q %q", tidHex, sidHex)
	}
}

func TestTraceIDSpanIDHexRoundtrip(t *testing.T) {
	tid := [2]uint64{1, 2}
	sid := uint64(3)
	tid2 := TraceIDFromHex(TraceIDToHex(tid))
	if tid2 != tid {
		t.Fatalf("TraceID roundtrip mismatch: %v vs %v", tid2, tid)
	}
	sid2 := SpanIDFromHex(SpanIDToHex(sid))
	if sid2 != sid {
		t.Fatalf("SpanID roundtrip mismatch: %v vs %v", sid2, sid)
	}

	if got := TraceIDFromHex("short"); !TraceIDIsZero(got) {
		t.Fatalf("expected zero for invalid TraceIDFromHex")
	}
	if got := SpanIDFromHex("short"); got != 0 {
		t.Fatalf("expected zero for invalid SpanIDFromHex")
	}
}

func TestGenerateAndParseOrGenerate(t *testing.T) {
	// ensure zid initialized for span id generation
	zid.InitFast(1)

	tid := GenerateTraceID()
	if TraceIDIsZero(tid) {
		t.Fatalf("expected non-zero trace id")
	}
	sid := GenerateSpanID()
	if sid == 0 {
		t.Fatalf("expected non-zero span id")
	}

	// invalid -> generated
	t2, s2 := ParseOrGenerate("")
	if TraceIDIsZero(t2) || s2 == 0 {
		t.Fatalf("expected generated ids")
	}
	// valid -> parsed
	in := "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01"
	t3, s3 := ParseOrGenerate(in)
	if TraceIDToHex(t3) != "0123456789abcdef0123456789abcdef" || SpanIDToHex(s3) != "0123456789abcdef" {
		t.Fatalf("expected parsed ids")
	}
}

func TestExtractFromHTTPRequest(t *testing.T) {
	if tid, sid := ExtractFromHTTPRequest(nil); !TraceIDIsZero(tid) || sid != 0 {
		t.Fatalf("expected zero for nil request")
	}
	r, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	r.Header.Set(TraceparentHeader, "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01")
	tid, sid := ExtractFromHTTPRequest(r)
	if TraceIDToHex(tid) != "0123456789abcdef0123456789abcdef" || SpanIDToHex(sid) != "0123456789abcdef" {
		t.Fatalf("unexpected extracted ids")
	}
}

func TestExtractOrGenerateFromHTTPRequest(t *testing.T) {
	zid.InitFast(2)
	// nil -> generate
	tid, sid := ExtractOrGenerateFromHTTPRequest(nil)
	if TraceIDIsZero(tid) || sid == 0 {
		t.Fatalf("expected generated ids")
	}
}

func TestLogFields(t *testing.T) {
	if got := LogFields([2]uint64{}, 0); got != nil {
		t.Fatalf("expected nil")
	}
	// trace only
	if got := LogFields([2]uint64{1, 2}, 0); len(got) != 1 {
		t.Fatalf("expected 1 field for trace only, got %d", len(got))
	}
	// span only
	if got := LogFields([2]uint64{}, 3); len(got) != 1 {
		t.Fatalf("expected 1 field for span only, got %d", len(got))
	}
	// both
	if got := LogFields([2]uint64{1, 2}, 3); len(got) != 2 {
		t.Fatalf("expected 2 fields for both, got %d", len(got))
	}
}
