"""The bench tests that drive a real `rtdd` need a v0.3.0 one.

v0.3.0 builds a node graph (`rtdd graph`) and emits `rtdd which --json` schema 3;
v0.2 seeded a coverage map and emitted schema 2. A test that executes the binary
runs only when the `rtdd` on PATH has `rtdd graph`, so a machine with an old
install skips it instead of failing for a reason no branch change can fix.
"""
from __future__ import annotations

import shutil
import subprocess

import pytest


def rtdd_on_path_is_v03() -> bool:
    exe = shutil.which("rtdd")
    if exe is None:
        return False
    out = subprocess.run([exe, "--help"], capture_output=True, text=True, check=False)
    return "rtdd graph" in out.stdout + out.stderr


requires_v03_rtdd = pytest.mark.skipif(
    not rtdd_on_path_is_v03(),
    reason="needs a v0.3.0 rtdd on PATH (rtdd graph, which --json schema 3)",
)
