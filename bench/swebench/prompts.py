"""Arm composition for Axis 1.

Every context-bearing arm is its control plus exactly one ``<test-context>``
block. The composition is done by :func:`build`, which has no per-arm prose of
its own, so there is no code path that can add a sentence to the RTDD arm
without adding it to the vanilla arm too. :func:`control_for` is the single
place the arm-to-control pairing lives; the byte-equality tests read it rather
than restating it.

TDAD measured procedural TDD instructions at 9.94% regressions against a 6.08%
vanilla baseline — worse than no intervention. The ``rtdd`` arm therefore states
what information exists and what it means, and never what to do with it. That
is a claim about the text, so it is checked by :func:`lint_context` rather than
left to a reviewer's eye: a phrase list in one auditable tuple, matched against
the block a context arm actually ships.
"""

from __future__ import annotations

import re

#: The arms of the pre-registration, in the order its table lists them.
ARMS: tuple[str, ...] = ("vanilla", "tdd", "tdad", "rtdd", "rtdd_tdd")

BASE = """You are working inside a Python repository at a specific commit. A bug report or \
feature request is described below. Edit the repository so the described problem is fixed.

Repository root: {repo_root}

Issue:
{problem_statement}

You have tools for listing directories, reading files, searching with grep, editing files, \
and running shell commands inside the repository. When you consider the work complete, call \
the finish tool. Your final answer is the diff of the working tree, so leave the repository \
in the state you want evaluated and do not commit."""

TDD_PROSE = """Follow a test-driven workflow. Before changing any source file, first write \
or identify a test that fails because of the described problem. Run it and confirm it fails. \
Then make the smallest change that makes it pass, and run the test again to confirm it \
passes. Before you finish, re-run the tests you have touched and make sure none of them \
regressed. Do not skip the failing-test step."""

RTDD_CONTEXT = """A tool named rtdd_which is available in this repository.

Given the repository's current uncommitted changes, it reports two things, derived from \
coverage recorded during a prior full run of this suite:

- the tests whose recorded execution touched the files you have changed, ranked by how much \
of each test's recorded file set the change overlaps;
- the lines you have changed that no recorded test executed, and separately the changed \
lines that executed only at import time and are attributed to no test.

The report reflects execution, not static analysis, so it includes edges reached through \
dynamic dispatch, dependency injection, and monkeypatching, and it omits any path no test \
has ever taken. Its output is information about this repository's test suite. It does not \
gate anything and it has no opinion about your patch."""

TDAD_CONTEXT = """A tool named test_impact is available in this repository.

Given a list of source files, it reports the tests at risk of regressing if those files \
change, drawn from a static index of the repository built by parsing its Python syntax \
trees: file-to-function edges, test-to-function edges, transitive call chains, and import \
dependencies. Each reported test carries a risk tier of high, medium, or low.

The index is static, so it covers call edges that no test has yet exercised, and it does \
not see edges that exist only at runtime."""

#: Phrases a context block may not carry, in one tuple so the ban is auditable
#: in a single read. Three kinds live here, and the leading marker is the only
#: thing that distinguishes them:
#:
#: * a plain phrase is banned anywhere in the block — second-person directives
#:   ("you must"), sequencing glue ("first, ", "then, "), step numbering
#:   ("step 1"), and the vocabulary of a protocol ("workflow", "test-driven");
#: * a phrase prefixed with ``^`` is a bare imperative opener, banned only when
#:   it opens a sentence, a line, or a bullet. "run" as a noun ("a prior full
#:   run of this suite") is description; "Run the ranked tests" is an order.
#:
#: Entries are lower-case; matching lower-cases the text first.
BANNED_IMPERATIVES: tuple[str, ...] = (
    # second-person directives
    "you must",
    "you should",
    "you need to",
    "you will need to",
    "make sure",
    "be sure to",
    "your job is to",
    "before you",
    # sequencing glue
    "first, ",
    "then, ",
    "first write",
    "before changing",
    "before committing",
    "start by",
    "then run",
    "always run",
    "do not skip",
    # step numbering
    "step 1",
    "step 2",
    "step 3",
    # the vocabulary of a protocol
    "follow a",
    "follow these",
    "follow the",
    "workflow",
    "red-green",
    "test-driven",
    "procedure",
    "protocol",
    # bare imperative openers
    "^run",
    "^rerun",
    "^re-run",
    "^write",
    "^add",
    "^edit",
    "^apply",
    "^fix",
    "^use",
    "^call",
    "^read",
    "^check",
    "^ensure",
    "^verify",
    "^confirm",
    "^avoid",
    "^prefer",
    "^consider",
    "^keep",
    "^make",
    "^do",
    "^don't",
    "^start",
    "^begin",
    "^follow",
    "^focus",
    "^pick",
    "^choose",
    "^select",
    "^prioritise",
    "^prioritize",
    "^treat",
    "^remember",
    "^note",
    "^first",
    "^next",
    "^then",
    "^finally",
    "^after",
    "^before",
)

