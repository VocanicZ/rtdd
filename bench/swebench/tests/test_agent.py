import ast
import json
import re
import subprocess
from dataclasses import dataclass, field
from pathlib import Path

import pytest

from agent import AgentConfig, AgentResult, BudgetExceeded, run_agent
from tools import Tool, base_tools
from workspace import Workspace


# --- a stubbed OpenAI-compatible client -------------------------------------
# The loop's control flow is covered without invoking a model: these objects
# expose exactly the attributes the OpenAI SDK's response objects expose.


@dataclass
class StubFunction:
    name: str
    arguments: str


@dataclass
class StubToolCall:
    id: str
    function: StubFunction
    type: str = "function"

    def model_dump(self) -> dict:
        return {
            "id": self.id,
            "type": self.type,
            "function": {"name": self.function.name, "arguments": self.function.arguments},
        }


@dataclass
class StubMessage:
    content: str = ""
    tool_calls: list = field(default_factory=list)


@dataclass
class StubChoice:
    message: StubMessage


@dataclass
class StubUsage:
    prompt_tokens: int
    completion_tokens: int


@dataclass
class StubResponse:
    choices: list
    usage: StubUsage | None = None


class StubClient:
    """Replays canned responses; records the kwargs of every request."""

    def __init__(self, responses):
        self.responses = list(responses)
        self.requests: list[dict] = []
        self.chat = self

    @property
    def completions(self):
        return self

    def create(self, **kwargs) -> StubResponse:
        self.requests.append(kwargs)
        if not self.responses:
            raise AssertionError("the loop asked for more completions than the stub has")
        response = self.responses.pop(0)
        return response(kwargs) if callable(response) else response


def say(text: str, prompt_tokens: int = 10, completion_tokens: int = 5) -> StubResponse:
    return StubResponse(
        choices=[StubChoice(StubMessage(content=text))],
        usage=StubUsage(prompt_tokens, completion_tokens),
    )


def call(name: str, args: dict, call_id: str = "c1", prompt_tokens: int = 10,
         completion_tokens: int = 5) -> StubResponse:
    return StubResponse(
        choices=[
            StubChoice(
                StubMessage(
                    content="",
                    tool_calls=[StubToolCall(id=call_id, function=StubFunction(name, json.dumps(args)))],
                )
            )
        ],
        usage=StubUsage(prompt_tokens, completion_tokens),
    )


def raw_call(name: str, arguments: str, call_id: str = "c1") -> StubResponse:
    return StubResponse(
        choices=[
            StubChoice(
                StubMessage(
                    content="", tool_calls=[StubToolCall(id=call_id, function=StubFunction(name, arguments))]
                )
            )
        ],
        usage=StubUsage(1, 1),
    )


@pytest.fixture
def ws(tmp_path):
    root = tmp_path / "repo"
    root.mkdir()
    subprocess.run(["git", "init", "-q", "-b", "main", str(root)], check=True)
    for k, v in (("user.email", "b@t"), ("user.name", "b")):
        subprocess.run(["git", "-C", str(root), "config", k, v], check=True)
    (root / "mod.py").write_text("def f():\n    return 1\n", encoding="utf-8")
    subprocess.run(["git", "-C", str(root), "add", "-A"], check=True)
    subprocess.run(["git", "-C", str(root), "commit", "-qm", "base"], check=True)
    head = subprocess.run(
        ["git", "-C", str(root), "rev-parse", "HEAD"], check=True, capture_output=True, text=True
    ).stdout.strip()
    return Workspace(root=root, instance_id="acme__widget-1", repo="acme/widget", base_commit=head)


def test_a_reply_without_a_tool_call_stops_the_loop(ws):
    client = StubClient([say("I am done thinking.")])
    res = run_agent(ws, "SYSTEM", base_tools(ws), AgentConfig(), client=client)

    assert isinstance(res, AgentResult)
    assert res.stop_reason == "no_tool_call"
    assert res.turns == 1


def test_the_finish_tool_stops_the_loop(ws):
    client = StubClient([call("list_dir", {"path": "."}), call("finish", {})])
    res = run_agent(ws, "SYSTEM", base_tools(ws), AgentConfig(), client=client)

    assert res.stop_reason == "finish"
    assert res.turns == 2


def test_the_turn_ceiling_stops_the_loop(ws):
    client = StubClient([call("list_dir", {"path": "."}) for _ in range(3)])
    res = run_agent(ws, "SYSTEM", base_tools(ws), AgentConfig(max_turns=3), client=client)

    assert res.stop_reason == "max_turns"
    assert res.turns == 3


def test_a_raising_budget_hook_stops_the_loop(ws):
    seen = []

    def hook(turn: int, prompt_tokens: int, completion_tokens: int) -> None:
        seen.append((turn, prompt_tokens, completion_tokens))
        if turn == 2:
            raise BudgetExceeded("spend ceiling reached")

    client = StubClient([call("list_dir", {"path": "."}) for _ in range(5)])
    res = run_agent(ws, "SYSTEM", base_tools(ws), AgentConfig(budget_hook=hook), client=client)

    assert res.stop_reason == "budget"
    assert res.turns == 2
    assert seen[0] == (0, 0, 0)
    assert seen[-1][0] == 2


def test_token_counts_accumulate_across_turns(ws):
    client = StubClient(
        [
            call("list_dir", {"path": "."}, prompt_tokens=100, completion_tokens=7),
            call("finish", {}, prompt_tokens=250, completion_tokens=9),
        ]
    )
    res = run_agent(ws, "SYSTEM", base_tools(ws), AgentConfig(), client=client)

    assert res.prompt_tokens == 350
    assert res.completion_tokens == 16


