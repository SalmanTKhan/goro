//go:build (windows || linux || darwin) && !android

package gamepad

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Zyko0/go-sdl3/sdl"
)

func TestSDL3CandidatesPreferBundledOverSystem(t *testing.T) {
	t.Setenv(sdl3PathEnv, "")
	candidates := sdl3Candidates()
	if len(candidates) < 2 {
		t.Fatalf("expected several candidates, got %v", candidates)
	}
	// The bare library name must come last so an archive's own copy wins and
	// Windows cannot load an SDL3.dll planted in the working directory.
	if last := candidates[len(candidates)-1]; last != sdl.Path() {
		t.Fatalf("last candidate = %q want the bare name %q", last, sdl.Path())
	}
	for _, candidate := range candidates[:len(candidates)-1] {
		if candidate == "" {
			continue
		}
		if !filepath.IsAbs(candidate) {
			t.Fatalf("candidate %q before the bare name is not absolute", candidate)
		}
	}
}

func TestSDL3CandidatesHonorEnvOverride(t *testing.T) {
	override := filepath.Join(os.TempDir(), "custom-sdl3")
	t.Setenv(sdl3PathEnv, override)
	candidates := sdl3Candidates()
	if len(candidates) == 0 || candidates[0] != override {
		t.Fatalf("override not first: %v", candidates)
	}
}

func TestLoadSDL3ReportsEveryAttempt(t *testing.T) {
	// A missing runtime must produce an error naming what was tried, since the
	// renderer only logs it and continues without controller support.
	t.Setenv(sdl3PathEnv, filepath.Join(os.TempDir(), "definitely-not-sdl3"))
	err := loadSDL3()
	if err == nil {
		t.Skip("an SDL3 runtime is installed on this machine")
	}
	if !strings.Contains(err.Error(), "definitely-not-sdl3") {
		t.Fatalf("error does not name the attempted path: %v", err)
	}
}
