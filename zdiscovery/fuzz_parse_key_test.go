package zdiscovery

import (
	"strconv"
	"strings"
	"testing"
)

func clampKey(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func FuzzParseKeyToActorType_NoPanic(f *testing.F) {
	f.Add("")
	f.Add("/servers/1")
	f.Add("servers/877001")
	f.Add("/servers")
	f.Add("/servers/")
	f.Add("/bad/1")

	f.Fuzz(func(t *testing.T, key string) {
		key = clampKey(key, 512)

		// Expected behavior mirrors parseKeyToActorType implementation:
		// - Trim leading '/'
		// - Must start with "servers/"
		// - Parse the substring after last '/' as uint32
		expectedKey := strings.TrimPrefix(key, "/")
		if !strings.HasPrefix(expectedKey, "servers/") {
			// Fast path: mismatch should always be (0,false)
			gotType, gotOk := parseKeyToActorType(key)
			if gotOk || gotType != 0 {
				t.Fatalf("unexpected ok/type: gotOk=%v gotType=%d key=%q", gotOk, gotType, key)
			}
			return
		}

		idx := strings.LastIndex(expectedKey, "/")
		if idx < 0 {
			gotType, gotOk := parseKeyToActorType(key)
			if gotOk || gotType != 0 {
				t.Fatalf("unexpected ok/type: gotOk=%v gotType=%d key=%q", gotOk, gotType, key)
			}
			return
		}

		n, err := strconv.ParseUint(expectedKey[idx+1:], 10, 32)
		wantOk := err == nil
		wantType := uint32(0)
		if wantOk {
			wantType = uint32(n)
		}

		gotType, gotOk := parseKeyToActorType(key)
		if gotOk != wantOk || gotType != wantType {
			t.Fatalf("mismatch: got=(%v,%d) want=(%v,%d) key=%q", gotOk, gotType, wantOk, wantType, key)
		}
	})
}
