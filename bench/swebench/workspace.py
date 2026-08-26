"""Per-instance checkout of the target repo at its base commit.

Repos are cloned once into a bare mirror cache and then worktree'd per instance,
so a five-arm run over 100 instances does not clone django 500 times. The mirror
is fetched only when the instance's ``base_commit`` is not already in it, so a
warm cache costs no network at all.
"""

from __future__ import annotations

import os
import shutil
import subprocess
from dataclasses import dataclass
from pathlib import Path

#: Overridable so tests (and any private mirror) can point somewhere other than
#: github.com. ``{repo}`` is the dataset's ``owner/name``.
GIT_URL_TEMPLATE_ENV = "RTDD_BENCH_GIT_URL_TEMPLATE"
DEFAULT_GIT_URL_TEMPLATE = "https://github.com/{repo}.git"


@dataclass
class Workspace:
    root: Path
    instance_id: str
    repo: str
    base_commit: str


def remote_url(repo: str) -> str:
    template = os.environ.get(GIT_URL_TEMPLATE_ENV) or DEFAULT_GIT_URL_TEMPLATE
    return template.format(repo=repo)


def _run(cwd: Path | None, *args: str, check: bool = True) -> subprocess.CompletedProcess:
    proc = subprocess.run(args, cwd=cwd, capture_output=True, text=True)
    if check and proc.returncode != 0:
        raise RuntimeError(f"{' '.join(args)} failed: {proc.stderr.strip()}")
    return proc


def _has_commit(mirror: Path, sha: str) -> bool:
    return _run(mirror, "git", "cat-file", "-e", f"{sha}^{{commit}}", check=False).returncode == 0


def _mirror(cache: Path, repo: str, base_commit: str) -> Path:
    target = cache / repo.replace("/", "__")
    if not target.exists():
        target.parent.mkdir(parents=True, exist_ok=True)
        _run(None, "git", "clone", "--quiet", "--mirror", remote_url(repo), str(target))
    elif not _has_commit(target, base_commit):
        _run(target, "git", "fetch", "--quiet", "--prune", "--tags", "origin")
    return target


def prepare(cache: Path, work: Path, row: dict) -> Workspace:
    """Check the instance's repo out at ``base_commit`` in its own worktree."""
    repo = row["repo"]
    base_commit = row["base_commit"]
    instance_id = row["instance_id"]
    mirror = _mirror(cache, repo, base_commit)

    root = work / instance_id
    if root.exists():
        shutil.rmtree(root)
    _run(mirror, "git", "worktree", "prune")
    root.parent.mkdir(parents=True, exist_ok=True)
    _run(mirror, "git", "worktree", "add", "--quiet", "--detach", str(root), base_commit)
    _run(root, "git", "config", "user.email", "bench@rtdd.local")
    _run(root, "git", "config", "user.name", "rtdd-bench")
    return Workspace(root=root, instance_id=instance_id, repo=repo, base_commit=base_commit)


def diff(ws: Workspace) -> str:
    """The prediction: every tracked and untracked change, as one patch.

    An agent that changed nothing yields ``""`` — an empty prediction, not an error.
    """
    _run(ws.root, "git", "add", "-A", "-N")
    return _run(ws.root, "git", "diff", "--no-color", "--binary").stdout
