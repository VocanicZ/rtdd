---
id: "bench/replay/derive.py::derive_static"
source: "bench/replay/derive.py"
start: 136
end: 178
kind: "func"
is-test: false
tags: [rtdd/code]
---
# derive_static

#rtdd/code

## Tests

- [[bench/tests/test_derive.py/test_static_is_level1_united_with_the_committed_import_graph]] (calls)
- [[bench/tests/test_derive.py/test_a_derived_record_carries_no_measurement_and_says_so]] (calls)
- [[bench/tests/test_derive.py/test_the_selection_is_ordered_so_the_record_is_byte_stable]] (calls)
- [[bench/tests/test_derive.py/test_an_id_the_commit_never_collected_is_refused]] (calls)
- [[bench/tests/test_derive.py/test_a_commit_with_no_import_graph_record_is_refused_not_guessed]] (calls)
- [[bench/tests/test_derive.py/test_re_deriving_replaces_rather_than_duplicates]] (calls)
- [[bench/tests/test_derive.py/test_a_derived_selection_only_ever_names_ids_the_commit_collected]] (calls)
- [[bench/tests/test_derive.py/test_a_changed_source_file_with_no_correspondence_selects_nothing]] (calls)

## Callers

- [[bench/replay/cli.py/cmd_derive]] (calls)

## Callees

- [[bench/replay/derive.py/DerivationError]] (calls)
- [[bench/replay/derive.py/level1_tests]] (calls)
- [[bench/replay/records.py/StrategyRecord]] (calls)
