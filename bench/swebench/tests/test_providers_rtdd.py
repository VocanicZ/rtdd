import json
import os
import stat
import subprocess
import sys
from pathlib import Path

import pytest

from providers import CONTEXT_TOOL_ARMS, context_tools
from providers import rtdd as rtdd_provider
from providers.rtdd import (
    DOCKER_BIN_ENV,
    RTDD_BIN_ENV,
    SEED_FAILED,
    SEED_TIMEOUT,
    SEEDED,
    SeedFailed,
    ensure_seed,
    install_seed,
    seed_cache_key,
    which_tool,
)
from tools import MAX_OUTPUT_BYTES
from workspace import Workspace

# The imperative lint is owned by the arm-composition task. Bind to it when it is
# present so the two lists cannot drift; fall back to a copy of the pre-registered
# blocklist until then, so this file's guarantee holds either way.
try:  # pragma: no cover - exercised by whichever half of the branch is live
    from prompts import BANNED_IMPERATIVES, lint_context
except ModuleNotFoundError:  # pragma: no cover
    BANNED_IMPERATIVES = (
        "you must",
        "you should",
        "make sure",
        "be sure to",
        "first write",
        "before you",
        "before changing",
        "before committing",
        "step 1",
        "step 2",
        "then run",
        "always run",
        "do not skip",
        "follow a",
        "follow these",
        "workflow",
        "red-green",
        "test-driven",
        "procedure",
        "your job is to",
        "start by",
    )

    def lint_context(text: str) -> list[str]:
        lowered = text.lower()
        return [phrase for phrase in BANNED_IMPERATIVES if phrase in lowered]


ROW = {
    "instance_id": "astropy__astropy-12907",
    "repo": "astropy/astropy",
    "base_commit": "d16bfe05a744909de4b27f5875fe0d4ed41ce607",
}

MAP_LINE = json.dumps({"t": "tests/test_a.py::test_x", "f": ["src/app.py"], "c": "abc123"})


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


def script(path: Path, body: str) -> Path:
    path.write_text("#!/usr/bin/env bash\n" + body, encoding="utf-8")
    path.chmod(path.stat().st_mode | stat.S_IXUSR)
    return path


@pytest.fixture
def rtdd_bin(tmp_path, monkeypatch) -> Path:
    """A stand-in for the rtdd binary the seed run mounts into the image."""
    binary = script(tmp_path / "rtdd-stub", 'echo "stub rtdd"\n')
    monkeypatch.setenv(RTDD_BIN_ENV, str(binary))
    return binary


@pytest.fixture
def docker_log(tmp_path) -> Path:
    return tmp_path / "docker-calls.txt"


def fake_docker(tmp_path, monkeypatch, log: Path, body: str) -> Path:
    binary = script(
        tmp_path / "docker-stub",
        f'echo "$@" >> {log}\n{body}',
    )
    monkeypatch.setenv(DOCKER_BIN_ENV, str(binary))
    return binary


def calls(log: Path) -> list[str]:
    return log.read_text(encoding="utf-8").splitlines() if log.exists() else []


# --- seed_cache_key ---------------------------------------------------------


def test_seed_cache_key_is_stable_across_calls():
    assert seed_cache_key(ROW) == seed_cache_key(dict(ROW))


def test_two_instances_sharing_a_base_commit_share_one_seed():
    other = dict(ROW, instance_id="astropy__astropy-99999")
    assert seed_cache_key(other) == seed_cache_key(ROW)


def test_a_different_base_commit_is_a_different_seed():
    assert seed_cache_key(dict(ROW, base_commit="0" * 40)) != seed_cache_key(ROW)


def test_a_different_repo_is_a_different_seed():
    assert seed_cache_key(dict(ROW, repo="django/django")) != seed_cache_key(ROW)


def test_the_key_is_content_addressed_not_name_addressed():
    key = seed_cache_key(ROW)
    assert "astropy" not in key
    assert ROW["base_commit"] not in key
    assert ROW["instance_id"] not in key
    assert all(c in "0123456789abcdef" for c in key)


# --- ensure_seed ------------------------------------------------------------


