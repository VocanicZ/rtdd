import json
import re
import stat
import subprocess
from pathlib import Path

import pytest

from providers import ARMS, UnknownArm, context_tools
from providers import tdad as tdad_provider
from providers.tdad import (
    STATIC_MAP,
    TDAD_BIN_ENV,
    TDAD_PIN,
    UV_BIN_ENV,
    TdadUnavailable,
    ensure_installed,
    impact_tool,
    index,
)
from tools import MAX_OUTPUT_BYTES
from workspace import Workspace

# The imperative lint belongs to the RTDD provider's test module, which owns the
# copy of the pre-registered blocklist until the arm-composition task lands. Arm
# C's context text is held to the same bar as arm D's, from the same list.
from test_providers_rtdd import lint_context  # noqa: E402

REPO_ROOT = Path(__file__).resolve().parents[3]

ROW = {
    "instance_id": "astropy__astropy-12907",
    "repo": "astropy/astropy",
    "base_commit": "d16bfe05a744909de4b27f5875fe0d4ed41ce607",
}

IMPACT_TABLE = """## Impacted Tests (1 found)

| Score | Test | File | Reason |
|-------|------|------|--------|
| 0.88 | test_add | tests/test_app.py | Directly tests changed code |"""


def script(path: Path, body: str) -> Path:
    path.write_text("#!/usr/bin/env bash\n" + body, encoding="utf-8")
    path.chmod(path.stat().st_mode | stat.S_IXUSR)
    return path


def calls(log: Path) -> list[str]:
    return log.read_text(encoding="utf-8").splitlines() if log.exists() else []


@pytest.fixture
def ws(tmp_path) -> Workspace:
    root = tmp_path / "repo"
    root.mkdir()
    return Workspace(
        root=root,
        instance_id=ROW["instance_id"],
        repo=ROW["repo"],
        base_commit=ROW["base_commit"],
    )


@pytest.fixture
def upstream(tmp_path) -> tuple[str, str]:
    """A local stand-in for github.com/pepealonso95/TDAD: ``(url, pinned sha)``.

    Real git, so the clone, the checkout and the pin resolution are exercised;
    local, so no test in this suite reaches the network.
    """
    origin = tmp_path / "upstream"
    (origin / "tdad").mkdir(parents=True)
    (origin / "tdad" / "pyproject.toml").write_text("[project]\nname='tdad'\n", encoding="utf-8")
    run = lambda *a: subprocess.run(  # noqa: E731
        ["git", "-C", str(origin), *a], capture_output=True, text=True, check=True
    )
    subprocess.run(["git", "init", "--quiet", str(origin)], check=True)
    run("config", "user.email", "bench@rtdd.local")
    run("config", "user.name", "rtdd-bench")
    run("add", "-A")
    run("commit", "--quiet", "-m", "pinned")
    sha = run("rev-parse", "HEAD").stdout.strip()
    return (str(origin), sha)


@pytest.fixture
def uv_log(tmp_path) -> Path:
    return tmp_path / "uv-calls.txt"


def fake_uv(tmp_path, monkeypatch, log: Path, *, install_rc: int = 0, help_rc: int = 0) -> Path:
    """A stand-in for ``uv`` that materialises a venv holding a stub ``tdad``."""
    body = (
        f'echo "$@" >> {log}\n'
        'if [ "$1" = "venv" ]; then\n'
        '  mkdir -p "$2/bin"\n'
        "  {\n"
        "    echo '#!/usr/bin/env bash'\n"
        f"    echo 'echo \"$@\" >> {log}'\n"
        f"    echo 'exit {help_rc}'\n"
        '  } > "$2/bin/tdad"\n'
        '  chmod +x "$2/bin/tdad"\n'
        "fi\n"
        f'if [ "$1" = "pip" ]; then exit {install_rc}; fi\n'
        "exit 0\n"
    )
    binary = script(tmp_path / "uv-stub", body)
    monkeypatch.setenv(UV_BIN_ENV, str(binary))
    return binary


def fake_tdad(tmp_path, monkeypatch, body: str, name: str = "tdad-stub") -> Path:
    binary = script(tmp_path / name, body)
    monkeypatch.setenv(TDAD_BIN_ENV, str(binary))
    return binary


# --- the pin ----------------------------------------------------------------


def test_the_pin_is_a_resolved_commit_sha_not_a_branch_name():
    assert re.fullmatch(r"[0-9a-f]{40}", TDAD_PIN)


