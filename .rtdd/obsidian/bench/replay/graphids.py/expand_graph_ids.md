---
id: "bench/replay/graphids.py::expand_graph_ids"
source: "bench/replay/graphids.py"
start: 38
end: 60
kind: "func"
is-test: false
tags: [rtdd/code]
---
# expand_graph_ids

#rtdd/code

## Tests

- [[bench/tests/test_graph_ids.py/test_graph_ids_expand_to_collected_ids]] (calls)
- [[bench/tests/test_graph_ids.py/test_the_file_is_part_of_the_key]] (calls)
- [[bench/tests/test_graph_ids.py/test_the_duplicate_definition_suffix_still_matches]] (calls)
- [[bench/tests/test_graph_ids.py/test_a_helper_in_a_test_file_matches_no_collected_id]] (calls)
- [[bench/tests/test_graph_ids.py/test_a_graph_id_never_widens_to_its_whole_file]] (calls)
- [[bench/tests/test_graph_ids.py/test_output_is_deduplicated_in_first_seen_order]] (calls)

## Callers

- [[bench/replay/strategies/rtdd.py/Rtdd/select]] (calls)

## Callees

- [[bench/replay/graphids.py/_collected_key]] (calls)
