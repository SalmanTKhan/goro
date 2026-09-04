package config

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kivutar/goro/input"
)

type Config struct {
	DataDir       string
	Window        WindowConfig
	Packet        PacketConfig
	Login         LoginConfig
	Audio         AudioConfig
	Render        RenderConfig
	Network       NetworkConfig
	MobileSession MobileSessionConfig
	Fog           FogConfig
	Gameplay      GameplayConfig
	UI            input.UISettings
	Mobile        input.MobileControls
	MobileDisplay input.MobileDisplaySettings
	Script        ScriptConfig
	Log           LogConfig
}

type WindowConfig struct {
	Title      string
	Width      int
	Height     int
	Fullscreen bool
}

type PacketConfig struct {
	ClientDate int
	Profile    int
}

type LoginConfig struct {
	Username  string
	Password  string
	AutoLogin bool
	CharSlot  int
}

type AudioConfig struct {
	Disabled  bool
	BGM       bool
	BGMVolume float64
	SFXVolume float64
}

type RenderConfig struct {
	GraphicsAPI        string
	VSync              bool
	FPS                bool
	NoUI               bool
	AsyncUI            bool
	UIProfile          bool
	BenchSeconds       int
	BenchWarmupSeconds int
	CPUProfile         string
	Stats              bool
	WorldDebugStats    bool
}

type NetworkConfig struct {
	Trace bool
}

type MobileSessionConfig struct {
	Mode   SessionMode
	Server ServerConfig
}

type FogConfig struct {
	Enabled bool
}

type GameplayConfig struct {
	NoShift     bool
	NoCtrl      bool
	LessEffects bool
	SnapTargets bool
	SnapItems   bool
	ForceUserAI bool
}

type ScriptConfig struct {
	Path string
}

type LogConfig struct {
	Level string
	File  string
}

func LoadConfig(args []string) (Config, error) {
	cfg := defaultConfig()

	if path, err := UserConfigPath(); err == nil {
		if err := applyINIFile(&cfg, path, false); err != nil {
			return Config{}, err
		}
	}
	configPath, explicitConfig := configPathFromArgs(args)
	if configPath != "" {
		if err := applyINIFile(&cfg, configPath, explicitConfig); err != nil {
			return Config{}, err
		}
	}
	if err := applyCLI(&cfg, args); err != nil {
		return Config{}, err
	}
	cfg.DataDir = resolveDataDir(cfg.DataDir)
	return cfg, nil
}

type UserSettings struct {
	UIScale       float32
	Fullscreen    bool
	VSync         bool
	FPS           bool
	BGMVolume     float64
	SFXVolume     float64
	NoShift       bool
	NoCtrl        bool
	LessEffects   bool
	SnapTargets   bool
	SnapItems     bool
	Mobile        *input.MobileControls
	MobileDisplay *input.MobileDisplaySettings
}

func UserConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "goro", "goro.ini"), nil
}

func UserDataDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "goro"), nil
}

// LoadUserMobileControls reads only the mobile controls portion of the user
// configuration. Android uses this because it constructs its platform host
// configuration separately from the desktop command-line configuration.
func LoadUserMobileControls() (input.MobileControls, error) {
	cfg := defaultConfig()
	path, err := UserConfigPath()
	if err != nil {
		return cfg.Mobile, err
	}
	if err := applyINIFile(&cfg, path, false); err != nil {
		return input.MobileControls{}, err
	}
	return cfg.Mobile, nil
}

