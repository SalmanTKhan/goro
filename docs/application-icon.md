# Application Icon

`internal/appicon/icon.png` is the canonical 32x32 pixel-art source. It is
embedded in the Go binary and passed to GoGPU for the native window.

After replacing the source icon, regenerate the platform assets with:

```sh
go generate ./internal/appicon
```

The generator creates:

- `packaging/windows/goro.ico`, containing 16, 32, 48, 64, and 256 pixel
  variants. The release workflow compiles this into each Windows executable.
- `packaging/linux/goro.png`, a 256x256 nearest-neighbor variant for desktop
  packaging.
- `android/host/app/src/main/res/mipmap-{mdpi,hdpi,xhdpi,xxhdpi,xxxhdpi}/ic_launcher.png`,
  the Android launcher icon (48/72/96/144/192 px). The manifest references it as
  `@mipmap/ic_launcher`.

`packaging/linux/goro.desktop` is a template for distribution packages. Install
the desktop file and PNG in the appropriate XDG application and icon directories.

GoGPU v0.44.6 uses a fixed `gogpu` application ID on Wayland, so some Wayland
compositors will not associate `goro.desktop` with a directly launched window.
X11 receives the embedded icon directly.

The macOS releases remain bare binaries. They do not include app-bundle or ICNS
icon packaging.
