---
id: "bench/replay/gitwork.py::remove_worktree"
source: "bench/replay/gitwork.py"
start: 196
end: 201
kind: "func"
is-test: false
tags: [rtdd/code]
---
# remove_worktree

#rtdd/code

## Tests

- [[bench/tests/test_gitwork.py/test_worktrees_are_detached_and_removed_without_disturbing_the_clone]] (calls)

## Callers

- [[bench/replay/cli.py/cmd_session]] (calls)
- [[bench/replay/replay.py/replay_repo]] (calls)

## Callees

- [[bench/replay/gitwork.py/git]] (calls)
