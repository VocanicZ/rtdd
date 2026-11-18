---
id: "bench/swebench/tests/test_workspace.py::make_upstream"
source: "bench/swebench/tests/test_workspace.py"
start: 16
end: 29
kind: "func"
is-test: false
tags: [rtdd/code]
---
# make_upstream

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

- [[bench/swebench/tests/test_workspace.py/git]] (calls)