def test_missing_usage_does_not_break_accounting(ws):
    client = StubClient([StubResponse(choices=[StubChoice(StubMessage(content="done"))], usage=None)])
    res = run_agent(ws, "SYSTEM", base_tools(ws), AgentConfig(), client=client)

    assert res.prompt_tokens == 0
    assert res.completion_tokens == 0


def test_the_patch_is_what_the_tools_actually_changed(ws):
    client = StubClient(
        [call("edit_file", {"path": "mod.py", "old": "return 1", "new": "return 42"}), call("finish", {})]
    )
    res = run_agent(ws, "SYSTEM", base_tools(ws), AgentConfig(), client=client)

    assert "+    return 42" in res.patch
    assert (ws.root / "mod.py").read_text(encoding="utf-8") == "def f():\n    return 42\n"


def test_an_agent_that_changed_nothing_returns_an_empty_patch(ws):
    client = StubClient([say("nothing to do")])
    res = run_agent(ws, "SYSTEM", base_tools(ws), AgentConfig(), client=client)

    assert res.patch == ""


def test_the_system_prompt_is_passed_through_verbatim(ws):
    client = StubClient([say("done")])
    run_agent(ws, "ARBITRARY PROMPT TEXT", base_tools(ws), AgentConfig(), client=client)

    messages = client.requests[0]["messages"]
    assert messages[0] == {"role": "system", "content": "ARBITRARY PROMPT TEXT"}


def test_the_registered_tools_are_advertised_to_the_model(ws):
    extra = Tool(
        name="test_context",
        schema={"type": "function", "function": {"name": "test_context", "description": "ctx",
                                                 "parameters": {"type": "object", "properties": {}}}},
        call=lambda args: "context",
    )
    client = StubClient([call("test_context", {}), call("finish", {})])
    res = run_agent(ws, "SYSTEM", [*base_tools(ws), extra], AgentConfig(), client=client)

    advertised = [t["function"]["name"] for t in client.requests[0]["tools"]]
    assert "test_context" in advertised
    assert res.stop_reason == "finish"
    assert any(m.get("role") == "tool" and m["content"] == "context" for m in res.transcript)


def test_an_unknown_tool_is_reported_to_the_model_and_the_loop_continues(ws):
    client = StubClient([call("teleport", {}), call("finish", {})])
    res = run_agent(ws, "SYSTEM", base_tools(ws), AgentConfig(), client=client)

    assert res.stop_reason == "finish"
    assert any("no such tool: teleport" in m.get("content", "") for m in res.transcript if m["role"] == "tool")


def test_unparsable_tool_arguments_are_reported_to_the_model(ws):
    client = StubClient([raw_call("list_dir", "{not json"), call("finish", {})])
    res = run_agent(ws, "SYSTEM", base_tools(ws), AgentConfig(), client=client)

    assert res.stop_reason == "finish"
    assert any("not valid JSON" in m.get("content", "") for m in res.transcript if m["role"] == "tool")


def test_a_crashing_tool_is_data_not_a_run_failure(ws):
    def boom(args: dict) -> str:
        raise RuntimeError("kaboom")

    exploding = Tool(
        name="boom",
        schema={"type": "function", "function": {"name": "boom", "description": "b",
                                                 "parameters": {"type": "object", "properties": {}}}},
        call=boom,
    )
    client = StubClient([call("boom", {}), call("finish", {})])
    res = run_agent(ws, "SYSTEM", [*base_tools(ws), exploding], AgentConfig(), client=client)

    assert res.stop_reason == "finish"
    assert any("tool error: RuntimeError: kaboom" in m.get("content", "") for m in res.transcript
               if m["role"] == "tool")


def test_every_tool_call_gets_a_reply_keyed_by_its_id(ws):
    client = StubClient([call("list_dir", {"path": "."}, call_id="abc"), call("finish", {}, call_id="xyz")])
    res = run_agent(ws, "SYSTEM", base_tools(ws), AgentConfig(), client=client)

    ids = [m["tool_call_id"] for m in res.transcript if m["role"] == "tool"]
    assert ids == ["abc", "xyz"]


def _code_tokens(source: str) -> set[str]:
    """Every identifier and non-docstring string literal, split into words.

    Prose in a docstring may name the arms; executable code may not, because a
    per-arm branch in the shared scaffold would be a confound.
    """
    tree = ast.parse(source)
    docstrings = set()
    for node in ast.walk(tree):
        if isinstance(node, (ast.Module, ast.ClassDef, ast.FunctionDef, ast.AsyncFunctionDef)):
            body = getattr(node, "body", None)
            if body and isinstance(body[0], ast.Expr) and isinstance(body[0].value, ast.Constant):
                if isinstance(body[0].value.value, str):
                    docstrings.add(id(body[0].value))

    words: set[str] = set()

    def add(text: str) -> None:
        words.update(w for w in re.split(r"[^A-Za-z0-9]+", text.lower()) if w)

    for node in ast.walk(tree):
        if isinstance(node, ast.Constant) and isinstance(node.value, str) and id(node) not in docstrings:
            add(node.value)
        elif isinstance(node, ast.Name):
            add(node.id)
        elif isinstance(node, ast.Attribute):
            add(node.attr)
        elif isinstance(node, ast.arg):
            add(node.arg)
        elif isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef)):
            add(node.name)
        elif isinstance(node, ast.keyword) and node.arg:
            add(node.arg)
    return words


@pytest.mark.parametrize("module", ["agent.py", "tools.py", "workspace.py"])
def test_the_scaffold_holds_no_arm_specific_branch(module):
    source = (Path(__file__).resolve().parents[1] / module).read_text(encoding="utf-8")
    arms = {"vanilla", "tdd", "tdad"}
    assert arms.isdisjoint(_code_tokens(source)), f"{module} branches on an arm name"
