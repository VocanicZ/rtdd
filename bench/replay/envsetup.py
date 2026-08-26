"""Provision one virtualenv per corpus repo, and put it in front of every subprocess.

A corpus repo's suite does not run in the harness's own interpreter. `sys.executable`
has `replay`'s dependencies in it and nothing else, so `pytest` there collects zero
tests in flask and every commit lands in `skipped` — a replay that "completes" while
measuring nothing. `corpus.yaml` already carries the recipe (`install:`) and the
interpreter version (`python:`) for exactly this; this module is the piece that
executes them.

Three properties make the result safe to publish from:

**Only `uv pip install` ever runs.** `install:` comes out of a YAML file, so an
entry that is not a `uv pip install` invocation is an :class:`EnvError`, not a
shell line. The corpus is a whitelist of repos, not a whitelist of commands.

**The recipe is the cache key.** :func:`recipe_digest` covers the interpreter
version, the install commands and the harness plugin set; the venv is rebuilt when
that digest changes and reused byte-for-byte when it does not, so a replay pays for
provisioning once and a changed recipe can never be silently served from a stale env.

**`bin` goes first on `PATH`.** `adapters/python.yaml` runs a bare `pytest`, so the
binary under test resolves its runner from `PATH`, not from `sys.executable`. If the
harness venv wins that lookup, `rtdd` and the baselines measure two different
interpreters and every comparison between them is void.
"""

from __future__ import annotations

import dataclasses
import hashlib
import json
import os
import pathlib
import shlex
import subprocess
from collections.abc import Callable, Mapping, Sequence

from replay.corpus import RepoSpec

#: The harness's own instruments. The corpus repo pins its test dependencies; these
#: are what `replay` needs present in the interpreter that actually runs the suite —
#: the report log it parses, the coverage it reads, and the two baselines that are
#: pytest plugins rather than harness code.
HARNESS_PLUGINS = (
    "pytest",
    "pytest-cov",
    "pytest-reportlog",
    "pytest-testmon",
    "pytest-xdist",
    "coverage",
)

STAMP = "rtdd-bench-env.json"

_ALLOWED = ("uv", "pip", "install")


class EnvError(RuntimeError):
    pass


@dataclasses.dataclass(frozen=True)
class RepoEnv:
    """A provisioned interpreter for one corpus repo."""

    repo_id: str
    root: pathlib.Path
    python: pathlib.Path
    digest: str


def _run(argv: Sequence[str], cwd: pathlib.Path, env: Mapping[str, str]) -> None:
    proc = subprocess.run(
        list(argv), cwd=str(cwd), env=dict(env), capture_output=True, text=True
    )
    if proc.returncode != 0:
        raise EnvError(
            f"{' '.join(argv)} exited {proc.returncode} in {cwd}: "
            f"{(proc.stderr or proc.stdout).strip()[-2000:]}"
        )


def recipe_digest(spec: RepoSpec, plugins: Sequence[str] = HARNESS_PLUGINS) -> str:
    """Everything that decides what lands in the venv, in one digest."""
    payload = {
        "python": spec.python,
        "install": list(spec.install),
        "plugins": sorted(plugins),
    }
    blob = json.dumps(payload, sort_keys=True, separators=(",", ":"))
    return hashlib.sha256(blob.encode("utf-8")).hexdigest()


def _check(command: str) -> list[str]:
    argv = shlex.split(command)
    if tuple(argv[:3]) != _ALLOWED:
        raise EnvError(
            f"corpus install command must be a `uv pip install ...` invocation, got {command!r}"
        )
    return argv


def activate(env: RepoEnv, base: Mapping[str, str] | None = None) -> dict[str, str]:
    """`base` with the venv in front — the environment every subprocess inherits.

    Returns a new mapping; the caller decides whether to hand it to `subprocess`
    or to push it into `os.environ` for the library layers that read it there.
    """
    out = dict(os.environ if base is None else base)
    bindir = str(env.python.parent)
    out["PATH"] = bindir + os.pathsep + out.get("PATH", "")
    out["VIRTUAL_ENV"] = str(env.root)
    # A stale PYTHONHOME aims the venv's interpreter at another stdlib.
    out.pop("PYTHONHOME", None)
    return out


