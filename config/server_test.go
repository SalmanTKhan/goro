package config

import (
	"strings"
	"testing"
)

func TestServerConfigNormalizesDefaults(t *testing.T) {
	s := (ServerConfig{}).Normalized()
	if s.Host != "" || s.AuthPort != 6900 || s.CharPort != 6121 || s.ZonePort != 5121 || s.ClientDate != 20080910 {
		t.Fatalf("normalized server = %+v", s)
	}
	if s.Name != "Local server" {
		t.Fatalf("name = %q", s.Name)
	}
}

func TestMobileSessionConfigFromINI(t *testing.T) {
	cfg := defaultConfig()
	data := "[mobile]\nmode = online\n\n[server]\nhost = sabine.local\nauth_port = 7000\nchar_port = 6121\nzone_port = 5121\n"
	if err := applyINI(&cfg, strings.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	if cfg.MobileSession.Mode != SessionModeOnline || cfg.MobileSession.Server.Host != "sabine.local" || cfg.MobileSession.Server.AuthPort != 7000 {
		t.Fatalf("mobile session = %+v", cfg.MobileSession)
	}
}
