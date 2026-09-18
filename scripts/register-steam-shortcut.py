#!/usr/bin/env python3
"""Register/update a SteamOS Devkit shortcut without shell-quoting JSON.

The SteamOS helper expects the entire JSON document as the value of one
--parms argument. This wrapper reads the JSON from disk and invokes the helper
with subprocess argv, avoiding PowerShell/SSH/remote-shell word splitting.
"""

from __future__ import annotations

import argparse
import json
from pathlib import Path
import subprocess
import sys


DEFAULT_TOOL = Path.home() / "devkit-utils" / "steam-client-create-shortcut"


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--parms-file",
        required=True,
        type=Path,
        help="Path to the uploaded steam-shortcut.json file.",
    )
    parser.add_argument(
        "--tool",
        type=Path,
        default=DEFAULT_TOOL,
        help="Path to steam-client-create-shortcut.",
    )
    args = parser.parse_args()

    parms_file = args.parms_file.expanduser().resolve()
    tool = args.tool.expanduser()

    if not parms_file.is_file():
        parser.error(f"parameter file does not exist: {parms_file}")
    if not tool.is_file():
        parser.error(f"Steam Devkit shortcut tool does not exist: {tool}")

    # Validate before invoking the Devkit utility. Re-serialize compactly so
    # the child receives a canonical JSON string as exactly one argv element.
    try:
        document = json.loads(parms_file.read_text(encoding="utf-8-sig"))
    except (OSError, UnicodeError, json.JSONDecodeError) as exc:
        parser.error(f"could not read valid JSON from {parms_file}: {exc}")

    payload = json.dumps(document, ensure_ascii=False, separators=(",", ":"))

    command = [
        sys.executable,
        str(tool),
        "--parms",
        payload,
    ]

    completed = subprocess.run(command, check=False)
    return completed.returncode


if __name__ == "__main__":
    raise SystemExit(main())
