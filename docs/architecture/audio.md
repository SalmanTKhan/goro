# Audio

Package `audio` decodes and plays BGM and sound effects and manages volume. It
does not decide *which* sound to play — that policy lives in `game/sfx.go` and
the feature files.

## BGM

`audio.NewBGM(resources, enabled, bgmVolume, sfxVolume, disabled)` is the single
audio object held by `app.Game` and exposed on `client.Context`.

- `PlayMap(mapName)` resolves the map's BGM through `res` (`mp3nametable`) and
  starts it; `Play(path)` plays a specific track.
- `Enabled` / `SetEnabled`, `Volume` / `SetVolume`, `BGMVolume` /
  `SetBGMVolume` — the settings window and `config` user settings drive these.
- `disabled` (`--no-audio`) turns output off entirely, which is the intended
  mode for profiling runs.

Decoding: native PCM (`decodeNativePCM`) and MP3 through the demuxer/decoder
engines, normalized to PCM16 stereo. `infinitePCM` provides looping.
Resampling to the output rate is windowed-sinc with a linear fallback
(`resamplePCM16Stereo`, `resamplePCM16StereoLinear`, `buildResampleTaps`).

`bgm_stub.go` is the build-constrained no-op implementation for platforms
without an output device.

## SFX

`sfx.go` plays one-shot WAV effects with the SFX volume applied
(`volume.go` holds the shared volume math). Hit sounds and weapon-specific
sounds come from the `db` tables (`db/hit_sounds.go`, `db/weapon.go`); map
ambient sounds come from RSW via `res`.

## Config

`[audio] bgm`, `bgm_volume`, and the SFX volume, with CLI equivalents
`--bgm=false`, `--bgm-volume`, `--no-audio`. User changes made in the settings
window persist through `config.UserSettings`.
