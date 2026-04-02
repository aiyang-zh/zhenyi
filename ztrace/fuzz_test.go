package ztrace

import "testing"

func clampString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func FuzzParseTraceparent_NoPanic(f *testing.F) {
	f.Add("")
	f.Add("00-0123456789abcdef0123456789abcdef-0123456789abcdef-01")
	f.Add("00-xyz-abc-01")
	f.Add("bad")

	f.Fuzz(func(t *testing.T, s string) {
		s = clampString(s, 512)
		// invalid formats must return zero values and must not panic.
		_, _ = ParseTraceparent(s)
		_, _ = ParseTraceparentString(s)
	})
}

func FuzzTraceIDSpanIDFromHex_NoPanic(f *testing.F) {
	f.Add("")
	f.Add("0123456789abcdef0123456789abcdef")
	f.Add("short")
	f.Add("not-hex-xxxxxxxxxxxxxxxxxxxxxxxx")

	f.Fuzz(func(t *testing.T, s string) {
		s = clampString(s, 512)
		_ = TraceIDFromHex(s)
		_ = SpanIDFromHex(s)
	})
}
