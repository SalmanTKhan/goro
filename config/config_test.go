package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kivutar/goro/input"
)

func isolateUserConfig(t *testing.T) {
	t.Helper()
	configRoot := t.TempDir()
	t.Setenv("APPDATA", configRoot)
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatal(err)
		}
	})
}

func TestNextCapturePathUsesFormatAndCollisionSuffix(t *testing.T) {
	isolateUserConfig(t)
	now := time.Date(2026, time.September, 3, 14, 5, 6, 0, time.UTC)
	first, err := NextCapturePath(now, ".webm")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(first) != ".webm" || !strings.HasSuffix(first, "goro-20260903-140506.webm") {
		t.Fatalf("first capture path = %q", first)
	}
	if err := os.WriteFile(first, []byte("placeholder"), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := NextCapturePath(now, "webm")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(second, "goro-20260903-140506-02.webm") {
		t.Fatalf("collision capture path = %q", second)
	}
}

func TestLoadConfigReadsINIAndCLIOverrides(t *testing.T) {
	isolateUserConfig(t)
	root := t.TempDir()
	dataDir := filepath.Join(root, "OldRO")
	if err := os.Mkdir(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, "goro.ini")
	if err := os.WriteFile(configPath, []byte(`
data_dir = ./ignored

[window]
width = 1024
height = 768
fullscreen = true

[packet]
client_date = 20211103

[login]
char_slot = 2

[audio]
bgm = false
bgm_volume = 0.25
sfx_volume = 0.35

[render]
graphics_api = gles
vsync = false
fps = true
no_ui = true
async_ui = false
profile_ui = false

[capture]
ffmpeg_path = ./ffmpeg-from-ini.exe

[network]
trace = true

[fog]
enabled = false

[gameplay]
no_shift = true
no_ctrl = false
mineffect = true
snap = true
itemsnap = false
force_user_ai = false

[mobile]
movement = tap
camera_sensitivity = 1.50
zoom_sensitivity = 0.75
invert_camera_y = true
long_press_ms = 900
show_target_names = false
show_minimap = false

[script]
path = ./ignored.lua

[log]
level = warn
file = ./ignored.log
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig([]string{
		"--config", configPath,
		"--data-dir", dataDir,
		"--width", "1280",
		"--fullscreen=false",
		"--bgm=true",
		"--no-audio=true",
		"--bgm-volume", "0.75",
		"--sfx-volume", "0.85",
		"--graphics-api", "vulkan",
		"--no-ui=false",
		"--async-ui=true",
		"--profile-ui=true",
		"--ffmpeg-path", filepath.Join(root, "ffmpeg.exe"),
		"--char-slot", "3",
		"--no-shift=false",
		"--no-ctrl=true",
		"--mineffect=false",
		"--snap=false",
		"--itemsnap=true",
		"--force-user-ai=true",
		"--script", filepath.Join(root, "bot.lua"),
		"--log-level", "debug",
		"--log-file", filepath.Join(root, "goro.log"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DataDir != dataDir {
		t.Fatalf("data dir = %q, want %q", cfg.DataDir, dataDir)
	}
	if cfg.Window.Width != 1280 || cfg.Window.Height != 768 || cfg.Window.Fullscreen {
		t.Fatalf("unexpected window config: %#v", cfg.Window)
	}
	if cfg.Packet.ClientDate != 20211103 {
		t.Fatalf("packet client date = %d", cfg.Packet.ClientDate)
	}
	if cfg.Login.CharSlot != 3 {
		t.Fatalf("login char slot = %d, want 3", cfg.Login.CharSlot)
	}
	if !cfg.Audio.Disabled || !cfg.Audio.BGM || cfg.Audio.BGMVolume != 0.75 || cfg.Audio.SFXVolume != 0.85 {
		t.Fatalf("unexpected audio config: %#v", cfg.Audio)
	}
	if cfg.Render.GraphicsAPI != "vulkan" || cfg.Render.VSync || !cfg.Render.FPS || cfg.Render.NoUI || !cfg.Render.AsyncUI || !cfg.Render.UIProfile {
		t.Fatalf("unexpected render config: %#v", cfg.Render)
	}
	if cfg.Capture.FFmpegPath != filepath.Join(root, "ffmpeg.exe") {
		t.Fatalf("ffmpeg path = %q, want CLI override", cfg.Capture.FFmpegPath)
	}
	if !cfg.Network.Trace {
		t.Fatalf("network trace = false, want true")
	}
	if cfg.Fog.Enabled {
		t.Fatalf("fog enabled = true, want false")
	}
	if cfg.Gameplay.NoShift || !cfg.Gameplay.NoCtrl || cfg.Gameplay.LessEffects || cfg.Gameplay.SnapTargets || !cfg.Gameplay.SnapItems || !cfg.Gameplay.ForceUserAI {
		t.Fatalf("unexpected gameplay config: %#v", cfg.Gameplay)
	}
	if cfg.Mobile.MovementMode != input.MovementTapToMove || cfg.Mobile.CameraSensitivity != 1.5 || cfg.Mobile.ZoomSensitivity != 0.75 || !cfg.Mobile.InvertCameraY || cfg.Mobile.LongPressMS != 900 || cfg.Mobile.ShowTargetNames {
		t.Fatalf("unexpected mobile config: %#v", cfg.Mobile)
	}
	if cfg.MobileDisplay.ShowMinimap {
		t.Fatalf("show minimap = true, want false")
	}
	if cfg.Script.Path != filepath.Join(root, "bot.lua") {
		t.Fatalf("script path = %q", cfg.Script.Path)
	}
	if cfg.Log.Level != "debug" || cfg.Log.File != filepath.Join(root, "goro.log") {
		t.Fatalf("unexpected log config: %#v", cfg.Log)
	}
}

func TestLoadConfigWindowedOverridesFullscreenINI(t *testing.T) {
	isolateUserConfig(t)
	root := t.TempDir()
	configPath := filepath.Join(root, "goro.ini")
	if err := os.WriteFile(configPath, []byte("[window]\nfullscreen = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig([]string{"--config", configPath, "--windowed"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Window.Fullscreen {
		t.Fatal("fullscreen = true, want false")
	}
}

func TestLoadConfigRejectsInvalidCharacterSlot(t *testing.T) {
	isolateUserConfig(t)
	if _, err := LoadConfig([]string{"--char-slot", "9"}); err == nil {
		t.Fatal("expected invalid character slot error")
	}
}

func TestLoadConfigRejectsInvalidLogLevel(t *testing.T) {
	isolateUserConfig(t)
	if _, err := LoadConfig([]string{"--log-level", "verbose"}); err == nil {
		t.Fatal("expected invalid log level error")
	}
}

func TestLoadConfigReadsUserConfig(t *testing.T) {
	isolateUserConfig(t)
	path, err := UserConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`
[window]
fullscreen = true

[audio]
bgm_volume = 0.10
sfx_volume = 0.20

[render]
vsync = false
fps = true
async_ui = true
profile_ui = true

[gameplay]
no_shift = true
no_ctrl = false
less_effects = true
snap = true
itemsnap = true
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Window.Fullscreen || cfg.Audio.BGMVolume != 0.10 || cfg.Audio.SFXVolume != 0.20 || cfg.Render.VSync || !cfg.Render.FPS || !cfg.Render.AsyncUI || !cfg.Render.UIProfile || !cfg.Gameplay.NoShift || cfg.Gameplay.NoCtrl || !cfg.Gameplay.LessEffects || !cfg.Gameplay.SnapTargets || !cfg.Gameplay.SnapItems {
		t.Fatalf("user config not loaded: %#v", cfg)
	}
}

func TestSaveUserSettingsPreservesUnrelatedINI(t *testing.T) {
	isolateUserConfig(t)
	path, err := UserConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	initial := `data_dir = /tmp/OldRO

[login]
username = Kivutar

[window]
width = 1024
fullscreen = false
`
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}
	writtenPath, err := SaveUserSettings(UserSettings{
		Fullscreen:  true,
		VSync:       false,
		FPS:         true,
		BGMVolume:   0.33,
		SFXVolume:   0.44,
		NoShift:     true,
		NoCtrl:      false,
		LessEffects: true,
		SnapTargets: true,
		SnapItems:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if writtenPath != path {
		t.Fatalf("written path = %q, want %q", writtenPath, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"data_dir = /tmp/OldRO",
		"username = Kivutar",
		"width = 1024",
		"fullscreen = true",
		"vsync = false",
		"fps = true",
		"bgm_volume = 0.33",
		"sfx_volume = 0.44",
		"no_shift = true",
		"no_ctrl = false",
		"less_effects = true",
		"snap = true",
		"itemsnap = true",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("saved config missing %q:\n%s", want, text)
		}
	}
}

func TestMobileSettingsRoundTripPreservesUnrelatedINI(t *testing.T) {
	isolateUserConfig(t)
	path, err := UserConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("[login]\nusername = KeepMe\n\n[audio]\nbgm_volume = 0.33\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	controls := input.DefaultMobileControls()
	controls.MovementMode = input.MovementTapToMove
	controls.CameraSensitivity = 1.5
	controls.ZoomSensitivity = 0.75
	controls.InvertCameraY = true
	controls.LongPressMS = 900
	controls.ShowTargetNames = false
	if _, err := SaveMobileControls(controls); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadUserMobileControls()
	if err != nil {
		t.Fatal(err)
	}
	if loaded != controls {
		t.Fatalf("loaded mobile controls = %#v, want %#v", loaded, controls)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"username = KeepMe", "bgm_volume = 0.33", "movement = tap", "camera_sensitivity = 1.50", "zoom_sensitivity = 0.75", "invert_camera_y = true", "long_press_ms = 900", "show_target_names = false"} {
		if !strings.Contains(text, want) {
			t.Fatalf("saved config missing %q:\n%s", want, text)
		}
	}
}

func TestMobileSettingsRoundTripPreservesAllSupportedValues(t *testing.T) {
	isolateUserConfig(t)
	path, err := UserConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("[login]\nusername = KeepMe\n\n[network]\ntrace = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	settings := input.DefaultMobileSettings()
	settings.Controls.MovementMode = input.MovementTapToMove
	settings.Controls.CameraSensitivity = 2
	settings.Controls.LongPressMS = 900
	settings.Controls.ShowTargetNames = false
	settings.Audio.BGMEnabled = false
	settings.Audio.BGMVolume = 0.25
	settings.Audio.SFXVolume = 0.75
	settings.UI.Scale = input.UIScaleLargeValue
	settings.Display.ShowMinimap = false
	settings.Gameplay.NoShift = true
	settings.Gameplay.NoCtrl = false
	settings.Gameplay.LessEffects = true
	settings.Gameplay.SnapTargets = true
	settings.Gameplay.SnapItems = true
	if _, err := SaveMobileSettings(settings); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadUserMobileSettings()
	if err != nil {
		t.Fatal(err)
	}
	if loaded != settings {
		t.Fatalf("loaded mobile settings = %#v, want %#v", loaded, settings)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"username = KeepMe", "trace = true", "scale = 1.20", "movement = tap", "bgm = false", "bgm_volume = 0.25", "sfx_volume = 0.75", "show_minimap = false", "no_shift = true", "itemsnap = true"} {
		if !strings.Contains(text, want) {
			t.Fatalf("saved config missing %q:\n%s", want, text)
		}
	}
}