// LoadUserMobileSettings reads the complete set of settings exposed by the
// mobile page. It starts from the same validated defaults as LoadConfig, so a
// legacy INI without newer mobile keys remains safe.
func LoadUserMobileSettings() (input.MobileSettings, error) {
	cfg := defaultConfig()
	path, err := UserConfigPath()
	if err != nil {
		return input.DefaultMobileSettings(), err
	}
	if err := applyINIFile(&cfg, path, false); err != nil {
		return input.MobileSettings{}, err
	}
	return input.MobileSettings{
		UI:       cfg.UI,
		Controls: cfg.Mobile,
		Audio: input.MobileAudioSettings{
			BGMEnabled: cfg.Audio.BGM,
			BGMVolume:  cfg.Audio.BGMVolume,
			SFXVolume:  cfg.Audio.SFXVolume,
		},
		Display:  cfg.MobileDisplay,
		Gameplay: input.MobileGameplaySettings{NoShift: cfg.Gameplay.NoShift, NoCtrl: cfg.Gameplay.NoCtrl, LessEffects: cfg.Gameplay.LessEffects, SnapTargets: cfg.Gameplay.SnapTargets, SnapItems: cfg.Gameplay.SnapItems},
	}.Normalized(), nil
}

func NextScreenshotPath(now time.Time) (string, error) {
	dir, err := UserDataDir()
	if err != nil {
		return "", err
	}
	dir = filepath.Join(dir, "screenshots")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("goro-%s.png", now.Format("20060102-150405"))
	path := filepath.Join(dir, name)
	for i := 2; ; i++ {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return path, nil
		} else if err != nil {
			return "", err
		}
		name = fmt.Sprintf("goro-%s-%02d.png", now.Format("20060102-150405"), i)
		path = filepath.Join(dir, name)
	}
}

