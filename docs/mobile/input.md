# Mobile input foundation

The mobile client uses a platform-independent semantic input boundary:

```text
input.State
    ├── DesktopInputAdapter
    └── GestureRecognizer -> MobileInputAdapter
                              ↓
                        PlayerCommand
```

Commands describe player intent (`MoveTo`, `AttackActor`, `ZoomCamera`, and so
on). They do not expose mouse buttons, key codes, touch IDs, screen coordinates,
Android events, or packet opcodes.

Gesture thresholds are centralized in `input.GestureConfig`. UI ownership is
checked before world gesture processing; a consumed touch cannot emit a world
command.

World gesture policy is deliberately separated by finger count: one-finger
dragging follows walkable ground when Hold-to-move is enabled; it never rotates
the camera. Two-finger dragging rotates the camera, pinch distance zooms it,
and a short two-finger tap performs secondary actor inspection. Moving a finger
over an actor also refreshes the target name without attacking or interacting.

The desktop adapter and Android host both feed the same `game.WorldMode`
consumer. In offline mode, world mode applies movement, target selection, NPC
interaction, item use/equip/unequip/drop, and camera commands locally. In online
mode, existing network/gameplay helpers remain authoritative. `WorldMode`
dispatch stays the compatibility consumer for branches a command-backed slice
has not yet replaced.

The package remains Android- and renderer-independent. Android lifecycle and
surface ownership live only under `android/host`.