def test_the_pin_is_recorded_in_the_published_config():
    config = json.loads(
        (REPO_ROOT / "bench" / "results" / "swebench" / "config.json").read_text(encoding="utf-8")
    )
    assert config["tdad_pin"] == TDAD_PIN


def test_the_pin_is_recorded_in_the_preregistration():
    text = (REPO_ROOT / "bench" / "PREREGISTRATION.md").read_text(encoding="utf-8")
    assert "## Reference implementations" in text
    heading = text.split("## Reference implementations", 1)[1]
    assert TDAD_PIN in heading
    # The RTDD commit under test sits beside it, so a reader can see both halves
    # of the comparison were pinned rather than only the outside one.
    assert "rtdd" in heading.lower()


def test_tdad_unavailable_is_a_runtime_error():
    assert issubclass(TdadUnavailable, RuntimeError)


# --- ensure_installed -------------------------------------------------------


def test_ensure_installed_clones_the_pin_and_returns_the_resolved_sha(
    tmp_path, monkeypatch, upstream, uv_log
):
    url, sha = upstream
    fake_uv(tmp_path, monkeypatch, uv_log)
    cache = tmp_path / "cache"

    assert ensure_installed(cache, repo=url, pin=sha) == sha

    checkout = cache / "tdad" / "src"
    assert (checkout / "tdad" / "pyproject.toml").is_file()
    head = subprocess.run(
        ["git", "-C", str(checkout), "rev-parse", "HEAD"], capture_output=True, text=True
    ).stdout.strip()
    assert head == sha
    assert any(c.startswith("pip install") for c in calls(uv_log))


def test_a_second_call_is_a_no_op_that_returns_the_same_sha(
    tmp_path, monkeypatch, upstream, uv_log
):
    url, sha = upstream
    fake_uv(tmp_path, monkeypatch, uv_log)
    cache = tmp_path / "cache"

    first = ensure_installed(cache, repo=url, pin=sha)
    before = list(calls(uv_log))
    second = ensure_installed(cache, repo=url, pin=sha)

    assert second == first == sha
    assert calls(uv_log) == before


def test_tdads_own_code_is_not_edited(tmp_path, monkeypatch, upstream, uv_log):
    url, sha = upstream
    fake_uv(tmp_path, monkeypatch, uv_log)
    cache = tmp_path / "cache"

    ensure_installed(cache, repo=url, pin=sha)

    dirty = subprocess.run(
        ["git", "-C", str(cache / "tdad" / "src"), "status", "--porcelain"],
        capture_output=True,
        text=True,
    ).stdout
    assert dirty == ""


def test_a_pin_that_is_not_in_the_repository_is_unavailable(
    tmp_path, monkeypatch, upstream, uv_log
):
    url, _ = upstream
    fake_uv(tmp_path, monkeypatch, uv_log)

    # Named as a rotted pin rather than as whatever git said about a bad ref: the
    # contingency's first question is whether the pin moved or the machine broke.
    with pytest.raises(TdadUnavailable, match="rotted"):
        ensure_installed(tmp_path / "cache", repo=url, pin="0" * 40)


def test_an_unclonable_repository_is_unavailable(tmp_path, monkeypatch, upstream, uv_log):
    _, sha = upstream
    fake_uv(tmp_path, monkeypatch, uv_log)

    with pytest.raises(TdadUnavailable):
        ensure_installed(tmp_path / "cache", repo=str(tmp_path / "no-such-repo"), pin=sha)


def test_a_failed_install_is_unavailable(tmp_path, monkeypatch, upstream, uv_log):
    url, sha = upstream
    fake_uv(tmp_path, monkeypatch, uv_log, install_rc=1)

    with pytest.raises(TdadUnavailable, match="install"):
        ensure_installed(tmp_path / "cache", repo=url, pin=sha)


def test_a_cli_that_will_not_run_is_unavailable(tmp_path, monkeypatch, upstream, uv_log):
    url, sha = upstream
    fake_uv(tmp_path, monkeypatch, uv_log, help_rc=1)

    with pytest.raises(TdadUnavailable):
        ensure_installed(tmp_path / "cache", repo=url, pin=sha)


def test_a_failed_install_is_not_cached_as_a_success(tmp_path, monkeypatch, upstream, uv_log):
    url, sha = upstream
    cache = tmp_path / "cache"
    fake_uv(tmp_path, monkeypatch, uv_log, install_rc=1)
    with pytest.raises(TdadUnavailable):
        ensure_installed(cache, repo=url, pin=sha)

    fake_uv(tmp_path, monkeypatch, uv_log)
    assert ensure_installed(cache, repo=url, pin=sha) == sha


