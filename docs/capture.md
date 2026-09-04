# Capture support

Goro captures the displayed framebuffer after the production draw pass. GPU
readback uses three staging buffers and never waits on the render thread; a
full staging or encoder queue drops the frame and logs the cumulative count.

## Screenshots

The console commands are:

```text
/screenshot                 # timestamped PNG
/screenshot webp            # WebP lossy, quality 90
/screenshot webp lossless   # lossless WebP
```

Screenshots are written below the configured Goro data directory in
`screenshots/`.

## Video

Windows desktop uses Media Foundation's Sink Writer for video-only H.264/MP4:

```text
/record start               # 30 FPS MP4
/record start 60            # 60 FPS MP4
/record stop                # flush and finalize
```

WebM/VP9 is an optional FFmpeg fallback:

```ini
[capture]
ffmpeg_path = C:\Tools\ffmpeg\bin\ffmpeg.exe
```

Then use `/record start 60 webm`. FFmpeg is not bundled. Recording stops and
finalizes automatically when the framebuffer is resized; start a new recording
after the resize has settled. Audio, animated WebP, AV1, HEVC, and recording
on non-Windows platforms are not part of this milestone.
