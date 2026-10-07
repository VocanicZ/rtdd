"""The rtdd binary adapter: schema 3, failure paths, and the argv contract.

Every test here except the last runs against a *stub* binary written into
``tmp_path``, so the module passes on a machine with no ``rtdd``. The last test
is the contract test: it runs the real v0.3.0 binary and is the only thing that
can catch the schema drifting away from what the parser assumes.
"""
from __future__ import annotations

import copy
import inspect
import json
import pathlib
import subprocess

import pytest

from replay import rtddio
from replay.rtddio import RtddError, parse_which
from tests.rtddbin import requires_v03_rtdd

# A trimmed but structurally faithful `rtdd which --base HEAD --json`, schema 3
# (cmd/rtdd/which.go): the nine top-level keys, three rounds, every array a list.
WHICH_PAYLOAD = {
    "schema": 3,
    "command": "which",
    "base": "HEAD",
    "graph": {
        "source": "scanner",
        "built_at_commit": "abc1234",
        "stale_files": 1,
        "nodes": 9,
        "edges": 7,
        "tests": 4,
    },
    "changed": [
        {"path": "src/alpha.py", "lines": [{"start": 2, "end": 2}]},
        {"path": "src/legacy.py", "lines": []},
    ],
    "changed_nodes": [
        {"id": "src/alpha.py::add", "file": "src/alpha.py", "name": "add", "start": 1, "end": 2}
    ],
    "rounds": [
        {
            "round": 1,
            "tests": [
                {"id": "tests/test_alpha.py::test_add", "file": "tests/test_alpha.py", "name": "test_add"},
                {
                    "id": "tests/test_alpha.py::TestAlpha::test_neg",
                    "file": "tests/test_alpha.py",
                    "name": "test_neg",
                },
            ],
            "files": ["tests/test_alpha.py"],
        },
        {
            "round": 2,
            "tests": [
                {"id": "tests/test_beta.py::test_mul", "file": "tests/test_beta.py", "name": "test_mul"}
            ],
            "files": ["tests/test_beta.py"],
        },
        {"round": 3, "full_suite": True},
    ],
    "untested": ["src/alpha.py::helper"],
    "warnings": ["src/legacy.py was deleted"],
}


def _stub(
    tmp_path: pathlib.Path,
    name: str,
    *,
    stdout: str = "",
    stderr: str = "",
    code: int = 0,
) -> pathlib.Path:
    """Write an executable stub that replays fixed output and logs its argv."""
    home = tmp_path / name
    home.mkdir(parents=True, exist_ok=True)
    (home / "stdout").write_text(stdout)
    (home / "stderr").write_text(stderr)
    binary = home / "rtdd"
    binary.write_text(
        "#!/bin/sh\n"
        f'printf "%s\\n" "$@" >> "{home}/argv"\n'
        f'cat "{home}/stdout"\n'
        f'cat "{home}/stderr" >&2\n'
        f"exit {code}\n"
    )
    binary.chmod(0o755)
    return binary


def _argv(binary: pathlib.Path) -> list[str]:
    return (binary.parent / "argv").read_text().splitlines()


def _work(tmp_path: pathlib.Path) -> pathlib.Path:
    work = tmp_path / "work"
    work.mkdir(exist_ok=True)
    return work


# --- parsing -----------------------------------------------------------------


def test_parse_which_reads_rounds_one_and_two():
    w = parse_which(WHICH_PAYLOAD, wall_ms=17)
    assert w.round1 == ("tests/test_alpha.py::test_add", "tests/test_alpha.py::TestAlpha::test_neg")
    assert w.round2 == ("tests/test_beta.py::test_mul",)
    assert w.changed == ("src/alpha.py", "src/legacy.py")
    assert w.untested == ("src/alpha.py::helper",)
    assert w.source == "scanner"
    assert w.warnings == ("src/legacy.py was deleted",)
    assert w.wall_ms == 17


def test_the_consumed_schema_is_3():
    assert rtddio.SCHEMA_VERSION == 3


@pytest.mark.parametrize("schema", [2, 4, "3", None])
def test_parse_which_rejects_any_schema_but_3(schema):
    payload = {**WHICH_PAYLOAD, "schema": schema}
    with pytest.raises(RtddError, match="schema"):
        parse_which(payload, wall_ms=0)


def test_parse_which_rejects_a_document_with_no_schema_key():
    payload = {k: v for k, v in WHICH_PAYLOAD.items() if k != "schema"}
    with pytest.raises(RtddError, match="no schema"):
        parse_which(payload, wall_ms=0)


def test_parse_which_rejects_a_schema_2_document():
    v2 = {"schema": 2, "command": "which", "tier": "T0", "selection": {"tests": []}, "changed": []}
    with pytest.raises(RtddError, match="schema 2"):
        parse_which(v2, wall_ms=0)


def test_parse_which_rejects_a_document_with_no_rounds():
    payload = {k: v for k, v in WHICH_PAYLOAD.items() if k != "rounds"}
    with pytest.raises(RtddError, match="rounds"):
        parse_which(payload, wall_ms=0)


