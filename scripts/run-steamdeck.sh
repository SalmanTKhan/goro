#!/usr/bin/env bash
set -o pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
if [[ -f "$SCRIPT_DIR/goro" || -x "$SCRIPT_DIR/goro" ]]; then
    # Deployed layout: the launcher is beside the Linux executable.
    BUILD_DIR="$SCRIPT_DIR"
else
    # Repository layout: scripts/run-steamdeck.sh.
    BUILD_DIR="$(cd -- "$SCRIPT_DIR/.." && pwd)"
fi
RUNTIME_DIR="${XDG_RUNTIME_DIR:-/run/user/$(id -u)}"

if [[ -z "${WAYLAND_DISPLAY:-}" ]]; then
    for socket in "$RUNTIME_DIR"/wayland-*; do
        if [[ -S "$socket" ]]; then
            WAYLAND_DISPLAY="${socket##*/}"
            break
        fi
    done
fi

if [[ -z "${WAYLAND_DISPLAY:-}" ]]; then
    echo "error: no Wayland display found in $RUNTIME_DIR" >&2
    echo "Run this from Desktop Mode, or launch Goro through Steam in Gaming Mode." >&2
    exit 2
fi

if [[ ! -x "$BUILD_DIR/goro" ]]; then
    echo "error: executable not found or not executable: $BUILD_DIR/goro" >&2
    exit 2
fi

if [[ ! -f "$BUILD_DIR/libSDL3.so.0" ]]; then
    echo "warning: bundled libSDL3.so.0 was not found; controller support may be unavailable" >&2
fi

mkdir -p "$BUILD_DIR/logs"
LOG_FILE="$BUILD_DIR/logs/goro-$(date +%Y%m%d-%H%M%S).log"

echo "Wayland: $RUNTIME_DIR/$WAYLAND_DISPLAY"
echo "Log:     $LOG_FILE"
echo "Starting Goro; output will be shown live and saved to the log."

export XDG_RUNTIME_DIR="$RUNTIME_DIR"
export WAYLAND_DISPLAY
export GORO_SDL3_PATH="${GORO_SDL3_PATH:-$BUILD_DIR/libSDL3.so.0}"

cd "$BUILD_DIR"
./goro "$@" 2>&1 | tee "$LOG_FILE"
status=${PIPESTATUS[0]}

echo "Goro exited with status $status; log saved to $LOG_FILE" >&2
exit "$status"