_OPEN = "<test-context>"
_CLOSE = "</test-context>"

#: The arm each context-bearing arm must equal, minus its block. ``None`` marks
#: a control: it is nobody's control-plus-a-block, it is the thing itself.
_CONTROL: dict[str, str | None] = {
    "vanilla": None,
    "tdd": None,
    "tdad": "tdd",
    "rtdd": "vanilla",
    "rtdd_tdd": "tdd",
}

_CONTEXT: dict[str, str | None] = {
    "vanilla": None,
    "tdd": None,
    "tdad": TDAD_CONTEXT,
    "rtdd": RTDD_CONTEXT,
    "rtdd_tdd": RTDD_CONTEXT,
}

_PROSE: dict[str, bool] = {
    "vanilla": False,
    "tdd": True,
    "tdad": True,
    "rtdd": False,
    "rtdd_tdd": True,
}

#: A sentence ends at one of these followed by whitespace, or at a line break.
#: Bullets, quotes and list numbering are stripped from what follows, so an
#: imperative cannot hide behind "- " or "1) ".
_BOUNDARY = re.compile(r"[.!?:;]\s|\n")
_LEADING = " \t\r-*•>#\"'()[]0123456789."


def context_block(text: str) -> str:
    """Wrap ``text`` in exactly one delimited block."""
    return f"{_OPEN}\n{text}\n{_CLOSE}\n"


class ContextBlockError(ValueError):
    """A prompt carries something other than zero or one well-formed context block."""


def extract_context(prompt: str) -> str | None:
    """Return the text inside ``prompt``'s one ``<test-context>`` block, or ``None``.

    This reads an *assembled* prompt rather than a module constant, so the arm
    driver's dry run can check the guarantee against the bytes an instance would
    actually be given. Two blocks, or a stray delimiter, is a
    :class:`ContextBlockError` rather than a best guess: a prompt whose shape is
    ambiguous is exactly the prompt nobody should be publishing a rate from.
    """
    opens = prompt.count(_OPEN)
    closes = prompt.count(_CLOSE)
    if opens == 0 and closes == 0:
        return None
    if opens != 1 or closes != 1:
        raise ContextBlockError(
            f"expected zero or one <test-context> block, found {opens} open and {closes} close"
        )
    start = prompt.index(_OPEN) + len(_OPEN)
    end = prompt.index(_CLOSE)
    if end < start:
        raise ContextBlockError("</test-context> precedes <test-context>")
    inner = prompt[start:end]
    return inner[1:-1] if inner.startswith("\n") and inner.endswith("\n") else inner


def control_for(arm: str) -> str | None:
    """Return the arm ``arm`` must equal minus its block, or ``None`` if it is a control."""
    return _CONTROL[arm]


def build(arm: str) -> str:
    """Return the system prompt for ``arm``.

    Raises :class:`KeyError` for anything that is not one of :data:`ARMS`. A
    default prompt for an unknown arm would run silently and publish as a real
    arm, which is the one failure this function can produce that nothing
    downstream could detect.
    """
    if arm not in _CONTROL:
        raise KeyError(arm)
    prompt = BASE
    if _PROSE[arm]:
        prompt = prompt + "\n" + TDD_PROSE
    context = _CONTEXT[arm]
    if context is not None:
        prompt = prompt + "\n" + context_block(context)
    return prompt


def _sentence_openers(text: str) -> list[str]:
    openers = []
    for chunk in _BOUNDARY.split(text):
        stripped = chunk.lstrip(_LEADING)
        if stripped:
            openers.append(stripped.lower())
    return openers


def lint_context(text: str) -> list[str]:
    """Return a finding for every banned imperative in ``text``; ``[]`` when clean.

    Each finding names the offending phrase, so a failure says what to delete
    rather than only that something is wrong.
    """
    lowered = text.lower()
    openers = _sentence_openers(text)
    findings: list[str] = []
    for phrase in BANNED_IMPERATIVES:
        if phrase.startswith("^"):
            verb = phrase[1:]
            if any(o == verb or o.startswith(verb + " ") for o in openers):
                findings.append(f'bare imperative opener: "{verb}"')
        elif phrase in lowered:
            findings.append(f'banned imperative phrase: "{phrase}"')
    return findings