func SaveUserSettings(settings UserSettings) (string, error) {
	if settings.BGMVolume < 0 || settings.BGMVolume > 1 {
		return "", fmt.Errorf("bgm volume must be between 0 and 1")
	}
	if settings.SFXVolume < 0 || settings.SFXVolume > 1 {
		return "", fmt.Errorf("sfx volume must be between 0 and 1")
	}
	path, err := UserConfigPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	values := map[string]map[string]string{
		"ui": {
			"scale": formatINIValueFloat(float64(input.UISettings{Scale: settings.UIScale}.Normalized().Scale)),
		},
		"window": {
			"fullscreen": formatINIValueBool(settings.Fullscreen),
		},
		"render": {
			"vsync": formatINIValueBool(settings.VSync),
			"fps":   formatINIValueBool(settings.FPS),
		},
		"audio": {
			"bgm_volume": formatINIValueFloat(settings.BGMVolume),
			"sfx_volume": formatINIValueFloat(settings.SFXVolume),
		},
		"gameplay": {
			"no_shift":     formatINIValueBool(settings.NoShift),
			"no_ctrl":      formatINIValueBool(settings.NoCtrl),
			"less_effects": formatINIValueBool(settings.LessEffects),
			"snap":         formatINIValueBool(settings.SnapTargets),
			"itemsnap":     formatINIValueBool(settings.SnapItems),
		},
	}
	if settings.Mobile != nil {
		mobile := settings.Mobile.Normalized()
		values["mobile"] = map[string]string{
			"movement":           formatINIMobileMovement(mobile.MovementMode),
			"camera_sensitivity": formatINIValueFloat(mobile.CameraSensitivity),
			"zoom_sensitivity":   formatINIValueFloat(mobile.ZoomSensitivity),
			"invert_camera_y":    formatINIValueBool(mobile.InvertCameraY),
			"long_press_ms":      strconv.Itoa(mobile.LongPressMS),
			"show_target_names":  formatINIValueBool(mobile.ShowTargetNames),
		}
	}
	if settings.MobileDisplay != nil {
		values["mobile"] = ensureINISection(values["mobile"])
		values["mobile"]["show_minimap"] = formatINIValueBool(settings.MobileDisplay.ShowMinimap)
		values["mobile"]["presentation"] = string(settings.MobileDisplay.Presentation)
	}
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	data := upsertINIValues(string(existing), values)
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// SaveMobileControls updates only the mobile section of the user INI. This is
// used by the Android host so changing a touch preference cannot reset
// unrelated desktop, audio, or login settings.
func SaveMobileControls(controls input.MobileControls) (string, error) {
	controls = controls.Normalized()
	path, err := UserConfigPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	values := map[string]map[string]string{"mobile": {
		"movement":           formatINIMobileMovement(controls.MovementMode),
		"camera_sensitivity": formatINIValueFloat(controls.CameraSensitivity),
		"zoom_sensitivity":   formatINIValueFloat(controls.ZoomSensitivity),
		"invert_camera_y":    formatINIValueBool(controls.InvertCameraY),
		"long_press_ms":      strconv.Itoa(controls.LongPressMS),
		"show_target_names":  formatINIValueBool(controls.ShowTargetNames),
	}}
	data := upsertINIValues(string(existing), values)
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// SaveMobileSettings updates only the sections represented by the mobile
// settings page. Existing login/server and unrelated INI entries remain
// untouched.
func SaveMobileSettings(settings input.MobileSettings) (string, error) {
	settings = settings.Normalized()
	path, err := UserConfigPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	values := map[string]map[string]string{
		"ui": {"scale": formatINIValueFloat(float64(settings.UI.Normalized().Scale))},
		"mobile": {
			"movement":           formatINIMobileMovement(settings.Controls.MovementMode),
			"camera_sensitivity": formatINIValueFloat(settings.Controls.CameraSensitivity),
			"zoom_sensitivity":   formatINIValueFloat(settings.Controls.ZoomSensitivity),
			"invert_camera_y":    formatINIValueBool(settings.Controls.InvertCameraY),
			"long_press_ms":      strconv.Itoa(settings.Controls.LongPressMS),
			"show_target_names":  formatINIValueBool(settings.Controls.ShowTargetNames),
			"show_minimap":       formatINIValueBool(settings.Display.ShowMinimap),
			"presentation":       string(settings.Display.Presentation),
		},
		"audio": {
			"bgm":        formatINIValueBool(settings.Audio.BGMEnabled),
			"bgm_volume": formatINIValueFloat(settings.Audio.BGMVolume),
			"sfx_volume": formatINIValueFloat(settings.Audio.SFXVolume),
		},
		"gameplay": {
			"no_shift":     formatINIValueBool(settings.Gameplay.NoShift),
			"no_ctrl":      formatINIValueBool(settings.Gameplay.NoCtrl),
			"less_effects": formatINIValueBool(settings.Gameplay.LessEffects),
			"snap":         formatINIValueBool(settings.Gameplay.SnapTargets),
			"itemsnap":     formatINIValueBool(settings.Gameplay.SnapItems),
		},
	}
	data := upsertINIValues(string(existing), values)
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func ensureINISection(values map[string]string) map[string]string {
	if values == nil {
		return map[string]string{}
	}
	return values
}

func defaultConfig() Config {
	return Config{
		Window: WindowConfig{
			Title:      "goro",
			Width:      1280,
			Height:     720,
			Fullscreen: false,
		},
		Packet: PacketConfig{
			ClientDate: 20080910,
			Profile:    23,
		},
		MobileSession: MobileSessionConfig{Mode: SessionModeOffline, Server: ServerConfig{Host: "127.0.0.1", AuthPort: 6900, CharPort: 6121, ZonePort: 5121, ClientDate: 20080910, Profile: 23}},
		Login: LoginConfig{
			CharSlot: -1,
		},
		Audio: AudioConfig{
			BGM:       true,
			BGMVolume: 0.55,
			SFXVolume: 0.55,
		},
		Render: RenderConfig{
			GraphicsAPI:        "vulkan",
			AsyncUI:            true,
			VSync:              true,
			BenchWarmupSeconds: 0,
		},
		Fog: FogConfig{
			Enabled: true,
		},
		Gameplay: GameplayConfig{
			NoCtrl: true,
		},
		UI:            input.DefaultUISettings(),
		Mobile:        input.DefaultMobileControls(),
		MobileDisplay: input.MobileDisplaySettings{ShowMinimap: true, Presentation: input.MobilePresentationMobileUI},
		Log: LogConfig{
			Level: "info",
		},
	}
}

func configPathFromArgs(args []string) (string, bool) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--config" && i+1 < len(args) {
			return args[i+1], true
		}
		if strings.HasPrefix(arg, "--config=") {
			return strings.TrimPrefix(arg, "--config="), true
		}
	}
	if _, err := os.Stat("goro.ini"); err == nil {
		return "goro.ini", false
	}
	return "", false
}

