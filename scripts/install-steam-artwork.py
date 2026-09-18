#!/usr/bin/env python3
"""Install local artwork for an existing Steam/SteamOS non-Steam shortcut.

This helper intentionally READS shortcuts.vdf but does not rewrite it.  Steam
assigns the shortcut AppID; we discover that stored AppID and use it to name
custom library artwork in userdata/<account>/config/grid.

Intended usage on Steam Deck:

    python3 install-steam-artwork.py \
        --gameid goro \
        --exe /home/deck/devkit-game/goro/goro \
        --art-dir /home/deck/devkit-game/goro/steam-artwork

Required artwork files in --art-dir:
    grid.png       portrait capsule
    gridwide.png   landscape capsule
    hero.png       library hero background
    logo.png       transparent logo overlay
    icon.png       shortcut icon source (best-effort cache copy)
"""

from __future__ import annotations

import argparse
import shutil
import struct
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Iterable


class VDFError(RuntimeError):
    pass


def _read_cstring(data: bytes, pos: int) -> tuple[str, int]:
    end = data.find(b"\x00", pos)
    if end < 0:
        raise VDFError("unterminated VDF string")
    raw = data[pos:end]
    return raw.decode("utf-8", errors="replace"), end + 1


def _read_wstring(data: bytes, pos: int) -> tuple[str, int]:
    end = pos
    while end + 1 < len(data):
        if data[end : end + 2] == b"\x00\x00":
            raw = data[pos:end]
            return raw.decode("utf-16-le", errors="replace"), end + 2
        end += 2
    raise VDFError("unterminated VDF UTF-16 string")


def _parse_object(data: bytes, pos: int = 0, *, root: bool = False) -> tuple[dict[str, Any], int]:
    result: dict[str, Any] = {}

    while pos < len(data):
        value_type = data[pos]
        pos += 1

        # TYPE_END
        if value_type == 0x08:
            return result, pos

        key, pos = _read_cstring(data, pos)

        if value_type == 0x00:  # TYPE_NONE / nested object
            value, pos = _parse_object(data, pos)
        elif value_type == 0x01:  # TYPE_STRING
            value, pos = _read_cstring(data, pos)
        elif value_type == 0x02:  # TYPE_INT
            if pos + 4 > len(data):
                raise VDFError("truncated VDF int32")
            value = struct.unpack_from("<i", data, pos)[0]
            pos += 4
        elif value_type == 0x03:  # TYPE_FLOAT
            if pos + 4 > len(data):
                raise VDFError("truncated VDF float")
            value = struct.unpack_from("<f", data, pos)[0]
            pos += 4
        elif value_type == 0x04:  # TYPE_PTR (encoded as 32-bit value)
            if pos + 4 > len(data):
                raise VDFError("truncated VDF pointer")
            value = struct.unpack_from("<I", data, pos)[0]
            pos += 4
        elif value_type == 0x05:  # TYPE_WSTRING
            value, pos = _read_wstring(data, pos)
        elif value_type == 0x06:  # TYPE_COLOR
            if pos + 4 > len(data):
                raise VDFError("truncated VDF color")
            value = data[pos : pos + 4]
            pos += 4
        elif value_type == 0x07:  # TYPE_UINT64
            if pos + 8 > len(data):
                raise VDFError("truncated VDF uint64")
            value = struct.unpack_from("<Q", data, pos)[0]
            pos += 8
        elif value_type == 0x09:  # TYPE_INT64 on newer KeyValues readers
            if pos + 8 > len(data):
                raise VDFError("truncated VDF int64")
            value = struct.unpack_from("<q", data, pos)[0]
            pos += 8
        else:
            raise VDFError(f"unsupported VDF value type 0x{value_type:02x} for key {key!r}")

        result[key] = value

    if root:
        return result, pos
    raise VDFError("unexpected end of VDF object")


def parse_shortcuts_vdf(path: Path) -> dict[str, Any]:
    data = path.read_bytes()
    parsed, _ = _parse_object(data, 0, root=True)
    return parsed


