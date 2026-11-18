---
id: "bench/replay/coldcache.py::findings"
source: "bench/replay/coldcache.py"
start: 103
end: 110
kind: "func"
is-test: false
tags: [rtdd/code]
---
# findings

#rtdd/code

## Tests

- [[bench/tests/test_coldcache.py/test_an_absent_store_is_cold_and_the_guard_does_not_create_it]] (calls)
- [[bench/tests/test_coldcache.py/test_an_empty_store_is_cold]] (calls)
- [[bench/tests/test_coldcache.py/test_a_store_with_entries_is_warm_and_the_finding_names_the_path_and_the_count]] (calls)
- [[bench/tests/test_coldcache.py/test_a_warm_HOME_is_a_finding_even_when_the_variable_points_somewhere_cold]] (calls)
- [[bench/tests/test_coldcache.py/test_both_stores_warm_reports_both]] (calls)

## Callers

- [[bench/replay/coldcache.py/main]] (calls)

## Callees

none