# --- index ------------------------------------------------------------------


def indexer(tmp_path, monkeypatch, *, rc: int = 0, body: str = "") -> Path:
    log = tmp_path / "tdad-index-calls.txt"
    fake_tdad(
        tmp_path,
        monkeypatch,
        f'echo "$@" >> {log}\n{body}exit {rc}\n',
    )
    return log


def test_index_builds_the_static_map_and_returns_its_path(tmp_path, monkeypatch, ws):
    (ws.root / ".tdad").mkdir()
    log = indexer(
        tmp_path,
        monkeypatch,
        body=f"echo 'src/app.py: tests/test_app.py' > {ws.root / STATIC_MAP}\n",
    )

    out = index(ws)

    assert out == ws.root / STATIC_MAP
    assert out.read_text(encoding="utf-8").strip() == "src/app.py: tests/test_app.py"
    assert calls(log) == [f"index {ws.root}"]


def test_a_failed_index_is_unavailable(tmp_path, monkeypatch, ws):
    indexer(tmp_path, monkeypatch, rc=1, body='echo "boom" >&2\n')

    with pytest.raises(TdadUnavailable, match=ws.instance_id):
        index(ws)


def test_an_index_that_produced_no_map_is_unavailable(tmp_path, monkeypatch, ws):
    indexer(tmp_path, monkeypatch)

    with pytest.raises(TdadUnavailable):
        index(ws)


def test_an_empty_map_is_unavailable(tmp_path, monkeypatch, ws):
    (ws.root / ".tdad").mkdir()
    indexer(tmp_path, monkeypatch, body=f"printf '' > {ws.root / STATIC_MAP}\n")

    with pytest.raises(TdadUnavailable):
        index(ws)


def test_a_missing_tdad_cli_is_unavailable(tmp_path, monkeypatch, ws):
    monkeypatch.setenv(TDAD_BIN_ENV, str(tmp_path / "definitely-not-here"))

    with pytest.raises(TdadUnavailable):
        index(ws)


# --- impact_tool ------------------------------------------------------------


def test_impact_tool_is_the_test_impact_context_tool(tmp_path, monkeypatch, ws):
    fake_tdad(tmp_path, monkeypatch, "exit 0\n")
    tool = impact_tool(ws)

    assert tool.name == "test_impact"
    fn = tool.schema["function"]
    assert fn["name"] == "test_impact"
    assert fn["parameters"]["properties"]["files"] == {
        "type": "array",
        "items": {"type": "string"},
    }
    assert fn["parameters"]["required"] == ["files"]
    assert fn["parameters"]["additionalProperties"] is False
    json.dumps(tool.schema)


def test_impact_tool_reports_the_impacted_tests(tmp_path, monkeypatch, ws):
    fake_tdad(tmp_path, monkeypatch, f"cat <<'EOF'\n{IMPACT_TABLE}\nEOF\n")

    out = impact_tool(ws).call({"files": ["src/app.py"]})

    assert "tests/test_app.py" in out
    assert "test_add" in out


def test_impact_tool_passes_the_workspace_and_the_changed_files(tmp_path, monkeypatch, ws):
    log = tmp_path / "impact-args.txt"
    fake_tdad(tmp_path, monkeypatch, f'echo "$@" > {log}\necho "none"\n')

    impact_tool(ws).call({"files": ["src/app.py", "src/other.py"]})

    argv = log.read_text(encoding="utf-8").split()
    assert argv[0] == "impact"
    assert argv[1] == str(ws.root)
    assert argv[argv.index("--files") + 1 :] == ["src/app.py", "src/other.py"]


def test_no_files_is_answered_rather_than_run(tmp_path, monkeypatch, ws):
    log = tmp_path / "impact-args.txt"
    fake_tdad(tmp_path, monkeypatch, f'echo "$@" >> {log}\n')

    assert "no files" in impact_tool(ws).call({}).lower()
    assert calls(log) == []


def test_a_flag_shaped_argument_is_refused(tmp_path, monkeypatch, ws):
    log = tmp_path / "impact-args.txt"
    fake_tdad(tmp_path, monkeypatch, f'echo "$@" >> {log}\n')

    out = impact_tool(ws).call({"files": ["--max-tests"]})

    assert "--max-tests" in out
    assert calls(log) == []