def test_ensure_seed_runs_the_suite_in_the_instance_image_and_caches_the_map(
    tmp_path, monkeypatch, ws, rtdd_bin, docker_log
):
    fake_docker(tmp_path, monkeypatch, docker_log, f"echo '{MAP_LINE}'\n")
    cache = tmp_path / "cache"

    seed, status = ensure_seed(cache, ws, ROW)

    assert status == SEEDED
    assert seed is not None and seed.read_text(encoding="utf-8").strip() == MAP_LINE
    assert seed.parent == cache / "seeds" / seed_cache_key(ROW)
    argv = calls(docker_log)[0]
    assert "swebench/sweb.eval.x86_64.astropy_1776_astropy-12907:latest" in argv
    assert f"{rtdd_bin}:/usr/local/bin/rtdd:ro" in argv


def test_a_cached_seed_is_reused_without_re_running_the_suite(
    tmp_path, monkeypatch, ws, rtdd_bin, docker_log
):
    fake_docker(tmp_path, monkeypatch, docker_log, f"echo '{MAP_LINE}'\n")
    cache = tmp_path / "cache"

    first, _ = ensure_seed(cache, ws, ROW)
    second, status = ensure_seed(cache, ws, ROW)

    assert (second, status) == (first, SEEDED)
    assert len(calls(docker_log)) == 1


def test_the_second_arm_shares_the_first_arms_seed(tmp_path, monkeypatch, ws, rtdd_bin, docker_log):
    fake_docker(tmp_path, monkeypatch, docker_log, f"echo '{MAP_LINE}'\n")
    cache = tmp_path / "cache"
    sibling = Workspace(
        root=ws.root, instance_id=ROW["instance_id"], repo=ROW["repo"], base_commit=ROW["base_commit"]
    )

    ensure_seed(cache, ws, ROW)
    _, status = ensure_seed(cache, sibling, dict(ROW, instance_id="astropy__astropy-99999"))

    assert status == SEEDED
    assert len(calls(docker_log)) == 1


def test_a_failed_seed_is_recorded_and_the_instance_is_not_dropped(
    tmp_path, monkeypatch, ws, rtdd_bin, docker_log
):
    fake_docker(tmp_path, monkeypatch, docker_log, 'echo "boom" >&2\nexit 1\n')
    cache = tmp_path / "cache"

    seed, status = ensure_seed(cache, ws, ROW)

    assert (seed, status) == (None, SEED_FAILED)
    slot = cache / "seeds" / seed_cache_key(ROW)
    assert json.loads((slot / "status.json").read_text(encoding="utf-8"))["status"] == SEED_FAILED
    assert "boom" in (slot / "stderr.txt").read_text(encoding="utf-8")


def test_an_empty_map_from_a_zero_exit_run_is_a_seed_failure(
    tmp_path, monkeypatch, ws, rtdd_bin, docker_log
):
    fake_docker(tmp_path, monkeypatch, docker_log, "exit 0\n")
    seed, status = ensure_seed(tmp_path / "cache", ws, ROW)
    assert (seed, status) == (None, SEED_FAILED)


def test_a_timed_out_seed_returns_a_reason_rather_than_raising(
    tmp_path, monkeypatch, ws, rtdd_bin, docker_log
):
    fake_docker(tmp_path, monkeypatch, docker_log, "sleep 5\n")
    cache = tmp_path / "cache"

    seed, status = ensure_seed(cache, ws, ROW, timeout_s=1)

    assert (seed, status) == (None, SEED_TIMEOUT)
    marker = cache / "seeds" / seed_cache_key(ROW) / "status.json"
    assert json.loads(marker.read_text(encoding="utf-8"))["status"] == SEED_TIMEOUT


def test_a_cached_failure_is_not_retried(tmp_path, monkeypatch, ws, rtdd_bin, docker_log):
    fake_docker(tmp_path, monkeypatch, docker_log, 'echo "boom" >&2\nexit 1\n')
    cache = tmp_path / "cache"

    ensure_seed(cache, ws, ROW)
    seed, status = ensure_seed(cache, ws, ROW)

    assert (seed, status) == (None, SEED_FAILED)
    assert len(calls(docker_log)) == 1


