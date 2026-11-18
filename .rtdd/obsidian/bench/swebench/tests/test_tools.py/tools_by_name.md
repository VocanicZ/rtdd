---
id: "bench/swebench/tests/test_tools.py::tools_by_name"
source: "bench/swebench/tests/test_tools.py"
start: 20
end: 21
kind: "func"
is-test: false
tags: [rtdd/code]
---
# tools_by_name

#rtdd/code

## Tests

- [[bench/swebench/tests/test_tools.py/test_base_tools_expose_the_shared_surface]] (calls)
- [[bench/swebench/tests/test_tools.py/test_list_dir_lists_the_repository_without_git]] (calls)
- [[bench/swebench/tests/test_tools.py/test_read_file_numbers_lines]] (calls)
- [[bench/swebench/tests/test_tools.py/test_read_file_honours_a_line_range]] (calls)
- [[bench/swebench/tests/test_tools.py/test_read_file_reports_a_missing_file_as_output]] (calls)
- [[bench/swebench/tests/test_tools.py/test_grep_returns_repo_relative_matches]] (calls)
- [[bench/swebench/tests/test_tools.py/test_grep_reports_no_matches_as_output]] (calls)
- [[bench/swebench/tests/test_tools.py/test_edit_file_replaces_a_unique_block]] (calls)
- [[bench/swebench/tests/test_tools.py/test_edit_file_refuses_an_ambiguous_block]] (calls)
- [[bench/swebench/tests/test_tools.py/test_edit_file_creates_a_file_when_old_is_empty]] (calls)
- [[bench/swebench/tests/test_tools.py/test_run_shell_returns_exit_code_and_output]] (calls)
- [[bench/swebench/tests/test_tools.py/test_run_shell_runs_from_the_repository_root]] (calls)
- [[bench/swebench/tests/test_tools.py/test_run_shell_returns_a_timeout_as_tool_output]] (calls)
- [[bench/swebench/tests/test_tools.py/test_finish_signals_completion]] (calls)
- [[bench/swebench/tests/test_tools.py/test_path_traversal_is_rejected]] (calls)
- [[bench/swebench/tests/test_tools.py/test_an_absolute_path_outside_the_workspace_is_rejected]] (calls)
- [[bench/swebench/tests/test_tools.py/test_a_sibling_directory_sharing_the_root_prefix_is_rejected]] (calls)
- [[bench/swebench/tests/test_tools.py/test_a_symlink_out_of_the_workspace_is_rejected]] (calls)
- [[bench/swebench/tests/test_tools.py/test_tool_output_is_clipped_and_says_so]] (calls)
- [[bench/swebench/tests/test_tools.py/test_shell_output_is_clipped_and_says_so]] (calls)

## Callers

none

## Callees

- [[bench/swebench/tools.py/base_tools]] (calls)
