"""Parse and gate on bench/PREREGISTRATION.md.

Nothing in M4 may run unless this file is complete, SIGNED by a human, and
reachable from the `prereg-m4` git tag. The kill criterion in particular is
never defaulted: a missing, blank, or non-numeric value is a hard refusal.
"""

from __future__ import annotations

import re
import subprocess
from dataclasses import dataclass
from pathlib import Path

import yaml

REQUIRED: tuple[str, ...] = (
    "status",
    "sample_size",
    "sample_seed",
    "instance_list_sha256",
    "model",
    "arms",
    "stratified_recall_floor",
    "vanilla_equivalence_k",
    "signed_by",
    "signed_at",
)

NUMERIC: tuple[str, ...] = (
    "sample_size",
    "sample_seed",
    "stratified_recall_floor",
    "vanilla_equivalence_k",
)

_FRONT_MATTER = re.compile(r"\A---\n(.*?)\n---\n", re.S)


class PreregError(RuntimeError):
    """The pre-registration is absent, incomplete, unsigned, or untagged."""


@dataclass(frozen=True)
class Prereg:
    fields: dict
    path: Path


def load(path: Path) -> Prereg:
    if not path.exists():
        raise PreregError(f"{path}: pre-registration file does not exist")
    text = path.read_text(encoding="utf-8")
    match = _FRONT_MATTER.match(text)
    if match is None:
        raise PreregError(f"{path}: no YAML front-matter at the top of the file")
    try:
        fields = yaml.safe_load(match.group(1)) or {}
    except yaml.YAMLError as exc:
        raise PreregError(f"{path}: front-matter is not valid YAML: {exc}") from exc
    if not isinstance(fields, dict):
        raise PreregError(f"{path}: front-matter is not a mapping")

    blank = [k for k in REQUIRED if fields.get(k) in (None, "", [], {})]
    if blank:
        raise PreregError(f"{path}: missing or blank pre-registered fields: {sorted(blank)}")

    for key in NUMERIC:
        value = fields[key]
        if isinstance(value, bool) or not isinstance(value, (int, float)):
            raise PreregError(
                f"{path}: {key} must be numeric and chosen before the run, got {value!r}"
            )

    if fields["status"] != "SIGNED":
        raise PreregError(
            f"{path}: status is {fields['status']!r}, not SIGNED — a human must sign this"
        )
    return Prereg(fields=fields, path=path)


def _git(repo_root: Path, *args: str) -> str:
    proc = subprocess.run(
        ["git", "-C", str(repo_root), *args],
        capture_output=True,
        text=True,
    )
    if proc.returncode != 0:
        raise PreregError(f"git {' '.join(args)} failed: {proc.stderr.strip()}")
    return proc.stdout.strip()


def _tag_exists(repo_root: Path, tag: str) -> bool:
    """Whether ``tag`` is a ref here.

    Asked separately so an absent tag refuses in this module's own words. Left
    to ``rev-list``, the refusal would be git's "ambiguous argument" text, which
    names the tag but tells the reader nothing about what to do about it.
    """
    proc = subprocess.run(
        ["git", "-C", str(repo_root), "rev-parse", "--verify", "--quiet", f"refs/tags/{tag}"],
        capture_output=True,
        text=True,
    )
    return proc.returncode == 0


def assert_tagged(repo_root: Path, path: Path, tag: str = "prereg-m4") -> str:
    """Return the commit the tag points at, or refuse.

    The commit that last modified the pre-registration must be an ancestor of
    (or equal to) the tag, so the signed values cannot be edited after tagging
    without moving the tag — which is visible in the reflog.
    """
    if not _tag_exists(repo_root, tag):
        raise PreregError(
            f"the tag {tag} does not exist — the pre-registration has not been frozen; "
            f"a human signs {path.name} and then runs `git tag {tag}`"
        )
    tag_sha = _git(repo_root, "rev-list", "-n", "1", tag)
    rel = path.resolve().relative_to(repo_root.resolve()).as_posix()
    last = _git(repo_root, "log", "-n", "1", "--format=%H", "--", rel)
    if not last:
        raise PreregError(f"{rel} has never been committed")
    merge_base = _git(repo_root, "merge-base", tag_sha, last)
    if merge_base != last:
        raise PreregError(
            f"{rel} was modified in {last[:8]} which is not reachable from tag {tag} "
            f"({tag_sha[:8]}) — re-sign and re-tag before running"
        )
    return tag_sha
