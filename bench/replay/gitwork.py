"""Clones, detached worktrees, commit enumeration, diff parsing, and tree
materialisation for both replay variants.

Plain `git` subprocess — no GitPython. The corpus repos are pinned at unusual
commits with unusual layouts; stdlib `subprocess` is the fewest moving parts.

The two materialisations are the heart of the replay protocol:

`natural` — the worktree sits at the parent `P` and the child `C`'s content is
written over it *without moving HEAD*. Added files land untracked, deletions land
as removals, and `--no-renames` means a rename arrives as delete + add. That is
exactly the mid-cycle working tree `--base HEAD` must see, and exactly the class
of change audit A8 says mutation testing cannot produce.

`probe` — the worktree sits at `C` and only the *source* half of `C`'s diff is
reverted back to `P`. The tree is `C`'s tests over `P`'s source; HEAD stays at
`C`, so the map is seeded at `C` and every map-based strategy gets the same
upper-bound advantage.
"""

from __future__ import annotations

import dataclasses
import fnmatch
import os
import pathlib
import shutil
import subprocess
from collections.abc import Sequence

_ENV = {
    "GIT_TERMINAL_PROMPT": "0",
    "GIT_CONFIG_NOSYSTEM": "1",
}


class GitError(RuntimeError):
    """A git invocation failed, or a clone is not at the corpus pin."""


@dataclasses.dataclass(frozen=True)
class Change:
    """One changed path. `path` is repo-relative and slash-separated."""

    path: str
    status: str  # "A" | "M" | "D"


@dataclasses.dataclass(frozen=True)
class ReplayPoint:
    """A commit and the parent its replay worktree is built from."""

    commit: str
    parent: str


def git(cwd: pathlib.Path, *args: str, check: bool = True) -> str:
    env = dict(os.environ)
    env.update(_ENV)
    proc = subprocess.run(
        ["git", *args], cwd=cwd, env=env, capture_output=True, text=True
    )
    if check and proc.returncode != 0:
        raise GitError(f"git {' '.join(args)} failed in {cwd}: {proc.stderr.strip()}")
    # Only the trailing newline: `status --porcelain` leads with a significant
    # space (" M path"), which a bare .strip() would eat.
    return proc.stdout.rstrip("\n")


def clone_pinned(url: str, pin: str, dest: pathlib.Path) -> pathlib.Path:
    """Clone `url` into `dest` and check out `pin`.

    An existing clone is reused only when it is already at the pin; a clone that
    has moved is a `GitError` rather than a silent re-checkout, because a moved
    clone means someone has already replayed against the wrong history.
    """
    if dest.exists():
        head = git(dest, "rev-parse", "HEAD")
        if head != pin:
            raise GitError(
                f"{dest} is at {head}, corpus pin is {pin}; delete it and re-clone"
            )
        return dest
    dest.parent.mkdir(parents=True, exist_ok=True)
    proc = subprocess.run(
        ["git", "clone", "--quiet", url, str(dest)],
        env={**os.environ, **_ENV},
        capture_output=True,
        text=True,
    )
    if proc.returncode != 0:
        raise GitError(f"git clone {url} -> {dest} failed: {proc.stderr.strip()}")
    git(dest, "checkout", "-q", "--detach", pin)
    return dest


def replay_points(repo: pathlib.Path, ref: str, n: int) -> list[ReplayPoint]:
    """The last `n` commits ending at `ref`, oldest first, paired with parents.

    Merge commits are skipped: a merge's parent is ambiguous, and spec §4 already
    escalates merges to T1. The root commit is skipped too — there is no parent
    to build its replay worktree from.
    """
    out = git(repo, "rev-list", "--no-merges", f"--max-count={n}", ref)
    shas = [s for s in out.splitlines() if s]
    shas.reverse()
    points: list[ReplayPoint] = []
    for sha in shas:
        fields = git(repo, "rev-list", "--parents", "-n", "1", sha).split()
        if len(fields) < 2:
            continue  # root commit
        points.append(ReplayPoint(commit=sha, parent=fields[1]))
    return points