def test_a_missing_rtdd_binary_is_raised_not_published_as_a_seed_failure(
    tmp_path, monkeypatch, ws, docker_log
):
    fake_docker(tmp_path, monkeypatch, docker_log, f"echo '{MAP_LINE}'\n")
    monkeypatch.setenv(RTDD_BIN_ENV, str(tmp_path / "definitely-not-here"))
    cache = tmp_path / "cache"

    with pytest.raises(SeedFailed, match="rtdd"):
        ensure_seed(cache, ws, ROW)

    assert not (cache / "seeds" / seed_cache_key(ROW) / "status.json").exists()
    assert calls(docker_log) == []


def test_seed_failed_is_a_runtime_error():
    assert issubclass(SeedFailed, RuntimeError)


# --- install_seed -----------------------------------------------------------


def test_install_seed_places_the_map_where_rtdd_looks_for_it(tmp_path, ws):
    seed = tmp_path / "map.jsonl"
    seed.write_text(MAP_LINE + "\n", encoding="utf-8")

    assert install_seed(ws, seed) is True

    installed = ws.root / ".rtdd" / "map.jsonl"
    assert installed.read_text(encoding="utf-8") == MAP_LINE + "\n"
    meta = json.loads((ws.root / ".rtdd" / "meta.json").read_text(encoding="utf-8"))
    assert meta == {"v": 1, "adapter": "python", "seeded_at": ROW["base_commit"], "cycles": 0}


def test_install_seed_with_no_seed_leaves_an_empty_map(ws):
    assert install_seed(ws, None) is False
    assert (ws.root / ".rtdd").is_dir()
    assert not (ws.root / ".rtdd" / "map.jsonl").exists()


# --- which_tool -------------------------------------------------------------

SEEDED_JSON = json.dumps(
    {
        "schema": 1,
        "command": "which",
        "base": "HEAD",
        "adapter": "python",
        "tier": "T0",
        "reason": "tests whose recorded coverage intersects the changed set",
        "complete": True,
        "warnings": [],
        "changed": [
            {
                "path": "src/app.py",
                "status": "modified",
                "instrumentable": True,
                "lines": [{"start": 10, "end": 12}],
            }
        ],
        "selection": {
            "count": 2,
            "direct": [],
            "tests": ["tests/test_a.py::test_x", "tests/test_b.py::test_y"],
            "import_fallback": {"src/new.py": ["tests/test_new.py"]},
        },
        "run": {"executed": False},
        "uncovered": {"available": False},
        "unmapped_files": ["src/new.py"],
        "exit_code": 0,
    }
)

UNSEEDED_JSON = json.dumps(
    {
        "schema": 1,
        "command": "which",
        "base": "HEAD",
        "adapter": "python",
        "tier": "T2",
        "reason": "the map is unseeded, so no selection is trustworthy: run rtdd seed",
        "complete": False,
        "warnings": ["an empty selection is not a pass. Nothing was checked."],
        "changed": [
            {
                "path": "src/app.py",
                "status": "modified",
                "instrumentable": True,
                "lines": [{"start": 3, "end": 3}],
            }
        ],
        "selection": {"count": 0, "direct": [], "tests": [], "import_fallback": {}},
        "run": {"executed": False},
        "uncovered": {"available": False},
        "unmapped_files": [],
        "exit_code": 0,
    }
)


def fake_rtdd(tmp_path, monkeypatch, body: str) -> Path:
    binary = script(tmp_path / "rtdd-which-stub", body)
    monkeypatch.setenv(RTDD_BIN_ENV, str(binary))
    return binary


def seeded(ws: Workspace) -> None:
    install_seed(ws, None)
    (ws.root / ".rtdd" / "map.jsonl").write_text(MAP_LINE + "\n", encoding="utf-8")


