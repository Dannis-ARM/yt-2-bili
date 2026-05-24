from __future__ import annotations

import os
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
BIN = ROOT / "yt-2-bili.exe"
DEFAULT_URL = "https://www.youtube.com/watch?v=mGEfasQl2Zo"
DEFAULT_COOKIE = Path.home() / "cookies.json"


def run(command: list[str], cwd: Path | None = None) -> None:
    print("+ " + " ".join(f'"{part}"' if " " in part else part for part in command), flush=True)
    subprocess.run(command, cwd=cwd or ROOT, check=True)


def build_cli() -> None:
    print("Building yt-2-bili...", flush=True)
    run(["go", "build", "-o", str(BIN), "./cmd/yt-2-bili"])


def reset_dir(path: Path) -> None:
    if path.exists():
        shutil.rmtree(path)
    path.mkdir(parents=True, exist_ok=True)


def temp_case_dir(name: str) -> Path:
    return Path(tempfile.gettempdir()) / name


def require_file(path: Path, message: str) -> None:
    if not path.exists():
        raise SystemExit(f"{message}: {path}")


def require_tool(name: str) -> None:
    if shutil.which(name) is None:
        raise SystemExit(f"{name} is required but was not found in PATH")


def first_match(directory: Path, patterns: list[str]) -> Path:
    for pattern in patterns:
        matches = sorted(directory.glob(pattern))
        if matches:
            return matches[0]
    raise SystemExit(f"Expected one of {patterns} in {directory}")


def cookie_from_argv(argv: list[str], index: int = 1) -> Path:
    if len(argv) > index:
        return Path(argv[index]).expanduser().resolve()
    return DEFAULT_COOKIE