def provision(
    spec: RepoSpec,
    repo: pathlib.Path,
    root: pathlib.Path,
    *,
    plugins: Sequence[str] = HARNESS_PLUGINS,
    runner: Callable[[Sequence[str], pathlib.Path, Mapping[str, str]], None] = _run,
    force: bool = False,
) -> RepoEnv:
    """Create (or reuse) `root/<repo_id>` as `spec`'s interpreter and return it."""
    digest = recipe_digest(spec, plugins)
    venv = root / spec.id
    python = venv / "bin" / "python"
    stamp = venv / STAMP
    if not force and stamp.exists():
        try:
            if json.loads(stamp.read_text(encoding="utf-8")).get("digest") == digest:
                return RepoEnv(spec.id, venv, python, digest)
        except (OSError, ValueError):
            pass  # an unreadable stamp is a rebuild, never a silent reuse

    commands = [_check(c) for c in spec.install]
    root.mkdir(parents=True, exist_ok=True)
    env = dict(os.environ)
    env.pop("PYTHONHOME", None)
    # `--clear`: a rebuild is triggered by a changed recipe, and reusing the old
    # site-packages would leave the superseded pins installed beside the new ones.
    runner(["uv", "venv", "--clear", "--python", spec.python, str(venv)], root, env)
    env["VIRTUAL_ENV"] = str(venv)
    # Plugins first, the corpus's own commands last: a repo replaying commits from
    # before a pytest major has to be able to pin `pytest<9` in `corpus.yaml` and
    # have that pin survive. Installing the harness's instruments afterwards would
    # silently upgrade the runner out from under the history being replayed.
    runner(["uv", "pip", "install", *plugins], repo, env)
    for argv in commands:
        runner(argv, repo, env)
    venv.mkdir(parents=True, exist_ok=True)
    stamp.write_text(
        json.dumps({"digest": digest, "python": spec.python}, sort_keys=True) + "\n",
        encoding="utf-8",
    )
    return RepoEnv(spec.id, venv, python, digest)


def tool_versions_for(python: pathlib.Path | str, names: Sequence[str]) -> tuple[tuple[str, str], ...]:
    """The tracked tool versions *in the interpreter that will run the suite*.

    `config.tool_versions()` reports the harness's own; a number published beside
    the wrong pytest version is a number published without its config.
    """
    code = (
        "import importlib.metadata as m, json, sys\n"
        "out = {}\n"
        "for n in sys.argv[1:]:\n"
        "    try: out[n] = m.version(n)\n"
        "    except Exception: out[n] = 'absent'\n"
        "print(json.dumps(out))\n"
    )
    proc = subprocess.run(
        [str(python), "-c", code, *names], capture_output=True, text=True
    )
    if proc.returncode != 0:
        raise EnvError(f"could not read tool versions from {python}: {proc.stderr.strip()[-500:]}")
    return tuple(sorted(json.loads(proc.stdout).items()))


def source_root(source_globs: Sequence[str]) -> str:
    """The directory that must be on `sys.path` for the repo's package to import.

    Read off the corpus's own `source_globs`: the literal segments before the
    first wildcard name the package directory, and its parent is the import root —
    `src/flask/**/*.py` → `src`, `httpie/**/*.py` → `.`.
    """
    for glob in source_globs:
        parts = pathlib.PurePosixPath(glob).parts
        literal = []
        for part in parts:
            if any(ch in part for ch in "*?["):
                break
            literal.append(part)
        if not literal:
            continue
        parent = literal[:-1]
        return "/".join(parent) if parent else "."
    return "."


def with_source_path(
    env: Mapping[str, str], work: pathlib.Path, source_globs: Sequence[str]
) -> dict[str, str]:
    """`env` with this worktree's source root in front of `PYTHONPATH`.

    `uv pip install -e .` pins the *clone*: flit and setuptools write a `.pth`
    naming the clone's source directory, and `.pth` entries are appended to
    `sys.path` after everything `PYTHONPATH` contributes. The replay's whole point
    is that the worktree holds the materialised commit, so the worktree's source
    has to win that lookup — otherwise every commit is scored against the clone's
    code, `F_full` comes out empty, and the table measures nothing.
    """
    out = dict(env)
    root = source_root(source_globs)
    entry = str(work if root == "." else pathlib.Path(work) / root)
    existing = out.get("PYTHONPATH", "")
    out["PYTHONPATH"] = entry + (os.pathsep + existing if existing else "")
    return out
