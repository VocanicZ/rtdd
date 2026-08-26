import json

import pytest

from tools import MAX_OUTPUT_BYTES, SHELL_TIMEOUT_S, Tool, base_tools
from workspace import Workspace


@pytest.fixture
def ws(tmp_path):
    root = tmp_path / "repo"
    (root / "pkg").mkdir(parents=True)
    (root / ".git").mkdir()
    (root / "mod.py").write_text("def f():\n    return 1\n", encoding="utf-8")
    (root / "pkg" / "util.py").write_text("VALUE = 7\n", encoding="utf-8")
    (tmp_path / "secret.txt").write_text("do not read me\n", encoding="utf-8")
    return Workspace(root=root, instance_id="acme__widget-1", repo="acme/widget", base_commit="0" * 40)


def tools_by_name(ws, **kwargs) -> dict[str, Tool]:
    return {t.name: t for t in base_tools(ws, **kwargs)}


def test_base_tools_expose_the_shared_surface(ws):
    assert set(tools_by_name(ws)) == {"list_dir", "read_file", "grep", "edit_file", "run_shell", "finish"}


def test_every_tool_carries_an_openai_function_schema(ws):
    for tool in base_tools(ws):
        fn = tool.schema["function"]
        assert tool.schema["type"] == "function"
        assert fn["name"] == tool.name
        assert fn["description"]
        assert fn["parameters"]["type"] == "object"
        json.dumps(tool.schema)  # must be serialisable onto the wire


def test_list_dir_lists_the_repository_without_git(ws):
    out = tools_by_name(ws)["list_dir"].call({"path": "."})
    assert "mod.py" in out
    assert "pkg/" in out
    assert ".git" not in out


def test_read_file_numbers_lines(ws):
    out = tools_by_name(ws)["read_file"].call({"path": "mod.py"})
    assert out == "1\tdef f():\n2\t    return 1"


def test_read_file_honours_a_line_range(ws):
    out = tools_by_name(ws)["read_file"].call({"path": "mod.py", "start": 2, "end": 2})
    assert out == "2\t    return 1"


def test_read_file_reports_a_missing_file_as_output(ws):
    assert "no such file" in tools_by_name(ws)["read_file"].call({"path": "nope.py"})


def test_grep_returns_repo_relative_matches(ws):
    out = tools_by_name(ws)["grep"].call({"pattern": "VALUE"})
    assert "pkg/util.py:1:VALUE = 7" in out
    assert str(ws.root) not in out


def test_grep_reports_no_matches_as_output(ws):
    assert "(no matches)" in tools_by_name(ws)["grep"].call({"pattern": "zzzz-not-here"})


def test_edit_file_replaces_a_unique_block(ws):
    out = tools_by_name(ws)["edit_file"].call(
        {"path": "mod.py", "old": "return 1", "new": "return 42"}
    )
    assert "edited" in out
    assert (ws.root / "mod.py").read_text(encoding="utf-8") == "def f():\n    return 42\n"


def test_edit_file_refuses_an_ambiguous_block(ws):
    (ws.root / "dup.py").write_text("x = 1\nx = 1\n", encoding="utf-8")
    out = tools_by_name(ws)["edit_file"].call({"path": "dup.py", "old": "x = 1", "new": "x = 2"})
    assert "2 times" in out
    assert (ws.root / "dup.py").read_text(encoding="utf-8") == "x = 1\nx = 1\n"


def test_edit_file_creates_a_file_when_old_is_empty(ws):
    out = tools_by_name(ws)["edit_file"].call(
        {"path": "tests/test_new.py", "old": "", "new": "assert True\n"}
    )
    assert "created" in out
    assert (ws.root / "tests" / "test_new.py").read_text(encoding="utf-8") == "assert True\n"


def test_run_shell_returns_exit_code_and_output(ws):
    out = tools_by_name(ws)["run_shell"].call({"command": "echo hello && exit 3"})
    assert "exit=3" in out
    assert "hello" in out


def test_run_shell_runs_from_the_repository_root(ws):
    out = tools_by_name(ws)["run_shell"].call({"command": "pwd"})
    assert str(ws.root) in out


def test_run_shell_returns_a_timeout_as_tool_output(ws):
    out = tools_by_name(ws, shell_timeout_s=1)["run_shell"].call({"command": "sleep 30"})
    assert "timed out" in out
    assert "1" in out


def test_finish_signals_completion(ws):
    assert tools_by_name(ws)["finish"].call({}) == "FINISH"


@pytest.mark.parametrize(
    "name,args",
    [
        ("read_file", {"path": "../secret.txt"}),
        ("list_dir", {"path": ".."}),
        ("edit_file", {"path": "../secret.txt", "old": "", "new": "pwned"}),
        ("grep", {"pattern": "read", "path": ".."}),
    ],
)
def test_path_traversal_is_rejected(ws, name, args):
    out = tools_by_name(ws)[name].call(args)
    assert "escapes the repository" in out
    assert "do not read me" not in out
    assert (ws.root.parent / "secret.txt").read_text(encoding="utf-8") == "do not read me\n"


def test_an_absolute_path_outside_the_workspace_is_rejected(ws):
    out = tools_by_name(ws)["read_file"].call({"path": "/etc/passwd"})
    assert "escapes the repository" in out


def test_a_sibling_directory_sharing_the_root_prefix_is_rejected(ws):
    sibling = ws.root.parent / (ws.root.name + "-evil")
    sibling.mkdir()
    (sibling / "loot.txt").write_text("SIBLING CONTENTS\n", encoding="utf-8")
    out = tools_by_name(ws)["read_file"].call({"path": f"../{sibling.name}/loot.txt"})
    assert "escapes the repository" in out
    assert "SIBLING CONTENTS" not in out


def test_a_symlink_out_of_the_workspace_is_rejected(ws):
    (ws.root / "escape").symlink_to(ws.root.parent / "secret.txt")
    out = tools_by_name(ws)["read_file"].call({"path": "escape"})
    assert "escapes the repository" in out
    assert "do not read me" not in out


def test_tool_output_is_clipped_and_says_so(ws):
    (ws.root / "big.txt").write_text("A" * (MAX_OUTPUT_BYTES * 2), encoding="utf-8")
    out = tools_by_name(ws)["read_file"].call({"path": "big.txt"})
    assert len(out) < MAX_OUTPUT_BYTES * 2
    assert "clipped" in out


def test_shell_output_is_clipped_and_says_so(ws):
    out = tools_by_name(ws)["run_shell"].call({"command": f"head -c {MAX_OUTPUT_BYTES * 3} /dev/zero | tr '\\0' 'B'"})
    assert "clipped" in out
    assert len(out) < MAX_OUTPUT_BYTES * 3


def test_the_documented_ceilings_are_bounded(ws):
    assert 0 < MAX_OUTPUT_BYTES <= 200_000
    assert 0 < SHELL_TIMEOUT_S <= 3600
