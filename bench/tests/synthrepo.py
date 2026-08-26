"""The synthetic replay repo every later bench test is verified against.

The history and its outcomes are hand-computed in
`docs/plans/04-m3-replay-benchmark.md` Task 4 and reproduced in `EXPECTED`
below. That table is normative: change the builder and the table together, or
not at all.

| Commit | Change                                            | Full suite     | F_full vs parent |
|--------|---------------------------------------------------|----------------|------------------|
| c0     | root: alpha, beta + their tests                   | all pass       | —                |
| c1     | src/alpha.py: `a + b` -> `a + b + 1`              | test_add fails | {test_add}       |
| c2     | revert alpha; add src/gamma.py + tests/test_gamma | all pass       | empty (green)    |
| c3     | src/beta.py: `a * b` -> `a * b + 1`; touch gamma  | test_mul fails | {test_mul}       |
"""

from __future__ import annotations

import dataclasses
import os
import pathlib
import subprocess

ENV_OVERRIDES = {
    "GIT_AUTHOR_NAME": "bench",
    "GIT_AUTHOR_EMAIL": "bench@example.invalid",
    "GIT_COMMITTER_NAME": "bench",
    "GIT_COMMITTER_EMAIL": "bench@example.invalid",
    "GIT_AUTHOR_DATE": "2026-01-01T00:00:00+00:00",
    "GIT_COMMITTER_DATE": "2026-01-01T00:00:00+00:00",
}


@dataclasses.dataclass(frozen=True)
class CommitOutcome:
    """One row of the hand-computed history table."""

    index: int
    message: str
    returncode: int
    failures: tuple[str, ...]
    f_full: tuple[str, ...]


EXPECTED: tuple[CommitOutcome, ...] = (
    CommitOutcome(0, "c0: alpha and beta", 0, (), ()),
    CommitOutcome(
        1,
        "c1: off-by-one in alpha",
        1,
        ("tests/test_alpha.py::test_add",),
        ("tests/test_alpha.py::test_add",),
    ),
    CommitOutcome(2, "c2: fix alpha, add gamma", 0, (), ()),
    CommitOutcome(
        3,
        "c3: off-by-one in beta",
        1,
        ("tests/test_beta.py::test_mul",),
        ("tests/test_beta.py::test_mul",),
    ),
)


@dataclasses.dataclass(frozen=True)
class SynthRepo:
    path: pathlib.Path
    commits: tuple[str, ...]

    def sha(self, index: int) -> str:
        return self.commits[index]


def _git(path: pathlib.Path, *args: str) -> str:
    env = dict(os.environ)
    env.update(ENV_OVERRIDES)
    proc = subprocess.run(
        ["git", *args], cwd=path, env=env, capture_output=True, text=True, check=True
    )
    return proc.stdout.strip()


def _write(path: pathlib.Path, rel: str, text: str) -> None:
    target = path / rel
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(text, encoding="utf-8")


def _commit(path: pathlib.Path, message: str) -> str:
    _git(path, "add", "-A")
    _git(path, "commit", "-q", "--no-gpg-sign", "-m", message)
    return _git(path, "rev-parse", "HEAD")


def build_synth_repo(root: pathlib.Path) -> SynthRepo:
    root.mkdir(parents=True, exist_ok=True)
    _git(root, "init", "-q", "-b", "main")
    _git(root, "config", "user.name", "bench")
    _git(root, "config", "user.email", "bench@example.invalid")

    shas: list[str] = []

    # --- c0: green root -------------------------------------------------
    _write(root, "pytest.ini", "[pytest]\ntestpaths = tests\n")
    _write(root, "src/__init__.py", "")
    _write(root, "src/alpha.py", "def add(a, b):\n    return a + b\n")
    _write(root, "src/beta.py", "def mul(a, b):\n    return a * b\n")
    _write(
        root,
        "tests/test_alpha.py",
        "from src.alpha import add\n\n\ndef test_add():\n    assert add(1, 2) == 3\n",
    )
    _write(
        root,
        "tests/test_beta.py",
        "from src.beta import mul\n\n\ndef test_mul():\n    assert mul(2, 3) == 6\n",
    )
    shas.append(_commit(root, EXPECTED[0].message))

    # --- c1: break alpha ------------------------------------------------
    _write(root, "src/alpha.py", "def add(a, b):\n    return a + b + 1\n")
    shas.append(_commit(root, EXPECTED[1].message))

    # --- c2: fix alpha, add gamma (green) -------------------------------
    _write(root, "src/alpha.py", "def add(a, b):\n    return a + b\n")
    _write(root, "src/gamma.py", "def sub(a, b):\n    return a - b\n")
    _write(
        root,
        "tests/test_gamma.py",
        "from src.gamma import sub\n\n\ndef test_sub():\n    assert sub(5, 2) == 3\n",
    )
    shas.append(_commit(root, EXPECTED[2].message))

    # --- c3: break beta, touch gamma harmlessly -------------------------
    _write(root, "src/beta.py", "def mul(a, b):\n    return a * b + 1\n")
    _write(root, "src/gamma.py", "def sub(a, b):\n    # subtraction\n    return a - b\n")
    shas.append(_commit(root, EXPECTED[3].message))

    _git(root, "checkout", "-q", shas[-1])
    return SynthRepo(path=root, commits=tuple(shas))
