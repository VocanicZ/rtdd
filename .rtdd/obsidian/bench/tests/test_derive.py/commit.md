---
id: "bench/tests/test_derive.py::commit"
source: "bench/tests/test_derive.py"
start: 36
end: 41
kind: "func"
is-test: false
tags: [rtdd/code]
---
# commit

#rtdd/code

## Tests

- [[bench/tests/test_derive.py/test_a_changed_test_file_contributes_its_own_collected_ids]] (calls)
- [[bench/tests/test_derive.py/test_a_non_python_change_contributes_nothing]] (calls)
- [[bench/tests/test_derive.py/test_level1_expands_a_resolved_file_to_every_id_it_collected]] (calls)
- [[bench/tests/test_derive.py/test_static_is_level1_united_with_the_committed_import_graph]] (calls)
- [[bench/tests/test_derive.py/test_a_derived_record_carries_no_measurement_and_says_so]] (calls)
- [[bench/tests/test_derive.py/test_the_selection_is_ordered_so_the_record_is_byte_stable]] (calls)
- [[bench/tests/test_derive.py/test_an_id_the_commit_never_collected_is_refused]] (calls)
- [[bench/tests/test_derive.py/test_a_commit_with_no_import_graph_record_is_refused_not_guessed]] (calls)
- [[bench/tests/test_derive.py/test_re_deriving_replaces_rather_than_duplicates]] (calls)
- [[bench/tests/test_derive.py/test_a_derived_selection_only_ever_names_ids_the_commit_collected]] (calls)
- [[bench/tests/test_derive.py/test_a_changed_source_file_with_no_correspondence_selects_nothing]] (calls)

## Callers

none

## Callees

- [[bench/replay/records.py/CommitRecord]] (calls)
