"""A signed, tagged pre-registration over a throwaway instance list.

Every driver test has to get past ``preflight`` before it can exercise anything
else, and the gate is deliberately unforgiving: SIGNED, tagged ``prereg-m4``,
and describing the very ``instances.txt`` about to be read. Building that repo
by hand in each test would mean restating the contract in each test, so it lives
here once and the tests state only what they are actually about.
"""

from __future__ import annotations

import hashlib
import subprocess
from pathlib import Path

import pytest

PREREG = """---
status: {status}
sample_size: {n}
sample_seed: 20260826
instance_list_sha256: {sha}
model: Qwen3-Coder-30B-A3B-Instruct
arms: [vanilla, tdd, tdad, rtdd, rtdd_tdd]
stratified_recall_floor: 0.95
vanilla_equivalence_k: 2
signed_by: VocanicZ
signed_at: 2026-08-27
---

body
"""


def _git(repo: Path, *args: str) -> None:
    subprocess.run(["git", "-C", str(repo), *args], check=True, capture_output=True)


@pytest.fixture
def make_repo(tmp_path):
    """Build a repo whose pre-registration covers ``ids``; ``status`` picks the gate's verdict."""

    def build(ids: list[str], *, status: str = "SIGNED", tag: bool = True) -> Path:
        root = tmp_path / "repo"
        (root / "bench" / "swebench").mkdir(parents=True, exist_ok=True)
        text = "\n".join(ids) + "\n"
        (root / "bench" / "swebench" / "instances.txt").write_text(text, encoding="utf-8")
        sha = hashlib.sha256(text.encode("utf-8")).hexdigest()
        (root / "bench" / "PREREGISTRATION.md").write_text(
            PREREG.format(status=status, n=len(ids), sha=sha), encoding="utf-8"
        )
        subprocess.run(["git", "init", "-q", str(root)], check=True)
        _git(root, "config", "user.email", "t@t")
        _git(root, "config", "user.name", "t")
        _git(root, "add", "-A")
        _git(root, "commit", "-qm", "prereg")
        if tag:
            _git(root, "tag", "prereg-m4")
        return root

    return build
