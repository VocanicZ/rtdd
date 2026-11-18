---
id: "bench/replay/envsetup.py::with_source_path"
source: "bench/replay/envsetup.py"
start: 205
end: 222
kind: "func"
is-test: false
tags: [rtdd/code]
---
# with_source_path

#rtdd/code

## Tests

- [[bench/tests/test_envsetup.py/test_the_worktree_source_goes_in_front_of_the_editable_install]] (calls)
- [[bench/tests/test_envsetup.py/test_a_flat_layout_puts_the_worktree_itself_on_the_path]] (calls)

## Callers

- [[bench/replay/cli.py/cmd_session]] (calls)
- [[bench/replay/replay.py/replay_repo]] (calls)

## Callees

- [[bench/replay/envsetup.py/source_root]] (calls)
