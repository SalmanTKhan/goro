package buildinfo

import "testing"

func TestVersionNonEmpty(t *testing.T) {
	if Version() == "" {
		t.Fatal("Version() returned empty string")
	}
}

func TestVersionUsesGenerated(t *testing.T) {
	if version == "" {
		t.Skip("version_gen.go not generated in this build")
	}
	if got := Version(); got != version {
		t.Fatalf("Version() = %q, want generated value %q", got, version)
	}
}
