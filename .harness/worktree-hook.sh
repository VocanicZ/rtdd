#!/usr/bin/env bash
# Per-role model routing for the Harness fleet.
#
# The engine has ONE global HARNESS_CLAUDE_FLAGS for every session type (scripts/lib.sh:768), so
# there is no per-role model knob. This hook is the only seam that can tell the roles apart: the
# engine calls it with the session's working directory, and the directory name encodes the role.
#
#   .harness/checkouts/rtdd          orchestration: PLAN / PRD / DECOMPOSE / REVIEW  -> opus
#   .harness/worktrees/triage-<slug>-iN  bug triage: analysis, read-only, no commits    -> opus
#   .harness/worktrees/rtdd-iN         implementer                                    -> sonnet
#   .harness/worktrees/bug-<slug>-iN     bug fix (an implementer by another name)       -> sonnet
#
# Critic sub-agents need no handling: review.md spawns them from the REVIEW session, so they
# inherit whatever that session runs on.
#
# ensure_bypass (lib.sh:167) merges permissions.defaultMode into this same file after the hook
# runs rather than overwriting it, so the model key survives the launch path.
set -uo pipefail

wd="${1:-$PWD}"
case "$(basename "$wd")" in
  triage-*)   model=opus   ;;
  *-i[0-9]*)  model=sonnet ;;
  *)          model=opus   ;;
esac

mkdir -p "$wd/.claude"
printf '{\n  "model": "%s"\n}\n' "$model" > "$wd/.claude/settings.local.json"
echo "worktree-hook: model=$model for $wd"