@pytest.mark.parametrize("missing", [1, 2])
def test_parse_which_rejects_a_missing_round(missing):
    payload = copy.deepcopy(WHICH_PAYLOAD)
    payload["rounds"] = [r for r in payload["rounds"] if r["round"] != missing]
    with pytest.raises(RtddError, match=f"round {missing}"):
        parse_which(payload, wall_ms=0)


def test_parse_which_rejects_a_test_with_no_id():
    payload = copy.deepcopy(WHICH_PAYLOAD)
    payload["rounds"][0]["tests"].append({"file": "tests/x.py", "name": "test_x"})
    with pytest.raises(RtddError, match="id"):
        parse_which(payload, wall_ms=0)


def test_parse_which_does_not_need_round_3():
    payload = copy.deepcopy(WHICH_PAYLOAD)
    payload["rounds"] = payload["rounds"][:2]
    assert parse_which(payload, wall_ms=0).round2 == ("tests/test_beta.py::test_mul",)


def test_v02_entry_points_are_gone():
    for name in ("seed", "run", "parse_run", "RunOutput", "read_cycles", "expand_ranges"):
        assert not hasattr(rtddio, name), f"rtddio.{name} drives a v0.2 binary"


# --- subprocess failure paths ------------------------------------------------


def test_which_raises_when_the_binary_is_missing(tmp_path):
    with pytest.raises(RtddError, match="not found"):
        rtddio.which(_work(tmp_path), binary=str(tmp_path / "no-such-rtdd"))


def test_which_raises_on_non_json_stdout(tmp_path):
    b = _stub(tmp_path, "b", stdout="rtdd: something human\n")
    with pytest.raises(RtddError, match="non-JSON"):
        rtddio.which(_work(tmp_path), binary=str(b))


def test_which_raises_on_a_usage_exit_code(tmp_path):
    b = _stub(tmp_path, "b", stderr='rtdd which: gitctx: unknown base "NOPE"\n', code=2)
    with pytest.raises(RtddError, match="exited 2"):
        rtddio.which(_work(tmp_path), binary=str(b))


def test_which_raises_on_exit_1(tmp_path):
    # v0.3.0 has no exit code 1 at all; one is a broken binary, not a test result.
    b = _stub(tmp_path, "b", stdout=json.dumps(WHICH_PAYLOAD), code=1)
    with pytest.raises(RtddError, match="exited 1"):
        rtddio.which(_work(tmp_path), binary=str(b))


def test_graph_raises_on_a_graph_that_cannot_be_built(tmp_path):
    b = _stub(tmp_path, "g", stderr="rtdd graph: not a git repository\n", code=3)
    with pytest.raises(RtddError, match="exited 3"):
        rtddio.graph(_work(tmp_path), binary=str(b))


def test_wall_ms_is_measured_not_reported(tmp_path):
    b = _stub(tmp_path, "b", stdout=json.dumps(WHICH_PAYLOAD))
    assert rtddio.which(_work(tmp_path), binary=str(b)).wall_ms >= 0


# --- the argv contract: shipped defaults only --------------------------------


def test_the_harness_passes_no_tuning_flags(tmp_path):
    graph_bin = _stub(tmp_path, "g", stdout="source: scanner\n")
    which_bin = _stub(tmp_path, "w", stdout=json.dumps(WHICH_PAYLOAD))
    work = _work(tmp_path)

    rtddio.graph(work, binary=str(graph_bin))
    rtddio.which(work, binary=str(which_bin))

    assert _argv(graph_bin) == ["graph"]
    assert _argv(which_bin) == ["which", "--base", "HEAD", "--json"]


def test_no_entry_point_accepts_extra_rtdd_arguments():
    # A knob for extra argv is how "shipped defaults only" quietly stops being true.
    for fn in (rtddio.graph, rtddio.which):
        params = set(inspect.signature(fn).parameters)
        assert params <= {"work", "binary", "base"}, fn.__name__


# --- version capture for RunConfig -------------------------------------------


def test_rtdd_version_prefers_the_binarys_own_version_output(tmp_path):
    b = _stub(tmp_path, "v", stdout="rtdd v0.3.0 (commit a3f21e0)\n")
    assert rtddio.rtdd_version(str(b)) == "rtdd v0.3.0 (commit a3f21e0)"


def test_rtdd_version_falls_back_to_a_content_digest(tmp_path):
    b = _stub(tmp_path, "v", stderr="rtdd: unknown command\n", code=2)
    got = rtddio.rtdd_version(str(b))
    assert got.startswith("sha256:")
    assert len(got) == len("sha256:") + 64


def test_rtdd_version_raises_when_the_binary_is_missing(tmp_path):
    with pytest.raises(RtddError, match="not found"):
        rtddio.rtdd_version(str(tmp_path / "no-such-rtdd"))


# --- the contract test: the real binary --------------------------------------


@requires_v03_rtdd
def test_real_binary_honours_the_consumed_schema(synth):
    subprocess.run(["git", "checkout", "-q", synth.sha(2)], cwd=synth.path, check=True)
    rtddio.graph(synth.path)
    assert (synth.path / ".rtdd" / "graph.json").is_file()

    (synth.path / "src" / "alpha.py").write_text("def add(a, b):\n    return a + b + 1\n")
    w = rtddio.which(synth.path)
    assert "tests/test_alpha.py::test_add" in w.round1
    assert "src/alpha.py" in w.changed
    assert w.source == "scanner"