def paired_points(
    repo: pathlib.Path, ref: str, n: int, test_globs: Sequence[str], scan: int = 500
) -> list[ReplayPoint]:
    """The last `n` commits ending at `ref` that touch BOTH a test file and a non-test
    file, oldest first, paired with parents.

    This exists because the published corpus can measure cost and cannot measure
    recall. `natural` writes the child's whole diff over the parent, which reproduces
    the child commit — and real projects push green, so `natural` detects nothing in
    any repo, ever. `probe` is the population that can detect: the child's TESTS
    against the parent's SOURCE, so a test the commit's code change fixed fails, and
    that failure is what recall is scored against.

    A commit can only do that if it changed both halves. Replaying the most recent N
    commits spends most of them on docs, chores and refactors — flask yielded 3
    detecting commits out of 23, httpie and sqlfluff none at all — which is why every
    published recall figure reads `n/a (0/0)`.

    This is corpus SELECTION, not fault injection. Every commit returned is a real
    commit whose real tests really failed against its parent's real source; nothing is
    synthesised, mutated or injected. Audit finding A8 rejected a mutation harness for
    reasons that all still hold, and none of them apply here: there is no mutant, the
    failure modes are whatever the project's own history contains, and the cost is one
    `git log` rather than 455 CPU-hours.

    Selecting for the property does bias the population, and the bias must be
    published with the number: these are commits that changed code and tests together,
    which is not the average commit. What it does NOT do is bias toward RTDD — the
    filter reads the diff's paths and never asks what any strategy would select.

    `scan` bounds the history walk; merges and the root commit are skipped for the
    same reasons :func:`replay_points` skips them.
    """
    out = git(repo, "log", "--no-merges", f"--max-count={scan}", "--format=%x00%H", "--name-only", ref)
    points: list[ReplayPoint] = []
    for block in out.split("\0"):
        lines = [ln for ln in block.splitlines() if ln.strip()]
        if not lines:
            continue
        sha, paths = lines[0], lines[1:]
        if not paths:
            continue
        has_test = any(is_test_path(p, test_globs) for p in paths)
        has_source = any(not is_test_path(p, test_globs) for p in paths)
        if not (has_test and has_source):
            continue
        fields = git(repo, "rev-list", "--parents", "-n", "1", sha).split()
        if len(fields) < 2:
            continue  # root commit
        points.append(ReplayPoint(commit=sha, parent=fields[1]))
        if len(points) >= n:
            break
    points.reverse()  # oldest first, as replay_points returns
    return points


def diff_changes(repo: pathlib.Path, base: str, head: str) -> list[Change]:
    """`git diff --no-renames --name-status base head`, parsed.

    `--no-renames` is mandatory: RTDD must see a rename as delete + add.
    """
    out = git(repo, "diff", "--no-renames", "--name-status", "-z", base, head)
    fields = [f for f in out.split("\0") if f]
    changes: list[Change] = []
    for i in range(0, len(fields) - 1, 2):
        status = fields[i][0]
        path = fields[i + 1].replace(os.sep, "/")
        if status not in ("A", "M", "D"):
            status = "M"
        changes.append(Change(path=path, status=status))
    return changes


def add_worktree(repo: pathlib.Path, sha: str, dest: pathlib.Path) -> pathlib.Path:
    """A detached worktree at `sha`. The source clone's HEAD is never moved."""
    dest.parent.mkdir(parents=True, exist_ok=True)
    git(repo, "worktree", "add", "--detach", "--force", str(dest), sha)
    return dest


def remove_worktree(repo: pathlib.Path, dest: pathlib.Path) -> None:
    """Remove a worktree and its registration, leaving the source clone clean."""
    git(repo, "worktree", "remove", "--force", str(dest), check=False)
    if dest.exists():
        shutil.rmtree(dest, ignore_errors=True)
    git(repo, "worktree", "prune", check=False)


def _glob_variants(pattern: str) -> tuple[str, ...]:
    """`fnmatch` has no `**`, and its `*` already crosses `/`.

    So a `**/`-anchored pattern is matched both as written and with the `**/`
    removed; the caller matches the stripped form against every path suffix,
    which is what `**/` means.
    """
    return tuple({pattern, pattern.replace("**/", ""), pattern.replace("/**/", "/")})


