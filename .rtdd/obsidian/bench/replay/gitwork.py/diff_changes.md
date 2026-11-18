---
id: "bench/replay/gitwork.py::diff_changes"
source: "bench/replay/gitwork.py"
start: 172
end: 186
kind: "func"
is-test: false
tags: [rtdd/code]
---
# diff_changes

#rtdd/code

## Tests

- [[bench/tests/test_gitwork.py/test_diff_changes_reports_adds_and_modifies]] (calls)
- [[bench/tests/test_gitwork.py/test_diff_changes_reports_deletions]] (calls)
- [[bench/tests/test_strategy_full.py/test_full_selection_over_the_synth_repo_matches_the_hand_computed_table]] (calls)
- [[bench/tests/test_strategy_importgraph.py/test_import_selection_over_the_synth_repo_matches_the_hand_computed_table]] (calls)
- [[bench/tests/test_strategy_pathheuristic.py/test_path_selection_over_the_synth_repo_matches_the_hand_computed_table]] (calls)

## Callers

- [[bench/replay/gitwork.py/probe_candidates]] (calls)
- [[bench/replay/gitwork.py/materialise_natural]] (calls)
- [[bench/replay/gitwork.py/materialise_probe]] (calls)

## Callees

- [[bench/replay/gitwork.py/Change]] (calls)
- [[bench/replay/gitwork.py/git]] (calls)