def test_which_tool_is_the_rtdd_which_context_tool(tmp_path, monkeypatch, ws):
    fake_rtdd(tmp_path, monkeypatch, f"cat <<'EOF'\n{SEEDED_JSON}\nEOF\n")
    tool = which_tool(ws)

    assert tool.name == "rtdd_which"
    fn = tool.schema["function"]
    assert fn["name"] == "rtdd_which"
    assert fn["parameters"] == {
        "type": "object",
        "properties": {},
        "required": [],
        "additionalProperties": False,
    }
    json.dumps(tool.schema)


def test_which_tool_reports_the_selection(tmp_path, monkeypatch, ws):
    seeded(ws)
    fake_rtdd(tmp_path, monkeypatch, f"cat <<'EOF'\n{SEEDED_JSON}\nEOF\n")

    out = which_tool(ws).call({})

    assert "tests/test_a.py::test_x" in out
    assert "tests/test_b.py::test_y" in out
    assert "src/app.py" in out
    assert "src/new.py" in out
    assert "tests/test_new.py" in out
    assert "T0" in out


def test_which_tool_passes_json_and_the_working_tree_base(tmp_path, monkeypatch, ws):
    seeded(ws)
    log = tmp_path / "which-args.txt"
    fake_rtdd(tmp_path, monkeypatch, f'echo "$@" > {log}\ncat <<\'EOF\'\n{SEEDED_JSON}\nEOF\n')

    which_tool(ws).call({})

    argv = log.read_text(encoding="utf-8").split()
    assert argv[0] == "which"
    assert "--json" in argv
    assert argv[argv.index("--base") + 1] == "HEAD"


def test_an_unseeded_map_answers_that_nothing_was_recorded(tmp_path, monkeypatch, ws):
    install_seed(ws, None)
    fake_rtdd(tmp_path, monkeypatch, f"cat <<'EOF'\n{UNSEEDED_JSON}\nEOF\n")

    out = which_tool(ws).call({}).lower()

    assert "unseeded" in out
    assert "no coverage" in out or "no recorded" in out


def test_an_unseeded_map_does_not_read_as_no_tests_are_relevant(tmp_path, monkeypatch, ws):
    install_seed(ws, None)
    fake_rtdd(tmp_path, monkeypatch, f"cat <<'EOF'\n{UNSEEDED_JSON}\nEOF\n")

    unseeded_out = which_tool(ws).call({})

    # A selection of zero tests is stated as an absence of recording, not as a
    # verdict about the changed code.
    assert "tests whose recorded execution covered those changes (0):" in unseeded_out
    assert "no recorded test execution" in unseeded_out

    # The same empty selection with a real map says none of that.
    seeded(ws)
    fake_rtdd(tmp_path, monkeypatch, f"cat <<'EOF'\n{SEEDED_JSON}\nEOF\n")
    assert "no recorded test execution" not in which_tool(ws).call({})


def test_the_remediation_clause_is_not_addressed_to_the_agent(tmp_path, monkeypatch, ws):
    install_seed(ws, None)
    fake_rtdd(tmp_path, monkeypatch, f"cat <<'EOF'\n{UNSEEDED_JSON}\nEOF\n")

    out = which_tool(ws).call({})

    assert "run rtdd seed" not in out
    assert "no selection is trustworthy" in out


def test_the_tool_text_carries_no_imperative(tmp_path, monkeypatch, ws):
    fake_rtdd(tmp_path, monkeypatch, f"cat <<'EOF'\n{SEEDED_JSON}\nEOF\n")
    tool = which_tool(ws)

    seeded(ws)
    assert lint_context(tool.call({})) == []
    assert lint_context(tool.schema["function"]["description"]) == []

    fake_rtdd(tmp_path, monkeypatch, f"cat <<'EOF'\n{UNSEEDED_JSON}\nEOF\n")
    install_seed(ws, None)
    assert lint_context(which_tool(ws).call({})) == []


def test_an_unavailable_rtdd_is_tool_output_not_an_exception(tmp_path, monkeypatch, ws):
    seeded(ws)
    fake_rtdd(tmp_path, monkeypatch, 'echo "map is corrupt" >&2\nexit 3\n')

    out = which_tool(ws).call({})

    assert "rtdd_which unavailable" in out
    assert "map is corrupt" in out