def is_test_path(rel: str, test_globs: Sequence[str]) -> bool:
    """Does this repo-relative path name a test file?"""
    rel = rel.replace(os.sep, "/")
    parts = rel.split("/")
    suffixes = ["/".join(parts[i:]) for i in range(len(parts))]
    for pattern in test_globs:
        for variant in _glob_variants(pattern):
            if any(fnmatch.fnmatch(s, variant) for s in suffixes):
                return True
    return False


def probe_candidates(
    repo: pathlib.Path, points: Sequence[ReplayPoint], test_globs: Sequence[str]
) -> list[ReplayPoint]:
    """The `probe` population: commits touching a source file *and* a test file.

    A commit that touches only source has no test half to keep; one that touches
    only tests has no source half to revert. Neither produces a probe tree.
    """
    candidates: list[ReplayPoint] = []
    for point in points:
        changes = diff_changes(repo, point.parent, point.commit)
        kinds = {is_test_path(c.path, test_globs) for c in changes}
        if {True, False} <= kinds:
            candidates.append(point)
    return candidates


def _write_blob(work: pathlib.Path, repo: pathlib.Path, sha: str, rel: str) -> None:
    proc = subprocess.run(
        ["git", "show", f"{sha}:{rel}"],
        cwd=repo,
        env={**os.environ, **_ENV},
        capture_output=True,
    )
    if proc.returncode != 0:
        raise GitError(
            f"git show {sha}:{rel} failed in {repo}: {proc.stderr.decode().strip()}"
        )
    target = work / rel
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_bytes(proc.stdout)


def _remove(work: pathlib.Path, rel: str) -> None:
    target = work / rel
    if target.exists() or target.is_symlink():
        target.unlink()


def materialise_natural(
    work: pathlib.Path, repo: pathlib.Path, point: ReplayPoint
) -> list[Change]:
    """Worktree is at `point.parent`; write `point.commit` over it, HEAD unmoved."""
    changes = diff_changes(repo, point.parent, point.commit)
    for ch in changes:
        if ch.status == "D":
            _remove(work, ch.path)
        else:
            _write_blob(work, repo, point.commit, ch.path)
    return changes


def materialise_probe(
    work: pathlib.Path,
    repo: pathlib.Path,
    point: ReplayPoint,
    test_globs: Sequence[str],
) -> list[Change]:
    """Worktree is at `point.commit`; revert only the source half back to the parent.

    The returned changes are the *reversed* ones — what the working tree now
    differs by relative to `point.commit` — so an addition reverts to a deletion.
    """
    reverted: list[Change] = []
    for ch in diff_changes(repo, point.parent, point.commit):
        if is_test_path(ch.path, test_globs):
            continue
        if ch.status == "A":
            _remove(work, ch.path)  # did not exist at the parent
            reverted.append(Change(path=ch.path, status="D"))
        elif ch.status == "D":
            _write_blob(work, repo, point.parent, ch.path)
            reverted.append(Change(path=ch.path, status="A"))
        else:
            _write_blob(work, repo, point.parent, ch.path)
            reverted.append(Change(path=ch.path, status="M"))
    return reverted


def working_changed_paths(work: pathlib.Path) -> list[Change]:
    """Spec §5's changed set: `git status --porcelain -uall` over the worktree."""
    out = git(work, "status", "--porcelain=1", "-uall", "-z")
    fields = out.split("\0")
    changes: list[Change] = []
    i = 0
    while i < len(fields):
        entry = fields[i]
        i += 1
        if not entry:
            continue
        code, path = entry[:2], entry[3:]
        if code[0] in ("R", "C"):
            i += 1  # -z emits the source path of a rename/copy as its own field
        if "D" in code:
            status = "D"
        elif "?" in code or "A" in code:
            status = "A"
        else:
            status = "M"
        changes.append(Change(path=path.replace(os.sep, "/"), status=status))
    return changes
