"""The tool surface every arm shares, plus the per-arm context tool slot.

Nothing here is arm-aware. An arm differs from another arm by its system prompt
and by which extra context tool the caller appends to this list — never by a
branch inside a tool, because a scaffold difference is a confound.

Two ceilings are documented and enforced, so one runaway ``grep`` cannot blow the
context window and one runaway test suite cannot hang the run:

* every tool's output is clipped to ``MAX_OUTPUT_BYTES`` and the clip is *visible*
  in the returned text — a silent truncation would read to the model as a
  complete answer;
* ``run_shell`` and ``grep`` run under ``SHELL_TIMEOUT_S`` wall-clock seconds and
  return the timeout as ordinary tool output, so a slow command costs a turn
  rather than the instance.
"""

from __future__ import annotations

import subprocess
from collections.abc import Callable
from dataclasses import dataclass
from pathlib import Path

from workspace import Workspace

#: Ceiling on a single ``read_file`` before line slicing, in bytes.
MAX_READ_BYTES = 60_000
#: Ceiling on any tool's returned text, in bytes.
MAX_OUTPUT_BYTES = 20_000
#: Wall-clock ceiling on ``run_shell`` and ``grep``, in seconds.
SHELL_TIMEOUT_S = 300


@dataclass
class Tool:
    name: str
    schema: dict
    call: Callable[[dict], str]


class PathEscape(ValueError):
    """A tool argument pointed outside the workspace root."""


def _clip(text: str, limit: int = MAX_OUTPUT_BYTES) -> str:
    if len(text) <= limit:
        return text
    return text[:limit] + f"\n... [clipped, {len(text) - limit} more bytes]"


def _resolve(ws: Workspace, rel: str) -> Path:
    root = ws.root.resolve()
    target = (root / rel).resolve()
    if target != root and not target.is_relative_to(root):
        raise PathEscape(f"path escapes the repository: {rel}")
    return target


def base_tools(ws: Workspace, *, shell_timeout_s: int = SHELL_TIMEOUT_S) -> list[Tool]:
    def list_dir(args: dict) -> str:
        rel = args.get("path", ".")
        target = _resolve(ws, rel)
        if not target.is_dir():
            return f"not a directory: {rel}"
        entries = sorted(
            (p.name + ("/" if p.is_dir() else "")) for p in target.iterdir() if p.name != ".git"
        )
        return _clip("\n".join(entries) or "(empty)")

    def read_file(args: dict) -> str:
        target = _resolve(ws, args["path"])
        if not target.is_file():
            return f"no such file: {args['path']}"
        text = target.read_text(encoding="utf-8", errors="replace")[:MAX_READ_BYTES]
        lines = text.splitlines()
        start = max(1, int(args.get("start", 1)))
        end = min(len(lines), int(args.get("end", len(lines))))
        return _clip("\n".join(f"{i}\t{lines[i - 1]}" for i in range(start, end + 1)))

    def grep(args: dict) -> str:
        target = _resolve(ws, args.get("path", "."))
        try:
            proc = subprocess.run(
                ["grep", "-rnI", "--exclude-dir=.git", "-e", args["pattern"], str(target)],
                capture_output=True,
                text=True,
                timeout=shell_timeout_s,
            )
        except subprocess.TimeoutExpired:
            return f"grep timed out after {shell_timeout_s}s; narrow the pattern or the path"
        rel = proc.stdout.replace(str(ws.root.resolve()) + "/", "")
        return _clip(rel or "(no matches)")

    def edit_file(args: dict) -> str:
        target = _resolve(ws, args["path"])
        old, new = args["old"], args["new"]
        if not target.exists():
            if old:
                return f"no such file: {args['path']}"
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_text(new, encoding="utf-8")
            return f"created {args['path']}"
        text = target.read_text(encoding="utf-8")
        occurrences = text.count(old)
        if occurrences == 0:
            return "old text not found; read the file again and retry"
        if occurrences > 1:
            return f"old text appears {occurrences} times; include more context"
        target.write_text(text.replace(old, new, 1), encoding="utf-8")
        return f"edited {args['path']}"

    def run_shell(args: dict) -> str:
        try:
            proc = subprocess.run(
                ["bash", "-lc", args["command"]],
                cwd=ws.root,
                capture_output=True,
                text=True,
                timeout=shell_timeout_s,
            )
        except subprocess.TimeoutExpired:
            return f"exit=timeout\ncommand timed out after {shell_timeout_s}s and was killed"
        return _clip(f"exit={proc.returncode}\n{proc.stdout}\n{proc.stderr}")

    def finish(args: dict) -> str:
        return "FINISH"

    def guarded(fn: Callable[[dict], str]) -> Callable[[dict], str]:
        def wrapped(args: dict) -> str:
            try:
                return fn(args)
            except PathEscape as exc:
                return str(exc)

        return wrapped

    def t(name: str, desc: str, props: dict, required: list[str], fn) -> Tool:
        return Tool(
            name=name,
            schema={
                "type": "function",
                "function": {
                    "name": name,
                    "description": desc,
                    "parameters": {
                        "type": "object",
                        "properties": props,
                        "required": required,
                        "additionalProperties": False,
                    },
                },
            },
            call=guarded(fn),
        )

    s = {"type": "string"}
    i = {"type": "integer"}
    return [
        t("list_dir", "List a directory in the repository.", {"path": s}, [], list_dir),
        t(
            "read_file",
            "Read a file, optionally a line range, with line numbers.",
            {"path": s, "start": i, "end": i},
            ["path"],
            read_file,
        ),
        t(
            "grep",
            "Recursively search the repository for a pattern.",
            {"pattern": s, "path": s},
            ["pattern"],
            grep,
        ),
        t(
            "edit_file",
            "Replace a unique block of text in a file, or create a new file by "
            "passing an empty old.",
            {"path": s, "old": s, "new": s},
            ["path", "old", "new"],
            edit_file,
        ),
        t("run_shell", "Run a bash command from the repository root.", {"command": s}, ["command"], run_shell),
        t("finish", "Signal that the work is complete.", {}, [], finish),
    ]
