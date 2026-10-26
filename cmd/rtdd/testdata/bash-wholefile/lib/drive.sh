#!/usr/bin/env bash
# The code under test: drives git, tmux and gh, which the test scripts stub.

spawn_impl() {
  git worktree add "$1"
  tmux new-session -d -s "$1"
}

file_bug() {
  gh issue create --title "$1" --label bug
}
