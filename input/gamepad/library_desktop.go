//go:build (windows || linux || darwin) && !android

package gamepad

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/kivutar/goro/glog"
)

// sdl3PathEnv lets a packager or developer point at a specific SDL3 build.
const sdl3PathEnv = "GORO_SDL3_PATH"

// loadSDL3 finds the SDL3 runtime. Absolute paths beside the executable are
// tried before the bare library name so a release archive uses the copy it
// shipped with, and so Windows cannot be induced to load an SDL3.dll planted in
// the working directory. The bare name comes last, which is what lets Linux
// prefer the distribution's own libSDL3.so.0.
func loadSDL3() error {
	var attempts []error
	for _, candidate := range sdl3Candidates() {
		if candidate == "" {
			continue
		}
		if err := sdl.LoadLibrary(candidate); err != nil {
			attempts = append(attempts, fmt.Errorf("%s: %w", candidate, err))
			continue
		}
		glog.Debugf("loaded SDL3 runtime from %s", candidate)
		return nil
	}
	if len(attempts) == 0 {
		return errors.New("no SDL3 runtime candidates for this platform")
	}
	return fmt.Errorf("load SDL3 gamepad library: %w", errors.Join(attempts...))
}

func sdl3Candidates() []string {
	name := sdl.Path()
	if name == "" {
		return nil
	}
	candidates := []string{os.Getenv(sdl3PathEnv)}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(dir, name),
			filepath.Join(dir, "lib", name),
		)
		if runtime.GOOS == "darwin" {
			// Inside a .app bundle the executable sits in Contents/MacOS and
			// libraries in Contents/Frameworks.
			candidates = append(candidates, filepath.Join(dir, "..", "Frameworks", name))
		}
	}
	// The bare name last, resolved by the platform loader against the system
	// library path.
	return append(candidates, name)
}