func applyINIFile(cfg *Config, path string, explicit bool) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) && !explicit {
			return nil
		}
		return fmt.Errorf("open config %s: %w", path, err)
	}
	defer file.Close()

	if err := applyINI(cfg, file); err != nil {
		return fmt.Errorf("read config %s: %w", path, err)
	}
	return nil
}

func applyCLI(cfg *Config, args []string) error {
	fs := flag.NewFlagSet("goro", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	configPath := ""
	windowed := false
	fs.StringVar(&configPath, "config", "", "path to goro ini configuration")
	fs.StringVar(&cfg.DataDir, "data-dir", cfg.DataDir, "Ragnarok data directory")
	fs.StringVar(&cfg.Window.Title, "title", cfg.Window.Title, "window title")
	fs.IntVar(&cfg.Window.Width, "width", cfg.Window.Width, "window width")
	fs.IntVar(&cfg.Window.Height, "height", cfg.Window.Height, "window height")
	fs.BoolVar(&cfg.Window.Fullscreen, "fullscreen", cfg.Window.Fullscreen, "start in fullscreen mode")
	fs.BoolVar(&windowed, "windowed", false, "force windowed mode")
	fs.IntVar(&cfg.Packet.ClientDate, "packet-client-date", cfg.Packet.ClientDate, "packet client date")
	fs.IntVar(&cfg.Packet.Profile, "packet-profile", cfg.Packet.Profile, "packet profile")
	fs.StringVar(&cfg.Login.Username, "username", cfg.Login.Username, "login username")
	fs.StringVar(&cfg.Login.Password, "password", cfg.Login.Password, "login password")
	fs.BoolVar(&cfg.Login.AutoLogin, "autologin", cfg.Login.AutoLogin, "attempt login automatically")
	fs.IntVar(&cfg.Login.CharSlot, "char-slot", cfg.Login.CharSlot, "character slot to select after autologin, 0 to 8")
	fs.BoolVar(&cfg.Audio.BGM, "bgm", cfg.Audio.BGM, "enable BGM")
	fs.BoolVar(&cfg.Audio.Disabled, "no-audio", cfg.Audio.Disabled, "disable all audio output")
	fs.Float64Var(&cfg.Audio.BGMVolume, "bgm-volume", cfg.Audio.BGMVolume, "BGM volume from 0 to 1")
	fs.Float64Var(&cfg.Audio.SFXVolume, "sfx-volume", cfg.Audio.SFXVolume, "SFX volume from 0 to 1")
	fs.StringVar(&cfg.Render.GraphicsAPI, "graphics-api", cfg.Render.GraphicsAPI, "graphics API: auto, vulkan, dx12, metal, gles, software")
	fs.BoolVar(&cfg.Render.VSync, "vsync", cfg.Render.VSync, "enable vsync")
	fs.BoolVar(&cfg.Render.FPS, "fps", cfg.Render.FPS, "show measured FPS counter")
	fs.BoolVar(&cfg.Render.NoUI, "no-ui", cfg.Render.NoUI, "disable UI rendering for benchmarking")
	fs.BoolVar(&cfg.Render.AsyncUI, "async-ui", cfg.Render.AsyncUI, "rasterize UI off the draw thread")
	fs.BoolVar(&cfg.Render.UIProfile, "profile-ui", cfg.Render.UIProfile, "log aggregate UI frame and redraw timings")
	fs.IntVar(&cfg.Render.BenchSeconds, "bench-seconds", cfg.Render.BenchSeconds, "quit after benchmarking for this many seconds")
	fs.IntVar(&cfg.Render.BenchWarmupSeconds, "bench-warmup-seconds", cfg.Render.BenchWarmupSeconds, "benchmark warmup seconds")
	fs.StringVar(&cfg.Render.CPUProfile, "cpu-profile", cfg.Render.CPUProfile, "write CPU profile to this path during benchmark")
	fs.BoolVar(&cfg.Render.Stats, "render-stats", cfg.Render.Stats, "show render stats")
	fs.BoolVar(&cfg.Render.WorldDebugStats, "world-debug-stats", cfg.Render.WorldDebugStats, "show world renderer debug stats")
	fs.BoolVar(&cfg.Network.Trace, "net-trace", cfg.Network.Trace, "log network reads and writes")
	fs.StringVar((*string)(&cfg.MobileSession.Mode), "mobile-mode", string(cfg.MobileSession.Mode), "mobile session mode: online or offline")
	fs.StringVar(&cfg.MobileSession.Server.Host, "server-host", cfg.MobileSession.Server.Host, "online server host")
	fs.IntVar(&cfg.MobileSession.Server.AuthPort, "server-auth-port", cfg.MobileSession.Server.AuthPort, "online auth server port")
	fs.IntVar(&cfg.MobileSession.Server.CharPort, "server-char-port", cfg.MobileSession.Server.CharPort, "online char server port")
	fs.IntVar(&cfg.MobileSession.Server.ZonePort, "server-zone-port", cfg.MobileSession.Server.ZonePort, "online zone server port")
	fs.BoolVar(&cfg.Fog.Enabled, "fog", cfg.Fog.Enabled, "enable map fog")
	fs.BoolVar(&cfg.Gameplay.NoShift, "no-shift", cfg.Gameplay.NoShift, "allow support skills to target enemies without holding Shift")
	fs.BoolVar(&cfg.Gameplay.NoCtrl, "no-ctrl", cfg.Gameplay.NoCtrl, "keep attacking with one click without holding Ctrl")
	fs.BoolVar(&cfg.Gameplay.LessEffects, "mineffect", cfg.Gameplay.LessEffects, "use simpler visual effects")
	fs.BoolVar(&cfg.Gameplay.SnapTargets, "snap", cfg.Gameplay.SnapTargets, "magnetize attack and enemy skill cursors to targets")
	fs.BoolVar(&cfg.Gameplay.SnapItems, "itemsnap", cfg.Gameplay.SnapItems, "magnetize pickup cursor to floor items")
	fs.BoolVar(&cfg.Gameplay.SnapItems, "item-snap", cfg.Gameplay.SnapItems, "magnetize pickup cursor to floor items")
	fs.BoolVar(&cfg.Gameplay.ForceUserAI, "force-user-ai", cfg.Gameplay.ForceUserAI, "start companion AI in USER_AI/USER_AI_M custom mode")
	fs.StringVar(&cfg.Script.Path, "script", cfg.Script.Path, "Lua script to run while in game")
	fs.StringVar(&cfg.Log.Level, "log-level", cfg.Log.Level, "minimum log level: debug, info, warn, error, fatal")
	fs.StringVar(&cfg.Log.File, "log-file", cfg.Log.File, "append logs to this file")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if windowed {
		cfg.Window.Fullscreen = false
	}
	return validateConfig(cfg)
}

func applyINI(cfg *Config, r io.Reader) error {
	scanner := bufio.NewScanner(r)
	section := ""
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = normalizeKey(strings.TrimSpace(line[1 : len(line)-1]))
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("line %d: expected key=value", lineNo)
		}
		if err := applyConfigValue(cfg, section, normalizeKey(key), cleanINIValue(value)); err != nil {
			return fmt.Errorf("line %d: %w", lineNo, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return validateConfig(cfg)
}

func applyConfigValue(cfg *Config, section, key, value string) error {
	switch section + "." + key {
	case ".datadir", "data.dir", "data.datadir", "config.datadir", "core.datadir":
		cfg.DataDir = value
	case "window.title":
		cfg.Window.Title = value
	case "window.width":
		return setInt(value, &cfg.Window.Width)
	case "window.height":
		return setInt(value, &cfg.Window.Height)
	case "window.fullscreen":
		return setBool(value, &cfg.Window.Fullscreen)
	case "packet.clientdate":
		return setInt(value, &cfg.Packet.ClientDate)
	case "packet.profile":
		return setInt(value, &cfg.Packet.Profile)
	case "login.username":
		cfg.Login.Username = value
	case "login.password":
		cfg.Login.Password = value
	case "login.autologin":
		return setBool(value, &cfg.Login.AutoLogin)
	case "login.charslot":
		return setInt(value, &cfg.Login.CharSlot)
	case "audio.bgm":
		return setBool(value, &cfg.Audio.BGM)
	case "audio.noaudio":
		return setBool(value, &cfg.Audio.Disabled)
	case "audio.bgmvolume":
		return setFloat(value, &cfg.Audio.BGMVolume)
	case "audio.sfxvolume":
		return setFloat(value, &cfg.Audio.SFXVolume)
	case "render.graphicsapi":
		cfg.Render.GraphicsAPI = value
	case "render.vsync":
		return setBool(value, &cfg.Render.VSync)
	case "render.fps":
		return setBool(value, &cfg.Render.FPS)
	case "render.noui":
		return setBool(value, &cfg.Render.NoUI)
	case "render.asyncui":
		return setBool(value, &cfg.Render.AsyncUI)
	case "render.profileui":
		return setBool(value, &cfg.Render.UIProfile)
	case "render.benchseconds":
		return setInt(value, &cfg.Render.BenchSeconds)
	case "render.benchwarmupseconds":
		return setInt(value, &cfg.Render.BenchWarmupSeconds)
	case "render.cpuprofile":
		cfg.Render.CPUProfile = value
	case "render.stats":
		return setBool(value, &cfg.Render.Stats)
	case "render.worlddebugstats":
		return setBool(value, &cfg.Render.WorldDebugStats)
	case "ui.scale":
		var scale float64
		if err := setFloat(value, &scale); err != nil {
			return err
		}
		cfg.UI.Scale = float32(scale)
		return nil
	case "network.trace":
		return setBool(value, &cfg.Network.Trace)
	case "mobile.mode":
		cfg.MobileSession.Mode = SessionMode(strings.ToLower(strings.TrimSpace(value)))
	case "server.host":
		cfg.MobileSession.Server.Host = value
	case "server.name":
		cfg.MobileSession.Server.Name = value
	case "server.authport":
		return setInt(value, &cfg.MobileSession.Server.AuthPort)
	case "server.charport":
		return setInt(value, &cfg.MobileSession.Server.CharPort)
	case "server.zoneport":
		return setInt(value, &cfg.MobileSession.Server.ZonePort)
	case "server.clientdate":
		return setInt(value, &cfg.MobileSession.Server.ClientDate)
	case "server.profile":
		return setInt(value, &cfg.MobileSession.Server.Profile)
	case "fog.enabled":
		return setBool(value, &cfg.Fog.Enabled)
	case "gameplay.noshift":
		return setBool(value, &cfg.Gameplay.NoShift)
	case "gameplay.noctrl":
		return setBool(value, &cfg.Gameplay.NoCtrl)
	case "gameplay.lesseffects", "gameplay.lesseffect", "gameplay.mineffect", "gameplay.less_effects", "gameplay.less_effect":
		return setBool(value, &cfg.Gameplay.LessEffects)
	case "gameplay.snap", "gameplay.snaptargets", "gameplay.targetsnap":
		return setBool(value, &cfg.Gameplay.SnapTargets)
	case "gameplay.itemsnap", "gameplay.snapitems", "gameplay.itemsnapping":
		return setBool(value, &cfg.Gameplay.SnapItems)
	case "gameplay.forceuserai":
		return setBool(value, &cfg.Gameplay.ForceUserAI)
	case "mobile.movement":
		return setMobileMovement(value, &cfg.Mobile.MovementMode)
	case "mobile.camerasensitivity":
		return setFloat(value, &cfg.Mobile.CameraSensitivity)
	case "mobile.zoomsensitivity":
		return setFloat(value, &cfg.Mobile.ZoomSensitivity)
	case "mobile.invertcameray":
		return setBool(value, &cfg.Mobile.InvertCameraY)
	case "mobile.longpressms":
		return setInt(value, &cfg.Mobile.LongPressMS)
	case "mobile.showtargetnames":
		return setBool(value, &cfg.Mobile.ShowTargetNames)
	case "mobile.showminimap":
		return setBool(value, &cfg.MobileDisplay.ShowMinimap)
	case "mobile.presentation":
		mode := input.MobilePresentationMode(strings.ToLower(strings.TrimSpace(value)))
		if !mode.Valid() {
			return fmt.Errorf("mobile.presentation must be %q or %q, got %q", input.MobilePresentationMobileUI, input.MobilePresentationDesktop, value)
		}
		cfg.MobileDisplay.Presentation = mode
		return nil
	case ".script", "script.path":
		cfg.Script.Path = value
	case "log.level":
		cfg.Log.Level = strings.ToLower(value)
	case "log.file":
		cfg.Log.File = value
	default:
		return fmt.Errorf("unknown key %q in section %q", key, section)
	}
	return nil
}

func validateConfig(cfg *Config) error {
	if cfg.Window.Width <= 0 {
		return fmt.Errorf("window width must be positive")
	}
	if cfg.Window.Height <= 0 {
		return fmt.Errorf("window height must be positive")
	}
	if cfg.Packet.ClientDate <= 0 {
		return fmt.Errorf("packet client date must be positive")
	}
	if cfg.Login.CharSlot < -1 || cfg.Login.CharSlot > 8 {
		return fmt.Errorf("character slot must be between 0 and 8")
	}
	if cfg.MobileSession.Mode != SessionModeOnline && cfg.MobileSession.Mode != SessionModeOffline {
		return fmt.Errorf("mobile session mode must be online or offline")
	}
	cfg.MobileSession.Server = cfg.MobileSession.Server.Normalized()
	if cfg.Audio.BGMVolume < 0 || cfg.Audio.BGMVolume > 1 {
		return fmt.Errorf("bgm volume must be between 0 and 1")
	}
	if cfg.Audio.SFXVolume < 0 || cfg.Audio.SFXVolume > 1 {
		return fmt.Errorf("sfx volume must be between 0 and 1")
	}
	cfg.UI = cfg.UI.Normalized()
	if cfg.Render.BenchSeconds < 0 || cfg.Render.BenchWarmupSeconds < 0 {
		return fmt.Errorf("benchmark durations must be non-negative")
	}
	if err := cfg.Mobile.Validate(); err != nil {
		return err
	}
	switch cfg.Log.Level {
	case "debug", "info", "warn", "warning", "error", "fatal":
	default:
		return fmt.Errorf("invalid log level %q", cfg.Log.Level)
	}
	return nil
}

func upsertINIValues(src string, values map[string]map[string]string) string {
	sectionOrder := []string{"window", "render", "ui", "audio", "gameplay", "mobile"}
	seenSections := make(map[string]bool)
	written := make(map[string]map[string]bool)
	for section := range values {
		written[section] = make(map[string]bool)
	}
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = strings.ReplaceAll(src, "\r", "\n")
	lines := strings.Split(src, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	out := make([]string, 0, len(lines)+12)
	currentSection := ""
	flushMissing := func(section string) {
		sectionValues, ok := values[section]
		if !ok {
			return
		}
		for _, key := range sortedINIKeys(sectionValues) {
			if written[section][normalizeKey(key)] {
				continue
			}
			out = append(out, fmt.Sprintf("%s = %s", key, sectionValues[key]))
			written[section][normalizeKey(key)] = true
		}
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			flushMissing(currentSection)
			currentSection = normalizeKey(strings.TrimSpace(trimmed[1 : len(trimmed)-1]))
			seenSections[currentSection] = true
			out = append(out, line)
			continue
		}
		if sectionValues, ok := values[currentSection]; ok && trimmed != "" && !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, ";") {
			if key, _, hasKey := strings.Cut(trimmed, "="); hasKey {
				normalizedKey := normalizeKey(key)
				replaced := false
				for saveKey, saveValue := range sectionValues {
					if normalizeKey(saveKey) != normalizedKey {
						continue
					}
					out = append(out, fmt.Sprintf("%s = %s", strings.TrimSpace(key), saveValue))
					written[currentSection][normalizedKey] = true
					replaced = true
					break
				}
				if replaced {
					continue
				}
			}
		}
		out = append(out, line)
	}
	flushMissing(currentSection)
	for _, section := range sectionOrder {
		if seenSections[section] || values[section] == nil {
			continue
		}
		if len(out) > 0 && strings.TrimSpace(out[len(out)-1]) != "" {
			out = append(out, "")
		}
		out = append(out, "["+section+"]")
		flushMissing(section)
	}
	return strings.Join(out, "\n") + "\n"
}

func sortedINIKeys(values map[string]string) []string {
	preferred := []string{"fullscreen", "vsync", "fps", "scale", "bgm", "bgm_volume", "sfx_volume", "no_shift", "no_ctrl", "less_effects", "snap", "itemsnap", "movement", "camera_sensitivity", "zoom_sensitivity", "invert_camera_y", "long_press_ms", "show_target_names", "show_minimap", "presentation"}
	keys := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, key := range preferred {
		if _, ok := values[key]; ok {
			keys = append(keys, key)
			seen[key] = true
		}
	}
	for key := range values {
		if !seen[key] {
			keys = append(keys, key)
		}
	}
	return keys
}

func formatINIValueBool(value bool) string {
	return strconv.FormatBool(value)
}

func formatINIValueFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', 2, 64)
}

