"""The bench tests that drive a real `rtdd` need a v0.2 one.

The replay harness's rtdd strategy seeds a coverage map (`rtdd seed`) and reads
`rtdd which --json` schema 2. rtdd v0.3.0 removes both (PRD #410); PRD #412 moves the
strategy to `rtdd graph` and schema 3 and deletes this file. Until then a test that
executes the binary runs only when the `rtdd` on PATH still has `rtdd seed`.
"""
from __future__ import annotations

import shutil
import subprocess

import pytest


def rtdd_on_path_is_v02() -> bool:
    exe = shutil.which("rtdd")
    if exe is None:
        return False
    out = subprocess.run([exe, "--help"], capture_output=True, text=True, check=False)
    return "rtdd seed" in out.stdout


requires_v02_rtdd = pytest.mark.skipif(
    not rtdd_on_path_is_v02(),
    reason="needs a v0.2 rtdd on PATH (seed, which --json schema 2); PRD #412 moves the bench to schema 3",
)
