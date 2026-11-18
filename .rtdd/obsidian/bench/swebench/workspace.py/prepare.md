---
id: "bench/swebench/workspace.py::prepare"
source: "bench/swebench/workspace.py"
start: 57
end: 72
kind: "func"
is-test: false
tags: [rtdd/code]
---
# prepare

#rtdd/code

## Tests

- [[bench/swebench/tests/test_workspace.py/test_prepare_checks_out_the_base_commit]] (calls)
- [[bench/swebench/tests/test_workspace.py/test_prepare_creates_a_worktree_of_the_shared_mirror]] (calls)
- [[bench/swebench/tests/test_workspace.py/test_second_instance_from_the_same_repo_reuses_the_mirror]] (calls)
- [[bench/swebench/tests/test_workspace.py/test_prepare_fetches_when_the_base_commit_is_not_yet_mirrored]] (calls)
- [[bench/swebench/tests/test_workspace.py/test_prepare_is_idempotent_for_a_repeated_instance]] (calls)
- [[bench/swebench/tests/test_workspace.py/test_two_repos_get_two_mirrors]] (calls)
- [[bench/swebench/tests/test_workspace.py/test_diff_is_empty_when_nothing_changed]] (calls)
- [[bench/swebench/tests/test_workspace.py/test_diff_returns_the_working_tree_patch]] (calls)
- [[bench/swebench/tests/test_workspace.py/test_diff_includes_files_the_agent_created]] (calls)

## Callers

none

## Callees

- [[bench/swebench/workspace.py/Workspace]] (calls)
- [[bench/swebench/workspace.py/_run]] (calls)
- [[bench/swebench/workspace.py/_mirror]] (calls)
