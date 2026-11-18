---
id: "bench/tests/test_falsesignal.py::_rec"
source: "bench/tests/test_falsesignal.py"
start: 19
end: 20
kind: "func"
is-test: false
tags: [rtdd/code]
---
# _rec

#rtdd/code

## Tests

- [[bench/tests/test_falsesignal.py/test_record_counts_hits_against_truth]] (calls)
- [[bench/tests/test_falsesignal.py/test_reported_pairs_are_sorted_so_the_record_line_is_byte_stable]] (calls)
- [[bench/tests/test_falsesignal.py/test_change_level_false_signal_requires_every_reported_line_to_be_tested]] (calls)
- [[bench/tests/test_falsesignal.py/test_line_level_false_signal_rate]] (calls)
- [[bench/tests/test_falsesignal.py/test_silent_commits_are_excluded_from_the_rate_but_counted_in_fire_rate]] (calls)
- [[bench/tests/test_falsesignal.py/test_a_rate_with_no_fired_commit_has_no_value_rather_than_a_zero]] (calls)
- [[bench/tests/test_falsesignal.py/test_summarise_reports_fired_silent_and_all_three_rates]] (calls)
- [[bench/tests/test_falsesignal.py/test_every_published_rate_refuses_to_pool_across_repos]] (calls)

## Callers

none

## Callees

- [[bench/replay/falsesignal.py/build_record]] (calls)
