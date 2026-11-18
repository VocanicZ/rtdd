---
id: "bench/replay/gitwork.py::clone_pinned"
source: "bench/replay/gitwork.py"
start: 70
end: 94
kind: "func"
is-test: false
tags: [rtdd/code]
---
# clone_pinned

#rtdd/code

## Tests

- [[bench/tests/test_gitwork.py/test_clone_pinned_checks_out_the_pin_and_refuses_a_moved_clone]] (calls)

## Callers

- [[bench/replay/cli.py/cmd_replay]] (calls)
- [[bench/replay/cli.py/cmd_session]] (calls)

## Callees

- [[bench/replay/gitwork.py/GitError]] (calls)
- [[bench/replay/gitwork.py/git]] (calls)
