"""A minimal OpenAI-compatible tool loop.

The endpoint is OpenAI-compatible on purpose: the pre-registered model is
Qwen3-Coder-30B-A3B-Instruct served locally, matching TDAD's model class so the
published 6.08% / 9.94% / 1.82% figures remain a meaningful reference point.
Swapping in a frontier model would make the comparison to their table
meaningless, and is listed as future work rather than run here.

One scaffold serves every arm. The system prompt is a parameter and the tool
list is a parameter; nothing in this module knows which arm it is running, so a
difference between two arms can only be the prompt or the extra context tool —
never the loop, the turn ceiling, or the tool surface.

The loop stops for exactly three reasons, each recorded in ``stop_reason``:

``no_tool_call``
    the model replied with prose and asked for nothing more;
``finish``
    the model called the ``finish`` tool;
``max_turns``
    the turn ceiling was reached;
``budget``
    the caller's budget hook raised :class:`BudgetExceeded`.
"""

from __future__ import annotations

import json
from collections.abc import Callable
from dataclasses import dataclass, field

from tools import Tool
from workspace import Workspace, diff


class BudgetExceeded(RuntimeError):
    """Raised by a caller's budget hook to end the instance early."""


@dataclass
class AgentConfig:
    base_url: str = "http://127.0.0.1:8000/v1"
    api_key: str = "local"
    model: str = "Qwen3-Coder-30B-A3B-Instruct"
    temperature: float = 0.0
    max_tokens: int = 4096
    max_turns: int = 40
    #: Called before every model request as ``hook(turn, prompt_tokens,
    #: completion_tokens)``. Raise :class:`BudgetExceeded` to stop the loop.
    budget_hook: Callable[[int, int, int], None] | None = None


@dataclass
class AgentResult:
    patch: str
    turns: int
    prompt_tokens: int
    completion_tokens: int
    stop_reason: str
    transcript: list[dict] = field(default_factory=list)


def _client(cfg: AgentConfig):
    from openai import OpenAI

    return OpenAI(base_url=cfg.base_url, api_key=cfg.api_key)


def _dispatch(call, by_name: dict[str, Tool]) -> str:
    try:
        args = json.loads(call.function.arguments or "{}")
    except json.JSONDecodeError as exc:
        return f"tool arguments were not valid JSON: {exc}"
    tool = by_name.get(call.function.name)
    if tool is None:
        return f"no such tool: {call.function.name}"
    try:
        return tool.call(args)
    except Exception as exc:  # a tool crash is data, not a run failure
        return f"tool error: {type(exc).__name__}: {exc}"


def run_agent(
    ws: Workspace,
    system_prompt: str,
    tools: list[Tool],
    cfg: AgentConfig,
    client=None,
) -> AgentResult:
    """Drive one instance to a patch, and report how it stopped."""
    client = client if client is not None else _client(cfg)
    by_name = {t.name: t for t in tools}
    messages: list[dict] = [
        {"role": "system", "content": system_prompt},
        {"role": "user", "content": "Begin."},
    ]

    prompt_tokens = completion_tokens = 0
    turns = 0
    stop_reason = "max_turns"

    for turn in range(cfg.max_turns):
        if cfg.budget_hook is not None:
            try:
                cfg.budget_hook(turn, prompt_tokens, completion_tokens)
            except BudgetExceeded:
                stop_reason = "budget"
                break

        response = client.chat.completions.create(
            model=cfg.model,
            messages=messages,
            tools=[t.schema for t in tools],
            temperature=cfg.temperature,
            max_tokens=cfg.max_tokens,
        )
        turns += 1

        usage = getattr(response, "usage", None)
        if usage is not None:
            prompt_tokens += usage.prompt_tokens or 0
            completion_tokens += usage.completion_tokens or 0

        message = response.choices[0].message
        tool_calls = list(message.tool_calls or [])
        messages.append(
            {
                "role": "assistant",
                "content": message.content or "",
                "tool_calls": [tc.model_dump() for tc in tool_calls],
            }
        )

        if not tool_calls:
            stop_reason = "no_tool_call"
            break

        finished = False
        for call in tool_calls:
            result = _dispatch(call, by_name)
            if result == "FINISH":
                finished = True
                result = "acknowledged"
            messages.append({"role": "tool", "tool_call_id": call.id, "content": result})

        if finished:
            stop_reason = "finish"
            break

    return AgentResult(
        patch=diff(ws),
        turns=turns,
        prompt_tokens=prompt_tokens,
        completion_tokens=completion_tokens,
        stop_reason=stop_reason,
        transcript=messages,
    )
