"""The arm-composition guarantee.

The first three assertions are the benchmark's credibility. Everything else in
this file exists to keep them from being quietly weakened: the lint guards the
context text itself, and the two ``xfail(strict=True)`` fixtures are shipped
evidence that both halves of the guarantee actually fail when violated.
"""

from __future__ import annotations

import pytest

from prompts import (
    ARMS,
    BANNED_IMPERATIVES,
    BASE,
    RTDD_CONTEXT,
    TDAD_CONTEXT,
    TDD_PROSE,
    build,
    context_block,
    control_for,
    lint_context,
)

# --- the guarantee -------------------------------------------------------


def test_rtdd_arm_is_vanilla_plus_one_context_block_byte_for_byte():
    assert build("rtdd") == build("vanilla") + "\n" + context_block(RTDD_CONTEXT)


def test_rtdd_tdd_arm_is_the_tdd_arm_plus_one_context_block():
    assert build("rtdd_tdd") == build("tdd") + "\n" + context_block(RTDD_CONTEXT)


def test_tdad_arm_is_the_tdd_arm_plus_one_context_block():
    # TDAD's published best arm is graph PLUS TDD prose; reproducing it means keeping both.
    assert build("tdad") == build("tdd") + "\n" + context_block(TDAD_CONTEXT)


def test_the_identities_are_driven_by_control_for():
    # The pairing lives in one place: the same mapping the tests above spell out
    # by hand must be the mapping the module exposes.
    for arm, context in (("rtdd", RTDD_CONTEXT), ("rtdd_tdd", RTDD_CONTEXT), ("tdad", TDAD_CONTEXT)):
        control = control_for(arm)
        assert control is not None, arm
        assert build(arm) == build(control) + "\n" + context_block(context), arm


def test_control_mapping_is_explicit():
    assert control_for("vanilla") is None
    assert control_for("tdd") is None
    assert control_for("rtdd") == "vanilla"
    assert control_for("rtdd_tdd") == "tdd"
    assert control_for("tdad") == "tdd"


def test_control_for_rejects_an_unknown_arm():
    with pytest.raises(KeyError):
        control_for("nope")


def test_rtdd_arm_contains_no_tdd_prose():
    assert TDD_PROSE not in build("rtdd")


def test_the_tdd_arm_is_vanilla_plus_exactly_the_prose():
    assert build("tdd") == build("vanilla") + "\n" + TDD_PROSE


def test_vanilla_is_exactly_base():
    assert build("vanilla") == BASE


def test_every_arm_is_base_plus_only_the_shared_parts():
    # Removing BASE, the prose and the one context block must leave nothing: no
    # arm may carry a sentence of its own.
    for arm in ARMS:
        remainder = build(arm).replace(BASE, "", 1).replace("\n" + TDD_PROSE, "", 1)
        for context in (RTDD_CONTEXT, TDAD_CONTEXT):
            remainder = remainder.replace("\n" + context_block(context), "", 1)
        assert remainder == "", arm


# --- the block -----------------------------------------------------------


def test_context_block_is_exactly_one_delimited_block():
    b = context_block("hello")
    assert b.count("<test-context>") == 1
    assert b.count("</test-context>") == 1
    assert b == "<test-context>\nhello\n</test-context>\n"


def test_each_context_arm_carries_exactly_one_block():
    for arm in ("rtdd", "rtdd_tdd", "tdad"):
        assert build(arm).count("<test-context>") == 1, arm
        assert build(arm).count("</test-context>") == 1, arm
    for arm in ("vanilla", "tdd"):
        assert "<test-context>" not in build(arm), arm


# --- the arms ------------------------------------------------------------


def test_arms_are_exactly_the_pre_registered_five():
    assert ARMS == ("vanilla", "tdd", "tdad", "rtdd", "rtdd_tdd")


def test_arms_agree_with_the_provider_registry():
    from providers import ARMS as PROVIDER_ARMS

    assert ARMS == PROVIDER_ARMS


