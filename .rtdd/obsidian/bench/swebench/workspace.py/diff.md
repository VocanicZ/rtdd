---
id: "bench/swebench/workspace.py::diff"
source: "bench/swebench/workspace.py"
start: 75
end: 81
kind: "func"
is-test: false
tags: [rtdd/code]
---
# diff

#rtdd/code

## Tests

- [[bench/swebench/tests/test_workspace.py/test_prepare_is_idempotent_for_a_repeated_instance]] (calls)
- [[bench/swebench/tests/test_workspace.py/test_diff_is_empty_when_nothing_changed]] (calls)
- [[bench/swebench/tests/test_workspace.py/test_diff_returns_the_working_tree_patch]] (calls)
- [[bench/swebench/tests/test_workspace.py/test_diff_includes_files_the_agent_created]] (calls)

## Callers

- [[bench/swebench/agent.py/run_agent]] (calls)

## Callees

- [[bench/swebench/workspace.py/_run]] (calls)
