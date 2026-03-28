package zpyroscope

import "testing"

func TestNormalize_setsLoggerAndProfiles(t *testing.T) {
	cfg := Config{
		ApplicationName: "test",
		ServerAddress:   "http://127.0.0.1:4040",
	}
	out := normalize(cfg)
	if out.Logger == nil {
		t.Fatal("expected default Logger")
	}
	if len(out.ProfileTypes) == 0 {
		t.Fatal("expected default ProfileTypes")
	}
}