def test_all_arms_build():
    for arm in ARMS:
        assert build(arm).strip()


def test_unknown_arm_is_an_error():
    with pytest.raises(KeyError):
        build("nope")


def test_build_never_returns_a_default_prompt_for_a_near_miss():
    for arm in ("Vanilla", "rtdd-tdd", "rtdd_TDD", "", "tddd"):
        with pytest.raises(KeyError):
            build(arm)


# --- the lint ------------------------------------------------------------


def test_rtdd_context_carries_no_imperative():
    assert lint_context(RTDD_CONTEXT) == []


def test_tdad_context_carries_no_imperative():
    # Arm C's procedure lives in TDD_PROSE, which it gets openly; its block is
    # description only, on the same terms as ours.
    assert lint_context(TDAD_CONTEXT) == []


def test_the_tdd_prose_is_what_the_lint_is_for():
    # Sanity: the lint is not vacuous — the prose the RTDD arm must never carry
    # is exactly what it flags.
    assert lint_context(TDD_PROSE)


def test_lint_catches_a_smuggled_procedure():
    smuggled = RTDD_CONTEXT + "\nBefore you commit, you must run every listed test first."
    findings = lint_context(smuggled)
    assert findings
    assert any("you must" in f for f in findings)


def test_lint_catches_step_numbering():
    assert lint_context("Step 1. Write a failing test.\nStep 2. Make it pass.")


def test_lint_catches_a_bare_imperative_opener():
    findings = lint_context("Run the ranked tests after editing.")
    assert findings
    assert any("run" in f.lower() for f in findings)


def test_lint_does_not_flag_a_banned_opener_used_descriptively():
    # "run" mid-sentence is a noun here; only the sentence-opening imperative is banned.
    assert lint_context("Coverage recorded during a prior full run of this suite.") == []


def test_lint_finds_an_opener_after_a_newline_or_a_bullet():
    assert lint_context("Some description.\nWrite the failing test.")
    assert lint_context("Some description:\n- Verify the ranked tests.")


def test_every_banned_phrase_is_actually_caught():
    for phrase in BANNED_IMPERATIVES:
        probe = phrase.lstrip("^")
        assert lint_context(f"Filler sentence. {probe} the tests."), phrase


def test_banned_imperatives_is_one_auditable_tuple():
    assert isinstance(BANNED_IMPERATIVES, tuple)
    assert all(isinstance(p, str) and p == p.lower() and p.strip() for p in BANNED_IMPERATIVES)
    assert len(set(BANNED_IMPERATIVES)) == len(BANNED_IMPERATIVES)


def test_banned_imperatives_covers_the_three_required_categories():
    for phrase in ("you must", "you should", "make sure", "before you", "first, ", "then, "):
        assert phrase in BANNED_IMPERATIVES, phrase
    for phrase in ("step 1", "step 2"):
        assert phrase in BANNED_IMPERATIVES, phrase
    assert any(p.startswith("^") for p in BANNED_IMPERATIVES)


def test_findings_name_the_offending_phrase():
    findings = lint_context("You should make sure the tests pass.")
    assert any("you should" in f for f in findings)
    assert any("make sure" in f for f in findings)


# --- shipped evidence that the guarantee is load-bearing ------------------


@pytest.mark.xfail(
    strict=True,
    reason="evidence: a procedure smuggled into RTDD_CONTEXT must fail the lint gate",
)
def test_a_smuggled_procedure_fails_the_lint_gate():
    smuggled = RTDD_CONTEXT + "\nStep 1. Write a failing test. Then, make sure it passes."
    assert lint_context(smuggled) == []


@pytest.mark.xfail(
    strict=True,
    reason="evidence: an extra sentence in the RTDD arm must break byte equality",
)
def test_an_extra_sentence_in_the_rtdd_arm_breaks_byte_equality():
    tampered = build("rtdd") + "\nAlso, you should re-run the tests you touched."
    assert tampered == build("vanilla") + "\n" + context_block(RTDD_CONTEXT)
