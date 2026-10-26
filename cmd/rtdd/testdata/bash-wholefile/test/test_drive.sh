#!/usr/bin/env bash
# Runs as a whole: `bash test/test_drive.sh`. Its functions are stubs and steps, not tests.
source "$(dirname "$0")/../lib/drive.sh"
LOG=$(mktemp)

git() {
  echo "git $*" >>"$LOG"
}

tmux() {
  echo "tmux $*" >>"$LOG"
}

gh() {
  echo "gh $*" >>"$LOG"
}

check_spawn() {
  spawn_impl wt1
  grep -q "git worktree add wt1" "$LOG"
}

gh() {
  echo "gh stubbed again" >>"$LOG"
}

check_spawn && echo ok