func formatINIMobileMovement(value input.MobileMovementMode) string {
	if value == input.MovementTapToMove {
		return "tap"
	}
	return "hold"
}

func cleanINIValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
			return value[1 : len(value)-1]
		}
	}
	return value
}

func normalizeKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "")
	value = strings.ReplaceAll(value, "_", "")
	return value
}

func setBool(raw string, dst *bool) error {
	value, err := strconv.ParseBool(raw)
	if err == nil {
		*dst = value
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "yes", "on", "enabled":
		*dst = true
		return nil
	case "no", "off", "disabled":
		*dst = false
		return nil
	default:
		return fmt.Errorf("invalid bool %q", raw)
	}
}

func setInt(raw string, dst *int) error {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("invalid int %q", raw)
	}
	*dst = value
	return nil
}

func setFloat(raw string, dst *float64) error {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return fmt.Errorf("invalid float %q", raw)
	}
	*dst = value
	return nil
}

func setMobileMovement(raw string, dst *input.MobileMovementMode) error {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "hold", "hold_to_move", "holdtomove":
		*dst = input.MovementHoldToMove
	case "tap", "tap_to_move", "taptomove":
		*dst = input.MovementTapToMove
	default:
		return fmt.Errorf("invalid mobile movement mode %q", raw)
	}
	return nil
}

func resolveDataDir(value string) string {
	if value != "" {
		if abs, err := filepath.Abs(value); err == nil {
			return abs
		}
		return value
	}

	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}
