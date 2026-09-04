# Desktop controller input

Goro's desktop build supports SDL3-standardized gamepads, including a PS5
DualSense over USB or Bluetooth. The backend uses positional gamepad controls,
so it does not depend on a vendor/product ID or an Xbox-style button label.
Android keeps its existing touch input path and excludes SDL.

Implementation references: [go-sdl3](https://github.com/Zyko0/go-sdl3), the
[SDL3 Gamepad API](https://wiki.libsdl.org/SDL3/CategoryGamepad), and SDL's
[standard gamepad definitions](https://github.com/libsdl-org/SDL/blob/main/include/SDL3/SDL_gamepad.h).

## Two independent modes

Controller behaviour is governed by two orthogonal settings, both persisted and
both changeable from Settings → Controller without a restart.

`move_mode` decides what the left stick does in the world:

- **`character`** (default) — the left stick and D-pad walk directly, through the
  same semantic `input.PlayerCommand` path as before: an 8-cell walk horizon,
  3-cell refill, cooldowns, and server-approved stop behaviour. The right stick
  drives the on-screen pointer. While the pointer is over the UI, walking is
  suppressed so aiming at a button does not also walk the player into it.
- **`cursor`** — the left stick drives the pointer and the player walks by
  clicking the ground, exactly as with a mouse. Because the click travels the
  real click path, picking up items, talking to NPCs, attacking, warps and
  vending all work with no controller-specific code.

`ui_nav_mode` decides how menus are driven:

- **`cursor`** (default) — the pointer moves over the UI and the confirm button
  clicks. Everything a mouse can reach is reachable: window drag and resize,
  drag & drop between bags, the minimap, context menus, sliders.
- **`focus`** — the stick and D-pad hop between focusable widgets and confirm
  activates the focused one. No pointer is involved.

The pointer is virtual: goro already hides the OS cursor and draws the RO cursor
sprite at the input position, so the sprite follows the stick with all of its
existing hover and snap behaviour intact. Stick input passes through a radial
deadzone and then a direction-preserving quadratic response curve, so small
deflections give fine control without skewing diagonals.

## Default action map

| DualSense control | World action | UI action |
| --- | --- | --- |
| Left stick / D-pad | Eight-way movement (character mode) or pointer (cursor mode) | Pointer, or focus navigation |
| Right stick | Camera, or pointer in character mode | Scroll |
| Cross / Circle | Interact or confirm / cancel or stop | Click and activate / back |
| Square / Triangle | Attack focused target / loot | Alternate / secondary action |
| L1 / R1 | Previous / next target | Previous / next focus |
| L2 / R2 (analog) | Camera zoom | — |
| L2 + Cross/Circle/Square/Triangle | Shortcut slots 1-4 | — |
| R2 + Cross/Circle/Square/Triangle | Shortcut slots 5-8 | — |
| Right-stick click / R3 | Reset camera orientation and distance | — |
| Options (tap) | Escape menu | Back |
| Options (hold) | Controller setup | Controller setup |
| Touchpad | Map | Map |

The triggers carry both zoom and the shortcut modifiers. They coexist because
zoom is a continuous analog reading while shortcuts fire on a face-button edge:
zoom is suppressed on any frame a shortcut fires, so neither needs rebinding.

Note a deliberate divergence from the C++ reference client: the shoulders cycle
targets rather than paging the hotbar, because goro's shortcut bar has visible
rows rather than pages.

## Rebinding

Settings → Controller → *Controller setup…*, or hold Options in the world, opens
the controller page. It shows live device diagnostics — name, kind, stick and
trigger values, and the currently held buttons — which is the fastest way to
confirm hardware is being read correctly.

Selecting a binding row captures the next button pressed. Bindings are kept
unique: assigning a button that another action already holds moves that action
to its default rather than leaving two actions on one button. Only the physical
button is configurable; what each action *does* stays in the input resolve and
dispatch code. Controller dispatch is suppressed while a capture is open, so the
button being bound does not also fire the action it is being bound to.

## Configuration

```ini
[controller]
enabled = true
deadzone = 0.15
outer_deadzone = 0.95
trigger_deadzone = 0.10
camera_sensitivity = 1.00
invert_camera_y = false
move_mode = character        ; character | cursor
ui_nav_mode = cursor         ; cursor | focus
cursor_speed = 1400.00       ; pixels per second at full deflection
nav_repeat_delay_ms = 350    ; hold before focus navigation repeats
nav_repeat_ms = 110          ; interval between repeats
rumble = true
confirm_button = south
cancel_button = east
attack_button = west
loot_button = north
target_previous = left_shoulder
target_next = right_shoulder
reset_camera = right_stick
menu_button = start
map_button = touchpad
left_modifier = left_trigger
right_modifier = right_trigger
```

Changes made in the settings window are applied to the running client
immediately and written back to this section without disturbing unrelated
settings.

## Safety behaviour

Movement is explicitly released — not merely skipped — whenever the pad can no
longer own it: on disconnect, on switching away from character move mode, when
the controller is disabled, and on window focus loss. Without this a controller
unplugged mid-walk leaves the character walking. A held pointer button is
released on the same events.

## Text entry

The on-screen keyboard opens automatically whenever text entry becomes active,
including the chat console, and consumes the whole frame while open — no pointer
motion, no zoom, no walking behind it.

NPC dialogs arm confirm only after it has been observed released, so the press
that opens a dialog cannot also advance past its first page.

## Shipping the SDL3 runtime

Controller support loads SDL3 at runtime. `gamepad.loadSDL3` searches, in order:
`$GORO_SDL3_PATH`, an absolute path beside the executable, `<exedir>/lib`, on
macOS `<exedir>/../Frameworks`, and finally the bare library name resolved by the
platform loader. Absolute paths come first so a release archive uses the copy it
shipped with, and so Windows cannot be induced to load an `SDL3.dll` planted in
the working directory.

- **Windows** — the release `.zip` archives bundle `SDL3.dll` beside the
  executable. The pinned artifacts and their checksums are in
  `packaging/sdl3/checksums.txt`; the release workflow verifies them.
- **Linux** — install the distribution's SDL3 package (`libsdl3-0` or
  equivalent) so `libSDL3.so.0` resolves through the system loader.
- **macOS** — install SDL3 (`brew install sdl3`) so `libSDL3.dylib` resolves.

If no runtime is found, goro logs the failure at warning level and continues
with keyboard and mouse input.

## Scripting

If `--script` selects a Lua input profile, that profile owns the frame and
production movement is suppressed to avoid duplicate requests. The Lua API still
receives normalized `goro.actions()` and `goro.controller()` state.

## Hardware validation checklist

Most of the behaviour above is covered by unit tests that need neither SDL nor a
physical device. The following genuinely require a controller and are **not**
covered by tests:

- SDL mapping correctness for the touchpad click and analog L2/R2 travel.
- HIDAPI hint ordering actually selecting the DualSense driver over **both** USB
  and Bluetooth.
- `EVENT_GAMEPAD_ADDED` / `EVENT_GAMEPAD_REMOVED` delivery, and the once-a-second
  re-enumeration fallback firing when an event is missed.
- `SDL_JOYSTICK_ALLOW_BACKGROUND_EVENTS` behaviour on window focus loss.
- Rumble and LED output.
- The `runtime.LockOSThread` interaction between go-sdl3's package init and
  gogpu's update goroutine.
- Pointer hit-testing at UI scale 0.85 and 1.20.
- A release archive on a clean machine with no SDL3 installed.