def _ci(entry: dict[str, Any]) -> dict[str, Any]:
    return {str(k).casefold(): v for k, v in entry.items()}


def _unquote_path(value: str) -> str:
    value = value.strip()
    if len(value) >= 2 and value[0] == value[-1] == '"':
        value = value[1:-1]
    return value


def steam_roots(home: Path) -> list[Path]:
    candidates = [
        home / ".local" / "share" / "Steam",
        home / ".steam" / "steam",
        home / ".steam" / "root",
    ]
    result: list[Path] = []
    seen: set[Path] = set()
    for candidate in candidates:
        try:
            resolved = candidate.resolve()
        except OSError:
            resolved = candidate
        if candidate.exists() and resolved not in seen:
            result.append(candidate)
            seen.add(resolved)
    return result


def shortcut_files(home: Path) -> Iterable[Path]:
    seen: set[Path] = set()
    for root in steam_roots(home):
        userdata = root / "userdata"
        if not userdata.is_dir():
            continue
        for path in userdata.glob("*/config/shortcuts.vdf"):
            try:
                resolved = path.resolve()
            except OSError:
                resolved = path
            if resolved not in seen:
                seen.add(resolved)
                yield path


@dataclass(frozen=True)
class ShortcutMatch:
    shortcuts_path: Path
    entry: dict[str, Any]
    appid: int
    score: int
    reason: str


def _iter_entries(document: dict[str, Any]) -> Iterable[dict[str, Any]]:
    root = _ci(document)
    shortcuts = root.get("shortcuts")
    if not isinstance(shortcuts, dict):
        return
    for key in sorted(shortcuts, key=lambda s: int(s) if str(s).isdigit() else 1 << 30):
        entry = shortcuts[key]
        if isinstance(entry, dict):
            yield entry


def score_entry(entry: dict[str, Any], *, gameid: str, appname: str | None, exe: str | None) -> tuple[int, str]:
    fields = _ci(entry)
    candidate_gameid = str(fields.get("devkitgameid", "")).strip()
    candidate_name = str(fields.get("appname", "")).strip()
    candidate_exe = _unquote_path(str(fields.get("exe", ""))).strip()

    score = 0
    reasons: list[str] = []

    if gameid and candidate_gameid.casefold() == gameid.casefold():
        score += 100
        reasons.append("DevkitGameID")

    expected_devkit_name = f"Devkit Game: {gameid}" if gameid else ""
    if expected_devkit_name and candidate_name.casefold() == expected_devkit_name.casefold():
        score += 60
        reasons.append("Devkit AppName")

    if appname and candidate_name.casefold() == appname.casefold():
        score += 50
        reasons.append("AppName")

    if exe:
        try:
            wanted = str(Path(exe))
        except Exception:
            wanted = exe
        if candidate_exe == wanted:
            score += 80
            reasons.append("Exe")
        elif candidate_exe.endswith("/" + Path(wanted).name):
            score += 15
            reasons.append("Exe basename")

    return score, "+".join(reasons) if reasons else "no match"


def find_shortcut(home: Path, *, gameid: str, appname: str | None, exe: str | None) -> ShortcutMatch:
    matches: list[ShortcutMatch] = []
    parse_errors: list[str] = []

    for path in shortcut_files(home):
        try:
            document = parse_shortcuts_vdf(path)
        except Exception as exc:
            parse_errors.append(f"{path}: {exc}")
            continue

        for entry in _iter_entries(document):
            fields = _ci(entry)
            raw_appid = fields.get("appid")
            if not isinstance(raw_appid, int):
                continue

            score, reason = score_entry(entry, gameid=gameid, appname=appname, exe=exe)
            if score <= 0:
                continue

            matches.append(
                ShortcutMatch(
                    shortcuts_path=path,
                    entry=entry,
                    appid=raw_appid & 0xFFFFFFFF,
                    score=score,
                    reason=reason,
                )
            )

    if not matches:
        detail = ""
        if parse_errors:
            detail = "\nCould not parse:\n  " + "\n  ".join(parse_errors)
        raise RuntimeError(
            f"could not find a Steam shortcut for DevkitGameID={gameid!r}. "
            "Create/update the devkit shortcut first." + detail
        )

    matches.sort(key=lambda item: item.score, reverse=True)
    best = matches[0]
    if len(matches) > 1 and matches[1].score == best.score and matches[1].appid != best.appid:
        raise RuntimeError(
            "multiple Steam shortcuts matched equally; specify --appname and/or --exe more precisely"
        )
    return best