def test_an_unavailable_tdad_is_tool_output_not_an_exception(tmp_path, monkeypatch, ws):
    fake_tdad(tmp_path, monkeypatch, 'echo "graph is gone" >&2\nexit 1\n')

    out = impact_tool(ws).call({"files": ["src/app.py"]})

    assert "test_impact unavailable" in out
    assert "graph is gone" in out


def test_the_tool_output_is_clipped(tmp_path, monkeypatch, ws):
    fake_tdad(tmp_path, monkeypatch, "head -c 200000 /dev/zero | tr '\\0' 'x'\n")

    out = impact_tool(ws).call({"files": ["src/app.py"]})

    assert len(out) < MAX_OUTPUT_BYTES + 200
    assert "clipped" in out


def test_the_tool_text_carries_no_imperative(tmp_path, monkeypatch, ws):
    fake_tdad(tmp_path, monkeypatch, f"cat <<'EOF'\n{IMPACT_TABLE}\nEOF\n")
    tool = impact_tool(ws)

    assert lint_context(tool.schema["function"]["description"]) == []
    assert lint_context(tool.call({"files": ["src/app.py"]})) == []


# --- registry ---------------------------------------------------------------


def arm_c(tmp_path, monkeypatch, upstream, uv_log, ws, *, index_body: str, index_rc: int = 0):
    url, sha = upstream
    fake_uv(tmp_path, monkeypatch, uv_log)
    monkeypatch.setattr(tdad_provider, "TDAD_REPO", url)
    monkeypatch.setattr(tdad_provider, "TDAD_PIN", sha)
    fake_tdad(tmp_path, monkeypatch, f"{index_body}exit {index_rc}\n", name="tdad-arm-c")
    return sha


def test_the_tdad_arm_gets_the_impact_tool_and_records_the_pin(
    tmp_path, monkeypatch, upstream, uv_log, ws
):
    (ws.root / ".tdad").mkdir()
    sha = arm_c(
        tmp_path,
        monkeypatch,
        upstream,
        uv_log,
        ws,
        index_body=f"echo 'src/app.py: tests/test_app.py' > {ws.root / STATIC_MAP}\n",
    )

    tools, provenance = context_tools("tdad", ws, tmp_path / "cache", ROW)

    assert [t.name for t in tools] == ["test_impact"]
    assert provenance == {"tdad_pin": sha}


def test_an_index_failure_is_raised_rather_than_silently_dropping_the_context(
    tmp_path, monkeypatch, upstream, uv_log, ws
):
    arm_c(tmp_path, monkeypatch, upstream, uv_log, ws, index_body="", index_rc=1)

    with pytest.raises(TdadUnavailable):
        context_tools("tdad", ws, tmp_path / "cache", ROW)


def test_an_install_failure_is_raised_rather_than_silently_dropping_the_context(
    tmp_path, monkeypatch, upstream, uv_log, ws
):
    url, sha = upstream
    fake_uv(tmp_path, monkeypatch, uv_log, install_rc=1)
    monkeypatch.setattr(tdad_provider, "TDAD_REPO", url)
    monkeypatch.setattr(tdad_provider, "TDAD_PIN", sha)

    with pytest.raises(TdadUnavailable):
        context_tools("tdad", ws, tmp_path / "cache", ROW)


def test_an_unknown_arm_is_rejected_rather_than_armed_with_nothing(tmp_path, ws):
    with pytest.raises(UnknownArm, match="rtdd_tdad"):
        context_tools("rtdd_tdad", ws, tmp_path / "cache", ROW)


def test_the_known_arms_are_the_preregistered_five():
    assert ARMS == ("vanilla", "tdd", "tdad", "rtdd", "rtdd_tdd")


# --- the contingency --------------------------------------------------------


def test_the_contingency_caveat_is_written_before_it_is_needed():
    caveat = (REPO_ROOT / "docs" / "results" / "tdad-caveat.md").read_text(encoding="utf-8")
    lowered = caveat.lower()
    # The two things a reader must not have to reconstruct: that the arm was not
    # run here, and that two deltas on two harnesses are not a ranking.
    assert "not" in lowered and "harness" in lowered
    assert "cited" in lowered or "quoted" in lowered
    assert "1.82" in caveat


def test_the_caveat_is_reachable_from_the_provider(tmp_path, monkeypatch, upstream, uv_log, ws):
    arm_c(tmp_path, monkeypatch, upstream, uv_log, ws, index_body="", index_rc=1)

    with pytest.raises(TdadUnavailable) as excinfo:
        context_tools("tdad", ws, tmp_path / "cache", ROW)

    assert "docs/results/tdad-caveat.md" in str(excinfo.value)
