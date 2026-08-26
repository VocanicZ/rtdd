import subprocess
from pathlib import Path

import pytest

from workspace import Workspace, diff, prepare, remote_url


def git(repo: Path, *args: str) -> str:
    proc = subprocess.run(
        ["git", "-C", str(repo), *args], check=True, capture_output=True, text=True
    )
    return proc.stdout.strip()


def make_upstream(root: Path, repo: str) -> Path:
    """A local stand-in for github.com/<repo>, with two commits."""
    path = root / repo
    path.mkdir(parents=True)
    subprocess.run(["git", "init", "-q", "-b", "main", str(path)], check=True)
    git(path, "config", "user.email", "up@t")
    git(path, "config", "user.name", "up")
    (path / "mod.py").write_text("def f():\n    return 1\n", encoding="utf-8")
    git(path, "add", "-A")
    git(path, "commit", "-qm", "base")
    base = git(path, "rev-parse", "HEAD")
    (path / "mod.py").write_text("def f():\n    return 2\n", encoding="utf-8")
    git(path, "commit", "-qam", "later")
    return base


@pytest.fixture
def upstream(tmp_path, monkeypatch):
    root = tmp_path / "upstream"
    monkeypatch.setenv("RTDD_BENCH_GIT_URL_TEMPLATE", f"file://{root}/{{repo}}")
    return root


def row(instance_id: str, repo: str, base_commit: str) -> dict:
    return {"instance_id": instance_id, "repo": repo, "base_commit": base_commit}


def test_prepare_checks_out_the_base_commit(tmp_path, upstream):
    base = make_upstream(upstream, "acme/widget")
    ws = prepare(tmp_path / "mirrors", tmp_path / "work", row("acme__widget-1", "acme/widget", base))

    assert isinstance(ws, Workspace)
    assert ws.instance_id == "acme__widget-1"
    assert ws.repo == "acme/widget"
    assert ws.base_commit == base
    assert git(ws.root, "rev-parse", "HEAD") == base
    assert (ws.root / "mod.py").read_text(encoding="utf-8") == "def f():\n    return 1\n"


def test_prepare_creates_a_worktree_of_the_shared_mirror(tmp_path, upstream):
    base = make_upstream(upstream, "acme/widget")
    ws = prepare(tmp_path / "mirrors", tmp_path / "work", row("acme__widget-1", "acme/widget", base))

    # A worktree links to the mirror through a .git *file*, not its own object store.
    assert (ws.root / ".git").is_file()


def test_second_instance_from_the_same_repo_reuses_the_mirror(tmp_path, upstream):
    base = make_upstream(upstream, "acme/widget")
    cache, work = tmp_path / "mirrors", tmp_path / "work"
    first = prepare(cache, work, row("acme__widget-1", "acme/widget", base))

    mirror = next(p for p in cache.iterdir() if p.is_dir())
    marker = mirror / "RTDD_MIRROR_MARKER"
    marker.write_text("first clone", encoding="utf-8")

    second = prepare(cache, work, row("acme__widget-2", "acme/widget", base))

    assert marker.exists(), "the mirror was re-cloned instead of reused"
    assert len([p for p in cache.iterdir() if p.is_dir()]) == 1
    assert second.root != first.root
    assert git(second.root, "rev-parse", "HEAD") == base


def test_prepare_fetches_when_the_base_commit_is_not_yet_mirrored(tmp_path, upstream):
    base = make_upstream(upstream, "acme/widget")
    cache, work = tmp_path / "mirrors", tmp_path / "work"
    prepare(cache, work, row("acme__widget-1", "acme/widget", base))

    origin = upstream / "acme/widget"
    (origin / "new.py").write_text("x = 1\n", encoding="utf-8")
    git(origin, "add", "-A")
    git(origin, "commit", "-qm", "newer")
    newer = git(origin, "rev-parse", "HEAD")

    ws = prepare(cache, work, row("acme__widget-2", "acme/widget", newer))
    assert git(ws.root, "rev-parse", "HEAD") == newer


def test_prepare_is_idempotent_for_a_repeated_instance(tmp_path, upstream):
    base = make_upstream(upstream, "acme/widget")
    cache, work = tmp_path / "mirrors", tmp_path / "work"
    r = row("acme__widget-1", "acme/widget", base)
    first = prepare(cache, work, r)
    (first.root / "scratch.py").write_text("junk\n", encoding="utf-8")

    second = prepare(cache, work, r)

    assert second.root == first.root
    assert not (second.root / "scratch.py").exists(), "a stale workspace leaked into the rerun"
    assert diff(second) == ""


def test_two_repos_get_two_mirrors(tmp_path, upstream):
    a = make_upstream(upstream, "acme/widget")
    b = make_upstream(upstream, "other/thing")
    cache, work = tmp_path / "mirrors", tmp_path / "work"
    prepare(cache, work, row("acme__widget-1", "acme/widget", a))
    prepare(cache, work, row("other__thing-1", "other/thing", b))
    assert len([p for p in cache.iterdir() if p.is_dir()]) == 2


def test_diff_is_empty_when_nothing_changed(tmp_path, upstream):
    base = make_upstream(upstream, "acme/widget")
    ws = prepare(tmp_path / "mirrors", tmp_path / "work", row("acme__widget-1", "acme/widget", base))
    assert diff(ws) == ""


def test_diff_returns_the_working_tree_patch(tmp_path, upstream):
    base = make_upstream(upstream, "acme/widget")
    ws = prepare(tmp_path / "mirrors", tmp_path / "work", row("acme__widget-1", "acme/widget", base))
    (ws.root / "mod.py").write_text("def f():\n    return 42\n", encoding="utf-8")

    patch = diff(ws)

    assert "--- a/mod.py" in patch
    assert "+++ b/mod.py" in patch
    assert "+    return 42" in patch


def test_diff_includes_files_the_agent_created(tmp_path, upstream):
    base = make_upstream(upstream, "acme/widget")
    ws = prepare(tmp_path / "mirrors", tmp_path / "work", row("acme__widget-1", "acme/widget", base))
    (ws.root / "test_new.py").write_text("def test_x():\n    assert True\n", encoding="utf-8")

    patch = diff(ws)

    assert "test_new.py" in patch
    assert "+def test_x():" in patch


def test_remote_url_defaults_to_github(monkeypatch):
    monkeypatch.delenv("RTDD_BENCH_GIT_URL_TEMPLATE", raising=False)
    assert remote_url("acme/widget") == "https://github.com/acme/widget.git"