def test_unparseable_output_is_tool_output_not_an_exception(tmp_path, monkeypatch, ws):
    seeded(ws)
    fake_rtdd(tmp_path, monkeypatch, 'echo "not json"\n')

    assert "rtdd_which unavailable" in which_tool(ws).call({})


def test_the_tool_output_is_clipped(tmp_path, monkeypatch, ws):
    seeded(ws)
    ids = [f"tests/test_{i}.py::test_{i}" for i in range(20_000)]
    doc = json.loads(SEEDED_JSON)
    doc["selection"] = {"count": len(ids), "direct": [], "tests": ids, "import_fallback": {}}
    fake_rtdd(tmp_path, monkeypatch, "cat <<'EOF'\n" + json.dumps(doc) + "\nEOF\n")

    out = which_tool(ws).call({})

    assert len(out) < MAX_OUTPUT_BYTES + 200
    assert "clipped" in out


# --- registry ---------------------------------------------------------------


def test_both_rtdd_arms_get_the_which_tool_and_share_one_seed_cache(
    tmp_path, monkeypatch, ws, rtdd_bin, docker_log
):
    fake_docker(tmp_path, monkeypatch, docker_log, f"echo '{MAP_LINE}'\n")
    cache = tmp_path / "cache"

    tools_d, prov_d = context_tools("rtdd", ws, cache, ROW)
    tools_e, prov_e = context_tools("rtdd_tdd", ws, cache, ROW)

    assert [t.name for t in tools_d] == ["rtdd_which"]
    assert [t.name for t in tools_e] == ["rtdd_which"]
    assert prov_d == prov_e == {"seed_status": SEEDED, "seed_installed": True}
    assert len(calls(docker_log)) == 1
    assert (ws.root / ".rtdd" / "map.jsonl").exists()


def test_a_failed_seed_still_arms_the_instance(tmp_path, monkeypatch, ws, rtdd_bin, docker_log):
    fake_docker(tmp_path, monkeypatch, docker_log, "exit 1\n")

    tools, provenance = context_tools("rtdd", ws, tmp_path / "cache", ROW)

    assert [t.name for t in tools] == ["rtdd_which"]
    assert provenance == {"seed_status": SEED_FAILED, "seed_installed": False}
    assert not (ws.root / ".rtdd" / "map.jsonl").exists()


def test_an_arm_without_a_context_source_gets_nothing(tmp_path, ws):
    for arm in ("vanilla", "tdd"):
        assert context_tools(arm, ws, tmp_path / "cache", ROW) == ([], {})


def test_the_context_tool_arms_are_the_three_context_bearing_arms():
    assert CONTEXT_TOOL_ARMS == {"tdad", "rtdd", "rtdd_tdd"}


def test_the_registry_imports_without_the_tdad_provider():
    # Importing the registry must not pull arm C's provider in, or a TDAD install
    # failure becomes arms D and E's problem too. Checked in a fresh interpreter:
    # in-process, any earlier import in this session — arm C's own test module,
    # collected before anything runs — would make the check pass for the wrong
    # reason, or fail for one.
    assert rtdd_provider.which_tool is not None
    proc = subprocess.run(
        [
            sys.executable,
            "-c",
            "import sys, providers; "
            "assert 'providers.tdad' not in sys.modules, "
            "'importing the registry pulled arm C in eagerly'",
        ],
        cwd=Path(__file__).resolve().parents[1],
        capture_output=True,
        text=True,
    )
    assert proc.returncode == 0, proc.stderr


def test_seed_timeout_default_is_the_documented_ceiling():
    assert rtdd_provider.SEED_TIMEOUT_S == 5400


def test_env_overrides_default_to_the_plain_binaries(monkeypatch):
    monkeypatch.delenv(DOCKER_BIN_ENV, raising=False)
    monkeypatch.delenv(RTDD_BIN_ENV, raising=False)
    assert os.environ.get(DOCKER_BIN_ENV) is None
    assert rtdd_provider._docker_bin() == "docker"
    assert rtdd_provider._rtdd_bin() == "rtdd"
