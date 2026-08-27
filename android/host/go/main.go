//go:build android && cgo

package main

/*
#include <stdint.h>
#include <android/log.h>
#include <stdlib.h>

static void goro_android_log(const char *message) {
	__android_log_print(ANDROID_LOG_INFO, "GoroAndroidGo", "%s", message);
}
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
	_ "github.com/gogpu/wgpu/hal/allbackends"
	"github.com/kivutar/goro/app"
	"github.com/kivutar/goro/config"
	"github.com/kivutar/goro/input"
	"github.com/kivutar/goro/render"
	"github.com/kivutar/goro/res"
)

// main is required by the Go linker for a c-shared package main. Android
// enters through the exported JNI bridge functions below.
func main() {}

type commandKind uint8

const (
	commandSurfaceCreated commandKind = iota
	commandSurfaceChanged
	commandSurfaceInsetsChanged
	commandSurfaceDestroyed
	commandTouch
	commandPause
	commandResume
	commandShutdown
	commandMountAssetOverlay
	commandBeginAssetRelease
	commandTextInput
	commandBack
)

type command struct {
	kind                                     commandKind
	window                                   uintptr
	width, height                            int
	safeLeft, safeTop, safeRight, safeBottom int
	touchID                                  input.TouchID
	x, y                                     int
	pressed                                  bool
	action                                   int
	text                                     string
	textInputMode                            uint32
	overlay                                  assetOverlayRequest
	release                                  string
	overlayCount                             int
	done                                     chan error
}

type assetOverlayRequest struct {
	name     string
	format   res.DeliveryFormat
	path     string
	priority int
}

type host struct {
	once     sync.Once
	commands chan command
}

type androidRuntimeMetrics struct {
	Format                     string   `json:"format"`
	Version                    int      `json:"version"`
	Map                        string   `json:"map"`
	MapLoadMS                  float64  `json:"map_load_ms"`
	TimeToFirstMapFrameMS      float64  `json:"time_to_first_map_frame_ms"`
	SteadyFPS                  float64  `json:"steady_fps"`
	AverageCPUFrameMS          float64  `json:"average_cpu_frame_ms"`
	PeakProcessRSSBytes        int64    `json:"peak_process_rss_bytes"`
	SteadyProcessRSSBytes      int64    `json:"steady_process_rss_bytes"`
	TerrainBuildMS             float64  `json:"terrain_build_ms"`
	TerrainChunkBuilds         int      `json:"terrain_chunk_builds"`
	TerrainTextureFallbacks    int      `json:"terrain_texture_fallbacks"`
	RSMTextureFallbacks        int      `json:"rsm_texture_fallbacks"`
	RSMEmptyTextureFallbacks   int      `json:"rsm_empty_texture_fallbacks"`
	RSMTextureFallbackExamples []string `json:"rsm_texture_fallback_examples,omitempty"`
	TextureDecodeMS            float64  `json:"texture_decode_ms"`
	TextureDecodeCount         int      `json:"texture_decode_count"`
	TextureEncodedBytes        int64    `json:"texture_encoded_bytes"`
	TextureDecodedRGBABytes    int64    `json:"texture_decoded_rgba_bytes"`
	TextureUploadMS            float64  `json:"texture_upload_ms"`
	TextureUploadCount         int      `json:"texture_upload_count"`
	TextureUploadedBytes       int64    `json:"texture_uploaded_bytes"`
	EstimatedTextureGPUBytes   int64    `json:"estimated_texture_gpu_bytes"`
	LastFrameAt                string   `json:"last_frame_at"`
}

type mobileAssetManifest struct {
	StartMap       string `json:"start_map"`
	DefaultProfile string `json:"default_profile"`
}

func resourceStartMap(root string) string {
	data, err := os.ReadFile(filepath.Join(root, "mobile-assets.json"))
	if err != nil {
		return "prontera"
	}
	var manifest mobileAssetManifest
	if err := json.Unmarshal(data, &manifest); err != nil || strings.TrimSpace(manifest.StartMap) == "" {
		return "prontera"
	}
	return strings.ToLower(strings.TrimSpace(manifest.StartMap))
}

var androidHost = &host{}
var presentedFrames uint64
var androidTextInputActive uint32
var resourceRootMu sync.RWMutex
var resourceRoot = "/sdcard/Android/data/com.kivutar.goro.host/files/goro-data"

//export GoroAndroidSetResourceRoot
func GoroAndroidSetResourceRoot(path *C.char) {
	if path == nil {
		return
	}
	value := C.GoString(path)
	if value == "" {
		return
	}
	resourceRootMu.Lock()
	resourceRoot = value
	resourceRootMu.Unlock()
	androidLog(fmt.Sprintf("resource-root=%s", value))
}

//export GoroAndroidMountAssetOverlay
func GoroAndroidMountAssetOverlay(name, format, path *C.char, priority C.int) {
	if name == nil || format == nil || path == nil {
		return
	}
	parsed, err := res.ParseDeliveryFormat(C.GoString(format))
	if err != nil {
		androidLog(fmt.Sprintf("asset-overlay invalid-format error=%v", err))
		return
	}
	androidHost.submit(command{kind: commandMountAssetOverlay, overlay: assetOverlayRequest{name: C.GoString(name), format: parsed, path: C.GoString(path), priority: int(priority)}})
}

//export GoroAndroidBeginAssetRelease
func GoroAndroidBeginAssetRelease(release *C.char, overlayCount C.int) {
	if release == nil || overlayCount < 0 || overlayCount > 10000 {
		return
	}
	androidHost.submit(command{kind: commandBeginAssetRelease, release: C.GoString(release), overlayCount: int(overlayCount)})
}

func currentResourceRoot() string {
	resourceRootMu.RLock()
	defer resourceRootMu.RUnlock()
	return resourceRoot
}

func logResourceProbe(root, mapName string) {
	androidLog(fmt.Sprintf("stage=resource-root path=%s", root))
	probe, err := probeGoroResources(root, mapName)
	if err != nil {
		androidLog(fmt.Sprintf("stage=resource-open map=%s error=%v", mapName, err))
		return
	}
	androidLog(fmt.Sprintf("stage=resource-open pak=%d grf=%d root=%s", probe.Packs, probe.Archives, root))
	androidLog(fmt.Sprintf("stage=gnd-load map=%s source=%s size=%dx%d textures=%d", mapName, probe.Source, probe.Width, probe.Height, probe.Textures))
	androidLog(fmt.Sprintf("stage=texture-resolve resolved=%d missing=%d", probe.TexturesResolved, probe.TexturesMissing))
	diagnostics := probe.Diagnostics
	androidLog(fmt.Sprintf("stage=gnd-diagnostics cells=%d top=%d empty-top=%d front=%d right=%d surfaces=%d used-surfaces=%d invalid-top=%d invalid-front=%d invalid-right=%d invalid-texture=%d invalid-lightmap=%d lightmap-pixels=%d lightmap-white=%d lightmap-zero=%d lightmap-alpha-zero=%d uv-outside=%d zero-alpha-surfaces=%d", diagnostics.CellCount, diagnostics.TopCells, diagnostics.EmptyTopCells, diagnostics.FrontCells, diagnostics.RightCells, diagnostics.SurfaceCount, diagnostics.UsedSurfaceCount, diagnostics.InvalidTopReferences, diagnostics.InvalidFrontReferences, diagnostics.InvalidRightReferences, diagnostics.InvalidTextureReferences, diagnostics.InvalidLightmapReferences, diagnostics.LightmapPixels, diagnostics.LightmapWhiteColorPixels, diagnostics.LightmapZeroColorPixels, diagnostics.LightmapZeroAlphaPixels, diagnostics.UVOutsideUnit, diagnostics.ZeroAlphaSurfaceColors))
	for _, texture := range diagnostics.Textures {
		androidLog(fmt.Sprintf("stage=gnd-texture id=%d name=%q top-uses=%d wall-uses=%d resolved=%t encoded-bytes=%d decoded-rgba-bytes=%d size=%dx%d transparent-pixels=%d error=%q", texture.ID, texture.Name, texture.UsedByTopCells, texture.UsedByWallCells, texture.Resolved, texture.EncodedBytes, texture.DecodedRGBABytes, texture.Width, texture.Height, texture.TransparentPixels, texture.DecodeError))
	}
	androidLog("stage=map-upload status=deferred-until-render-pass")
}

func processRSSBytes() int64 {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "VmRSS:" {
			continue
		}
		kilobytes, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return 0
		}
		return kilobytes * 1024
	}
	return 0
}

func writeAndroidRuntimeMetrics(path string, metrics androidRuntimeMetrics) {
	if path == "" {
		return
	}
	data, err := json.MarshalIndent(metrics, "", "  ")
	if err != nil {
		return
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		androidLog(fmt.Sprintf("stage=mobile-metrics write-error=%v", err))
	}
}

func writeAndroidActiveRelease(release string, overlays []res.AssetOverlay) error {
	if release == "" {
		return fmt.Errorf("asset release name is empty")
	}
	for _, character := range release {
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') &&
			(character < '0' || character > '9') && character != '_' && character != '-' {
			return fmt.Errorf("unsafe asset release name %q", release)
		}
	}

	// Downloaded overlays live below <files>/goro-mobile/releases/<release>/overlays.
	// Derive the state directory from that path when possible, so the pointer is
	// kept alongside the files that were just validated. A release with no
	// optional overlays uses the sibling of the installed embedded root.
	stateRoot := ""
	for _, overlay := range overlays {
		path, err := filepath.Abs(overlay.Path)
		if err != nil {
			continue
		}
		for current := filepath.Dir(path); current != filepath.Dir(current); current = filepath.Dir(current) {
			if filepath.Base(current) == "goro-mobile" {
				stateRoot = current
				break
			}
		}
		if stateRoot != "" {
			break
		}
	}
	if stateRoot == "" {
		installedRoot := currentResourceRoot()
		stateRoot = filepath.Join(filepath.Dir(installedRoot), "goro-mobile")
	}
	if err := os.MkdirAll(stateRoot, 0o700); err != nil {
		return fmt.Errorf("create asset release state directory: %w", err)
	}
	data, err := json.Marshal(struct {
		Release string `json:"release"`
		State   string `json:"state"`
	}{Release: release, State: "state.json"})
	if err != nil {
		return fmt.Errorf("encode active asset release: %w", err)
	}
	temporary := filepath.Join(stateRoot, "active-release.json.tmp")
	if err := os.WriteFile(temporary, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write active asset release: %w", err)
	}
	if err := os.Rename(temporary, filepath.Join(stateRoot, "active-release.json")); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("activate asset release: %w", err)
	}
	return nil
}

func androidLog(message string) {
	cMessage := C.CString(message)
	defer C.free(unsafe.Pointer(cMessage))
	C.goro_android_log(cMessage)
}

func (h *host) start() {
	h.once.Do(func() {
		h.commands = make(chan command)
		go h.renderLoop()
	})
}

func (h *host) submit(cmd command) {
	h.start()
	if cmd.done == nil {
		cmd.done = make(chan error, 1)
	}
	h.commands <- cmd
	if err := <-cmd.done; err != nil {
		log.Printf("command %d failed: %v", cmd.kind, err)
		androidLog(fmt.Sprintf("command=%d failed: %v", cmd.kind, err))
	}
}

//export GoroAndroidSurfaceCreated
func GoroAndroidSurfaceCreated(window C.uintptr_t, width C.int, height C.int) {
	androidHost.submit(command{kind: commandSurfaceCreated, window: uintptr(window), width: int(width), height: int(height)})
}

//export GoroAndroidSurfaceChanged
func GoroAndroidSurfaceChanged(width C.int, height C.int) {
	androidHost.submit(command{kind: commandSurfaceChanged, width: int(width), height: int(height)})
}

//export GoroAndroidSurfaceInsetsChanged
func GoroAndroidSurfaceInsetsChanged(left C.int, top C.int, right C.int, bottom C.int) {
	androidHost.submit(command{kind: commandSurfaceInsetsChanged, safeLeft: int(left), safeTop: int(top), safeRight: int(right), safeBottom: int(bottom)})
}

//export GoroAndroidSurfaceDestroyed
func GoroAndroidSurfaceDestroyed() { androidHost.submit(command{kind: commandSurfaceDestroyed}) }

//export GoroAndroidTouch
func GoroAndroidTouch(action C.int, pointerID C.int, x C.int, y C.int, pressed C.int) {
	androidHost.submit(command{kind: commandTouch, action: int(action), touchID: input.TouchID(pointerID), x: int(x), y: int(y), pressed: pressed != 0})
	log.Printf("touch action=%d id=%d x=%d y=%d pressed=%t", int(action), int(pointerID), int(x), int(y), pressed != 0)
}

//export GoroAndroidBack
func GoroAndroidBack() {
	androidHost.submit(command{kind: commandBack})
}

//export GoroAndroidTextInputMode
func GoroAndroidTextInputMode() C.int {
	return C.int(atomic.LoadUint32(&androidTextInputActive))
}

//export GoroAndroidTextInput
func GoroAndroidTextInput(text *C.char) {
	if text == nil {
		return
	}
	androidHost.submit(command{kind: commandTextInput, text: C.GoString(text), textInputMode: atomic.LoadUint32(&androidTextInputActive)})
}

//export GoroAndroidPause
func GoroAndroidPause() { androidHost.submit(command{kind: commandPause}) }

//export GoroAndroidResume
func GoroAndroidResume() { androidHost.submit(command{kind: commandResume}) }

//export GoroAndroidShutdown
func GoroAndroidShutdown() { androidHost.submit(command{kind: commandShutdown}) }

func (h *host) renderLoop() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	var instance *wgpu.Instance
	var adapter *wgpu.Adapter
	var device *wgpu.Device
	var surface *wgpu.Surface
	var state = input.NewState()
	var goroRenderer *render.GPURenderer
	var offlineGame *app.Game
	var mobileConfig config.Config
	var mobile *mobilePresentation
	var mobileInput *input.MobileInputAdapter
	var width, height int
	var surfaceStartedAt, mapReadyAt, firstMapFrameAt time.Time
	var renderedFrames int
	var cpuFrameTotal time.Duration
	var peakRSS int64
	var runtimeMetricsPath string
	var pendingOverlays []assetOverlayRequest
	var pendingRelease *struct {
		name  string
		count int
	}
	var pendingReleaseOverlays []assetOverlayRequest
	mobileSettings := input.DefaultMobileSettings()
	applyPendingRelease := func() error {
		if offlineGame == nil || pendingRelease == nil || len(pendingReleaseOverlays) != pendingRelease.count {
			return nil
		}
		overlays := make([]res.AssetOverlay, 0, len(pendingReleaseOverlays))
		for _, overlay := range pendingReleaseOverlays {
			overlays = append(overlays, res.AssetOverlay{Name: overlay.name, Format: overlay.format, Path: overlay.path, Priority: overlay.priority, Android: true})
		}
		if err := offlineGame.ReplaceAssetOverlays(overlays); err != nil {
			return err
		}
		if err := writeAndroidActiveRelease(pendingRelease.name, overlays); err != nil {
			return err
		}
		androidLog(fmt.Sprintf("asset-release activated name=%s overlays=%d", pendingRelease.name, len(overlays)))
		pendingRelease = nil
		pendingReleaseOverlays = nil
		return nil
	}
	startMap := resourceStartMap(currentResourceRoot())
	ticker := time.NewTicker(16 * time.Millisecond)
	defer ticker.Stop()

	initDevice := func(window uintptr) error {
		surfaceStartedAt = time.Now()
		mapReadyAt = time.Time{}
		firstMapFrameAt = time.Time{}
		renderedFrames = 0
		cpuFrameTotal = 0
		peakRSS = 0
		androidLog("stage=instance begin backend=vulkan")
		log.Printf("stage=instance backend=vulkan")
		var err error
		instance, err = wgpu.CreateInstance(&wgpu.InstanceDescriptor{Backends: gputypes.BackendsVulkan})
		if err != nil {
			androidLog(fmt.Sprintf("stage=instance error=%v", err))
			return fmt.Errorf("instance: %w", err)
		}
		androidLog("stage=instance ready")
		surface, err = instance.CreateSurfaceUnsafe(wgpu.SurfaceTargetFromAndroidNativeWindow(window))
		if err != nil {
			androidLog(fmt.Sprintf("stage=surface error=%v", err))
			return fmt.Errorf("surface: %w", err)
		}
		androidLog("stage=surface ready")
		adapter, err = instance.RequestAdapter(&wgpu.RequestAdapterOptions{CompatibleSurface: surface, PowerPreference: wgpu.PowerPreferenceHighPerformance})
		if err != nil {
			androidLog(fmt.Sprintf("stage=adapter error=%v", err))
			return fmt.Errorf("adapter: %w", err)
		}
		if adapter == nil {
			androidLog("stage=adapter error=nil adapter")
			return fmt.Errorf("adapter: nil adapter")
		}
		info := adapter.Info()
		androidLog(fmt.Sprintf("stage=adapter name=%q backend=%s", info.Name, info.Backend))
		log.Printf("stage=adapter name=%q backend=%s", info.Name, info.Backend)
		device, err = adapter.RequestDevice(nil)
		if err != nil {
			androidLog(fmt.Sprintf("stage=device error=%v", err))
			return fmt.Errorf("device: %w", err)
		}
		log.Printf("stage=device acquired")
		androidLog("stage=device acquired")
		if err := configureSurface(surface, adapter, device, width, height); err != nil {
			return err
		}
		androidLog("stage=goro-renderer begin")
		context, err := render.NewRawGPUContext(device, device.Queue(), configuredFormat(surface, adapter))
		if err != nil {
			androidLog(fmt.Sprintf("stage=goro-renderer device-bound error=%v", err))
			return err
		}
		goroRenderer, err = render.NewGPURenderer(context, config.RenderConfig{Stats: false})
		if err != nil {
			androidLog(fmt.Sprintf("stage=goro-renderer pipelines-ready error=%v", err))
			return fmt.Errorf("goro renderer: %w", err)
		}
		androidLog("stage=goro-renderer device-bound")
		androidLog("stage=goro-renderer pipelines-ready")
		if offlineGame == nil {
			// Android does not provide the Unix config environment variables to
			// the embedded Go library. Keep the shared config loader and point
			// its normal store at the app-private active resource root.
			if os.Getenv("XDG_CONFIG_HOME") == "" {
				_ = os.Setenv("XDG_CONFIG_HOME", currentResourceRoot())
			}
			if configPath, pathErr := config.UserConfigPath(); pathErr != nil {
				androidLog(fmt.Sprintf("stage=mobile-config path-error=%v home=%s", pathErr, os.Getenv("HOME")))
			} else {
				androidLog(fmt.Sprintf("stage=mobile-config path=%s home=%s", configPath, os.Getenv("HOME")))
			}
			cfg, configErr := config.LoadConfig(nil)
			if configErr != nil {
				androidLog(fmt.Sprintf("stage=mobile-config load-error=%v", configErr))
				return fmt.Errorf("mobile config: %w", configErr)
			} else {
				mobileSettings = input.MobileSettings{
					Controls: cfg.Mobile,
					Audio: input.MobileAudioSettings{
						BGMEnabled: cfg.Audio.BGM,
						BGMVolume:  cfg.Audio.BGMVolume,
						SFXVolume:  cfg.Audio.SFXVolume,
					},
					Display: input.MobileDisplaySettings{ShowMinimap: cfg.MobileDisplay.ShowMinimap},
					Gameplay: input.MobileGameplaySettings{
						NoShift: cfg.Gameplay.NoShift, NoCtrl: cfg.Gameplay.NoCtrl,
						LessEffects: cfg.Gameplay.LessEffects, SnapTargets: cfg.Gameplay.SnapTargets,
						SnapItems: cfg.Gameplay.SnapItems,
					},
				}.Normalized()
			}
			cfg.DataDir = currentResourceRoot()
			cfg.Window = config.WindowConfig{Width: width, Height: height, Title: "Goro"}
			cfg.Render.GraphicsAPI = "vulkan"
			cfg.Render.VSync = true
			cfg.Render.NoUI = true
			cfg.Render.Stats = false
			cfg.Mobile = mobileSettings.Controls
			cfg.MobileDisplay = mobileSettings.Display
			cfg.Audio.BGM = mobileSettings.Audio.BGMEnabled
			cfg.Audio.BGMVolume = mobileSettings.Audio.BGMVolume
			cfg.Audio.SFXVolume = mobileSettings.Audio.SFXVolume
			cfg.Gameplay.NoShift = mobileSettings.Gameplay.NoShift
			cfg.Gameplay.NoCtrl = mobileSettings.Gameplay.NoCtrl
			cfg.Gameplay.LessEffects = mobileSettings.Gameplay.LessEffects
			cfg.Gameplay.SnapTargets = mobileSettings.Gameplay.SnapTargets
			cfg.Gameplay.SnapItems = mobileSettings.Gameplay.SnapItems
			androidLog(fmt.Sprintf("stage=mobile-config loaded mode=%s server=%s:%d auth=%d profile=%d autologin=%t username=%q", cfg.MobileSession.Mode, cfg.MobileSession.Server.Host, cfg.MobileSession.Server.CharPort, cfg.MobileSession.Server.AuthPort, cfg.MobileSession.Server.Profile, cfg.Login.AutoLogin, cfg.Login.Username))
			mobileConfig = cfg
			if cfg.MobileSession.Mode == config.SessionModeOnline {
				offlineGame, err = app.New(cfg)
			} else {
				offlineGame, err = app.NewOfflineAtMap(cfg, startMap)
			}
			if err != nil {
				androidLog(fmt.Sprintf("stage=mobile-game error=%v", err))
				return fmt.Errorf("mobile game: %w", err)
			}
			for _, overlay := range pendingOverlays {
				if err := offlineGame.MountAssetOverlay(res.AssetOverlay{Name: overlay.name, Format: overlay.format, Path: overlay.path, Priority: overlay.priority, Android: true}); err != nil {
					androidLog(fmt.Sprintf("asset-overlay mount name=%s error=%v", overlay.name, err))
				} else {
					androidLog(fmt.Sprintf("asset-overlay mounted name=%s format=%s", overlay.name, overlay.format))
				}
			}
			pendingOverlays = nil
			if err := applyPendingRelease(); err != nil {
				androidLog(fmt.Sprintf("asset-release activation error=%v", err))
			}
			if offlineGame.Offline() != nil {
				savePath := filepath.Join(currentResourceRoot(), "offline-save.json")
				if err := offlineGame.LoadOfflineState(savePath); err != nil && !os.IsNotExist(err) {
					androidLog(fmt.Sprintf("stage=offline-save load-error=%v", err))
				}
			}
		} else if offlineGame.Offline() != nil {
			offlineGame.Offline().Resume()
			offlineGame.Resize(width, height)
		}
		if mobile == nil {
			mobile = newMobilePresentation(offlineGame, width, height)
			mobile.SetSettings(mobileSettings)
			mobileInput = input.NewMobileInputAdapterWithControls(mobileSettings.Controls, mobileWorldPicker{presentation: mobile}, mobile, mobileCommandSink{game: offlineGame, presentation: mobile})
			mobile.SetModeChanged(func(online bool) bool {
				if offlineGame == nil || (online == offlineGame.Online()) {
					return true
				}
				if offlineGame.Offline() != nil {
					savePath := filepath.Join(currentResourceRoot(), "offline-save.json")
					if saveErr := offlineGame.SaveOfflineState(savePath); saveErr != nil {
						androidLog(fmt.Sprintf("stage=mobile-mode save-error=%v", saveErr))
					}
				} else if offlineGame.Online() {
					offlineGame.Disconnect()
				}
				cfg := mobileConfig
				if online {
					cfg.MobileSession.Mode = config.SessionModeOnline
					// The Online button is an explicit mobile login request. Mobile
					// has no desktop credential form, so use the configured
					// credentials immediately after the mode handoff.
					cfg.Login.AutoLogin = true
				} else {
					cfg.MobileSession.Mode = config.SessionModeOffline
				}
				var next *app.Game
				var modeErr error
				if online {
					next, modeErr = app.New(cfg)
				} else {
					next, modeErr = app.NewOfflineAtMap(cfg, startMap)
				}
				if modeErr != nil {
					androidLog(fmt.Sprintf("stage=mobile-mode target=%s error=%v", cfg.MobileSession.Mode, modeErr))
					return false
				}
				if next.Offline() != nil {
					savePath := filepath.Join(currentResourceRoot(), "offline-save.json")
					if loadErr := next.LoadOfflineState(savePath); loadErr != nil && !os.IsNotExist(loadErr) {
						androidLog(fmt.Sprintf("stage=mobile-mode load-error=%v", loadErr))
					}
				}
				mobileConfig = cfg
				offlineGame = next
				mobile.SetGame(next)
				androidLog(fmt.Sprintf("stage=mobile-mode active=%s server=%s:%d", cfg.MobileSession.Mode, cfg.MobileSession.Server.Host, cfg.MobileSession.Server.ZonePort))
				return true
			})
			mobile.SetSettingsChanged(func(settings input.MobileSettings) bool {
				mobileSettings = settings
				if mobileInput != nil {
					mobileInput.SetControls(settings.Controls)
				}
				path, saveErr := config.SaveMobileSettings(settings)
				if saveErr != nil {
					androidLog(fmt.Sprintf("stage=mobile-settings save-error=%v", saveErr))
					return true
				}
				androidLog(fmt.Sprintf("stage=mobile-settings applied path=%s movement=%s camera=%.2f zoom=%.2f invert_y=%t long_press_ms=%d names=%t bgm=%t bgm_volume=%.2f sfx_volume=%.2f minimap=%t", path, settings.Controls.MovementMode.String(), settings.Controls.CameraSensitivity, settings.Controls.ZoomSensitivity, settings.Controls.InvertCameraY, settings.Controls.LongPressMS, settings.Controls.ShowTargetNames, settings.Audio.BGMEnabled, settings.Audio.BGMVolume, settings.Audio.SFXVolume, settings.Display.ShowMinimap))
				return true
			})
		} else {
			mobile.Resize(width, height)
		}
		monsterCount := 0
		monsterNames := make([]string, 0, 4)
		if offline := offlineGame.Offline(); offline != nil {
			for _, monster := range offline.Monsters() {
				monsterCount++
				if len(monsterNames) < cap(monsterNames) {
					monsterNames = append(monsterNames, monster.Name)
				}
			}
		}
		androidLog(fmt.Sprintf("stage=mobile-game ready mode=%s map=%s monsters=%d monster-names=%q audio-enabled=%t", mobileConfig.MobileSession.Mode, startMap, monsterCount, monsterNames, offlineGame.AudioEnabled()))
		mapReadyAt = time.Now()
		runtimeMetricsPath = filepath.Join(currentResourceRoot(), "mobile-runtime-metrics.json")
		androidLog(fmt.Sprintf("stage=mobile-metrics map-load-ms=%.2f", mapReadyAt.Sub(surfaceStartedAt).Seconds()*1000))
		return nil
	}

	releaseSurface := func() {
		if surface != nil {
			surface.Release()
			surface = nil
			log.Printf("stage=surface released")
		}
	}

	for {
		select {
		case cmd := <-h.commands:
			var err error
			switch cmd.kind {
			case commandSurfaceCreated:
				width, height = cmd.width, cmd.height
				logResourceProbe(currentResourceRoot(), startMap)
				releaseSurface()
				err = initDevice(cmd.window)
			case commandSurfaceChanged:
				width, height = cmd.width, cmd.height
				if surface != nil && device != nil && adapter != nil {
					err = configureSurface(surface, adapter, device, width, height)
					if offlineGame != nil {
						offlineGame.Resize(width, height)
					}
					if mobile != nil {
						mobile.Resize(width, height)
					}
				}
			case commandSurfaceInsetsChanged:
				if mobile != nil {
					mobile.SetSafeInsets(cmd.safeLeft, cmd.safeTop, cmd.safeRight, cmd.safeBottom)
				}
			case commandSurfaceDestroyed:
				if offlineGame != nil && offlineGame.Offline() != nil {
					offlineGame.Offline().Pause()
					offlineGame.PauseAudio()
				}
				releaseSurface()
			case commandTouch:
				if mobile != nil {
					if cmd.action == 0 || cmd.action == 5 {
						mobile.BeginTouch(cmd.touchID, cmd.x, cmd.y)
					}
					if cmd.action == 2 {
						mobile.Move(cmd.touchID, cmd.x, cmd.y)
					}
					if !cmd.pressed {
						if cmd.action == 3 {
							mobile.CancelTouch()
						} else {
							mobile.Release(cmd.touchID, cmd.x, cmd.y)
						}
					}
				}
				state.SetTouch(cmd.touchID, cmd.x, cmd.y, cmd.pressed)
				state.EndFrame()
				if mobileInput != nil {
					mobileInput.Update(input.TouchFrame{Points: state.TouchPoints, At: time.Now()})
				}
				log.Printf("touch-state active=%d pinch=%.2f", len(state.TouchPoints), state.PinchDelta)
				androidLog(fmt.Sprintf("touch-state active=%d pinch=%.2f", len(state.TouchPoints), state.PinchDelta))
			case commandTextInput:
				if mobile != nil {
					mobile.SetTextInput(cmd.textInputMode, cmd.text)
				}
			case commandBack:
				if mobile != nil {
					mobile.Back()
				}
			case commandPause:
				if offlineGame != nil && offlineGame.Offline() != nil {
					offlineGame.Offline().Pause()
					if err := offlineGame.SaveOfflineState(filepath.Join(currentResourceRoot(), "offline-save.json")); err != nil {
						androidLog(fmt.Sprintf("stage=offline-save pause-error=%v", err))
					} else {
						androidLog("stage=offline-save reason=pause")
					}
				}
			case commandResume:
				if offlineGame != nil && offlineGame.Offline() != nil {
					offlineGame.Offline().Resume()
					offlineGame.ResumeAudio()
				}
			case commandMountAssetOverlay:
				if pendingRelease != nil {
					if len(pendingReleaseOverlays) >= pendingRelease.count {
						err = fmt.Errorf("asset release %s received too many overlays", pendingRelease.name)
						break
					}
					pendingReleaseOverlays = append(pendingReleaseOverlays, cmd.overlay)
					err = applyPendingRelease()
				} else if offlineGame == nil {
					pendingOverlays = append(pendingOverlays, cmd.overlay)
				} else {
					err = offlineGame.MountAssetOverlay(res.AssetOverlay{Name: cmd.overlay.name, Format: cmd.overlay.format, Path: cmd.overlay.path, Priority: cmd.overlay.priority, Android: true})
					if err == nil {
						androidLog(fmt.Sprintf("asset-overlay mounted name=%s format=%s", cmd.overlay.name, cmd.overlay.format))
					}
				}
			case commandBeginAssetRelease:
				pendingRelease = &struct {
					name  string
					count int
				}{name: cmd.release, count: cmd.overlayCount}
				pendingReleaseOverlays = nil
				err = applyPendingRelease()
			case commandShutdown:
				if offlineGame != nil && offlineGame.Offline() != nil {
					if err := offlineGame.SaveOfflineState(filepath.Join(currentResourceRoot(), "offline-save.json")); err != nil {
						androidLog(fmt.Sprintf("stage=offline-save write-error=%v", err))
					}
				}
				releaseSurface()
				if device != nil {
					device.Release()
					device = nil
				}
				if adapter != nil {
					adapter.Release()
					adapter = nil
				}
				if instance != nil {
					instance.Release()
					instance = nil
				}
				cmd.done <- nil
				return
			}
			cmd.done <- err
		case <-ticker.C:
			if surface != nil && device != nil && width > 0 && height > 0 && (offlineGame == nil || offlineGame.Offline() == nil || !offlineGame.Offline().Paused) {
				if mobileInput != nil && len(state.TouchPoints) > 0 {
					// MotionEvent does not generate a new callback while a finger
					// is stationary. Feed the recognizer from the render tick so
					// long-press inspection still fires at its configured time.
					mobileInput.Update(input.TouchFrame{Points: state.TouchPoints, At: time.Now()})
				}
				if offlineGame != nil {
					frameStarted := time.Now()
					if mobile == nil || mobile.WorldActive() {
						if err := offlineGame.Update(); err != nil {
							androidLog(fmt.Sprintf("stage=offline-update error=%v", err))
						}
					}
					if offlineGame.Online() && (renderedFrames < 2 || renderedFrames%60 == 0) {
						androidLog(fmt.Sprintf("stage=online-tick login=%s network=%s playing=%t", offlineGame.LoginStatus(), offlineGame.NetworkStatus(), offlineGame.SessionPlaying()))
					}
					offlineGame.Resize(width, height)
					frame := render.NewFrame(width, height)
					offlineGame.Draw(frame)
					if mobile != nil && len(state.TouchPoints) == 1 && mobile.WorldTouchAvailable(state.TouchPoints[0]) {
						touch := state.TouchPoints[0]
						if target, ok := offlineGame.PickMobileTarget(input.WorldPosition{X: float64(touch.X), Y: float64(touch.Y)}); ok && target.Kind == input.TargetGround {
							offlineGame.DrawMobileTileCursor(target.Position, frame)
						}
					}
					offlineGame.DrawOverlay(frame)
					offlineGame.DrawUIOverlay(frame)
					if mobile != nil {
						mobile.Refresh()
						mobile.Draw(frame)
					}
					offlineGame.FrameSubmitted()
					renderFrame(surface, device, goroRenderer, frame, width, height, configuredFormat(surface, adapter))
					frameDuration := time.Since(frameStarted)
					renderedFrames++
					cpuFrameTotal += frameDuration
					rss := processRSSBytes()
					if rss > peakRSS {
						peakRSS = rss
					}
					if renderedFrames == 1 {
						firstMapFrameAt = time.Now()
						androidLog(fmt.Sprintf("stage=mobile-metrics first-map-frame-ms=%.2f", firstMapFrameAt.Sub(surfaceStartedAt).Seconds()*1000))
					}
					if renderedFrames%60 == 0 {
						worldMetrics := offlineGame.RenderMetrics()
						uploadMetrics := goroRenderer.TextureUploadMetrics()
						elapsed := time.Since(firstMapFrameAt).Seconds()
						fps := 0.0
						if elapsed > 0 {
							fps = float64(renderedFrames-1) / elapsed
						}
						firstFrameMS := 0.0
						if !firstMapFrameAt.IsZero() {
							firstFrameMS = firstMapFrameAt.Sub(surfaceStartedAt).Seconds() * 1000
						}
						metrics := androidRuntimeMetrics{Format: "goro-mobile-runtime-metrics", Version: 1, Map: startMap, MapLoadMS: mapReadyAt.Sub(surfaceStartedAt).Seconds() * 1000, TimeToFirstMapFrameMS: firstFrameMS, SteadyFPS: fps, AverageCPUFrameMS: cpuFrameTotal.Seconds() * 1000 / float64(renderedFrames), PeakProcessRSSBytes: peakRSS, SteadyProcessRSSBytes: rss, TerrainBuildMS: worldMetrics.TerrainBuildDuration.Seconds() * 1000, TerrainChunkBuilds: worldMetrics.TerrainChunkBuilds, TerrainTextureFallbacks: worldMetrics.TerrainTextureFallbacks, RSMTextureFallbacks: worldMetrics.RSMTextureFallbacks, RSMEmptyTextureFallbacks: worldMetrics.RSMEmptyTextureFallbacks, RSMTextureFallbackExamples: worldMetrics.RSMTextureFallbackExamples, TextureDecodeMS: worldMetrics.TextureDecodeDuration.Seconds() * 1000, TextureDecodeCount: worldMetrics.TextureDecodeCount, TextureEncodedBytes: worldMetrics.TextureEncodedBytes, TextureDecodedRGBABytes: worldMetrics.TextureDecodedRGBABytes, TextureUploadMS: uploadMetrics.Duration.Seconds() * 1000, TextureUploadCount: uploadMetrics.Count, TextureUploadedBytes: uploadMetrics.UploadedBytes, EstimatedTextureGPUBytes: uploadMetrics.EstimatedGPUBytes, LastFrameAt: time.Now().UTC().Format(time.RFC3339Nano)}
						writeAndroidRuntimeMetrics(runtimeMetricsPath, metrics)
						androidLog(fmt.Sprintf("stage=mobile-metrics frames=%d fps=%.2f cpu-frame-ms=%.2f rss=%d terrain-ms=%.2f terrain-fallbacks=%d rsm-fallbacks=%d rsm-empty-fallbacks=%d rsm-examples=%q texture-decode-ms=%.2f texture-upload-ms=%.2f texture-gpu-bytes=%d", renderedFrames, fps, metrics.AverageCPUFrameMS, rss, metrics.TerrainBuildMS, metrics.TerrainTextureFallbacks, metrics.RSMTextureFallbacks, metrics.RSMEmptyTextureFallbacks, metrics.RSMTextureFallbackExamples, metrics.TextureDecodeMS, metrics.TextureUploadMS, metrics.EstimatedTextureGPUBytes))
					}
				}
			}
		}
	}
}

func configuredFormat(surface *wgpu.Surface, adapter *wgpu.Adapter) gputypes.TextureFormat {
	if caps := adapter.GetSurfaceCapabilities(surface); caps != nil && len(caps.Formats) > 0 {
		return caps.Formats[0]
	}
	return gputypes.TextureFormatRGBA8Unorm
}

func configureSurface(surface *wgpu.Surface, adapter *wgpu.Adapter, device *wgpu.Device, width, height int) error {
	caps := adapter.GetSurfaceCapabilities(surface)
	format := gputypes.TextureFormatBGRA8Unorm
	alphaMode := gputypes.CompositeAlphaModeAuto
	if caps != nil {
		if len(caps.Formats) > 0 {
			format = caps.Formats[0]
		}
		if len(caps.AlphaModes) > 0 {
			alphaMode = caps.AlphaModes[0]
			for _, candidate := range caps.AlphaModes {
				if candidate == gputypes.CompositeAlphaModeOpaque {
					alphaMode = candidate
					break
				}
			}
		}
	}
	androidLog(fmt.Sprintf("stage=surface-capabilities formats=%v alpha-modes=%v selected-alpha=%s", capsFormats(caps), capsAlphaModes(caps), alphaMode))
	if err := surface.Configure(device, &wgpu.SurfaceConfiguration{Format: format, Usage: gputypes.TextureUsageRenderAttachment, Width: uint32(width), Height: uint32(height), PresentMode: gputypes.PresentModeFifo, AlphaMode: alphaMode}); err != nil {
		androidLog(fmt.Sprintf("stage=surface configure error=%v", err))
		return fmt.Errorf("configure: %w", err)
	}
	log.Printf("stage=surface configured format=%s size=%dx%d present=fifo alpha=%s", format, width, height, alphaMode)
	androidLog(fmt.Sprintf("stage=surface configured format=%s size=%dx%d present=fifo alpha=%s", format, width, height, alphaMode))
	return nil
}

func capsFormats(caps *wgpu.SurfaceCapabilities) []gputypes.TextureFormat {
	if caps == nil {
		return nil
	}
	return caps.Formats
}

func capsAlphaModes(caps *wgpu.SurfaceCapabilities) []gputypes.CompositeAlphaMode {
	if caps == nil {
		return nil
	}
	return caps.AlphaModes
}

func surfaceCapabilities(surface *wgpu.Surface, adapter *wgpu.Adapter) []gputypes.TextureFormat {
	if surface == nil || adapter == nil {
		return nil
	}
	caps := adapter.GetSurfaceCapabilities(surface)
	if caps == nil {
		return nil
	}
	return caps.Formats
}

func renderFrame(surface *wgpu.Surface, device *wgpu.Device, renderer *render.GPURenderer, frame *render.Frame, width, height int, format gputypes.TextureFormat) {
	texture, _, err := surface.GetCurrentTexture()
	if err != nil {
		log.Printf("stage=acquire error=%v", err)
		return
	}
	view, err := texture.CreateView(nil)
	if err != nil {
		surface.DiscardTexture()
		log.Printf("stage=view error=%v", err)
		return
	}
	if renderer == nil || frame == nil {
		surface.DiscardTexture()
		view.Release()
		return
	}
	androidLog("stage=render-pass begin")
	_, err = renderer.DrawTarget(render.FrameTarget{View: view, Width: width, Height: height, Format: format}, frame)
	if err != nil {
		view.Release()
		surface.DiscardTexture()
		androidLog(fmt.Sprintf("stage=render-pass error=%v", err))
		return
	}
	androidLog("stage=submit frame=1")
	err = surface.Present(texture)
	view.Release()
	if err != nil {
		log.Printf("stage=present error=%v", err)
		androidLog(fmt.Sprintf("stage=present error=%v", err))
		return
	}
	frames := atomic.AddUint64(&presentedFrames, 1)
	if frames == 1 || frames%60 == 0 {
		androidLog(fmt.Sprintf("stage=present map-frame=%d", frames))
	}
}

func renderClear(surface *wgpu.Surface, device *wgpu.Device) {
	texture, _, err := surface.GetCurrentTexture()
	if err != nil {
		log.Printf("stage=acquire error=%v", err)
		return
	}
	view, err := texture.CreateView(nil)
	if err != nil {
		surface.DiscardTexture()
		log.Printf("stage=view error=%v", err)
		return
	}
	encoder, err := device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{Label: "goro-phase0d-clear"})
	if err != nil {
		view.Release()
		surface.DiscardTexture()
		return
	}
	pass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{ColorAttachments: []wgpu.RenderPassColorAttachment{{View: view, LoadOp: gputypes.LoadOpClear, StoreOp: gputypes.StoreOpStore, ClearValue: gputypes.Color{R: 0.08, G: 0.16, B: 0.32, A: 1}}}})
	if err != nil {
		view.Release()
		surface.DiscardTexture()
		log.Printf("stage=render-pass error=%v", err)
		return
	}
	_ = pass.End()
	commands, err := encoder.Finish()
	if err == nil {
		_, err = device.Queue().Submit(commands)
	}
	if err == nil {
		err = surface.Present(texture)
	}
	view.Release()
	if err != nil {
		log.Printf("stage=present error=%v", err)
		androidLog(fmt.Sprintf("stage=present error=%v", err))
		return
	}
	frames := atomic.AddUint64(&presentedFrames, 1)
	if frames == 1 || frames%60 == 0 {
		androidLog(fmt.Sprintf("stage=present frame=%d", frames))
	}
}