ARTWORK = {
    "grid.png": "{appid}p.png",
    "gridwide.png": "{appid}.png",
    "hero.png": "{appid}_hero.png",
    "logo.png": "{appid}_logo.png",
}


def install_artwork(match: ShortcutMatch, art_dir: Path, *, dry_run: bool = False) -> Path:
    missing = [name for name in [*ARTWORK, "icon.png"] if not (art_dir / name).is_file()]
    if missing:
        raise RuntimeError("missing artwork file(s): " + ", ".join(missing))

    grid_dir = match.shortcuts_path.parent / "grid"
    if not dry_run:
        grid_dir.mkdir(parents=True, exist_ok=True)

    for source_name, target_template in ARTWORK.items():
        source = art_dir / source_name
        target = grid_dir / target_template.format(appid=match.appid)
        print(f"  {source_name:12s} -> {target.name}")
        if not dry_run:
            shutil.copy2(source, target)

    # Steam treats shortcut icons differently from Library artwork: the
    # shortcut's `icon` field/SteamClient.SetShortcutIcon is authoritative.
    # Keep conventional cache copies available without rewriting shortcuts.vdf
    # while Steam is running.  This makes the helper safe and gives a future
    # live SetShortcutIcon integration stable files to point at.
    icon_source = art_dir / "icon.png"
    for icon_name in (f"{match.appid}_icon.png", f"{match.appid}-icon.png"):
        icon_target = grid_dir / icon_name
        print(f"  {'icon.png':12s} -> {icon_target.name} (icon cache)")
        if not dry_run:
            shutil.copy2(icon_source, icon_target)

    return grid_dir


def main() -> int:
    parser = argparse.ArgumentParser(description="Install bundled Goro artwork for its Steam devkit shortcut.")
    parser.add_argument("--gameid", default="goro", help="DevkitGameID to match (default: goro)")
    parser.add_argument("--appname", default=None, help="Optional exact Steam shortcut name")
    parser.add_argument("--exe", default=None, help="Optional executable path used to disambiguate the shortcut")
    parser.add_argument("--art-dir", required=True, type=Path, help="Directory containing the five artwork PNGs")
    parser.add_argument("--home", type=Path, default=Path.home(), help=argparse.SUPPRESS)
    parser.add_argument("--dry-run", action="store_true", help="Resolve the shortcut and show target names without copying")
    args = parser.parse_args()

    try:
        art_dir = args.art_dir.expanduser().resolve()
        match = find_shortcut(
            args.home.expanduser(),
            gameid=args.gameid,
            appname=args.appname,
            exe=args.exe,
        )
        fields = _ci(match.entry)
        display_name = fields.get("appname", args.gameid)

        print(f"Steam shortcut: {display_name}")
        print(f"Shortcut AppID: {match.appid} ({match.reason})")
        grid_dir = install_artwork(match, art_dir, dry_run=args.dry_run)
        print(f"Artwork directory: {grid_dir}")
        if args.dry_run:
            print("Dry run only; no files changed.")
        else:
            print("Library artwork installed.")
            print("Note: Steam may need a UI/client restart before changed artwork is visible.")
            print("Note: icon cache files are installed, but this helper intentionally does not rewrite shortcuts.vdf.")
        return 0
    except Exception as exc:
        print(f"install-steam-artwork: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
