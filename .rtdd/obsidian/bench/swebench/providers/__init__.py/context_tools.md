---
id: "bench/swebench/providers/__init__.py::context_tools"
source: "bench/swebench/providers/__init__.py"
start: 37
end: 59
kind: "func"
is-test: false
tags: [rtdd/code]
---
# context_tools

#rtdd/code

## Tests

- [[bench/swebench/tests/test_providers_rtdd.py/test_both_rtdd_arms_get_the_which_tool_and_share_one_seed_cache]] (calls)
- [[bench/swebench/tests/test_providers_rtdd.py/test_a_failed_seed_still_arms_the_instance]] (calls)
- [[bench/swebench/tests/test_providers_rtdd.py/test_an_arm_without_a_context_source_gets_nothing]] (calls)
- [[bench/swebench/tests/test_providers_tdad.py/test_the_tdad_arm_gets_the_impact_tool_and_records_the_pin]] (calls)
- [[bench/swebench/tests/test_providers_tdad.py/test_an_index_failure_is_raised_rather_than_silently_dropping_the_context]] (calls)
- [[bench/swebench/tests/test_providers_tdad.py/test_an_install_failure_is_raised_rather_than_silently_dropping_the_context]] (calls)
- [[bench/swebench/tests/test_providers_tdad.py/test_an_unknown_arm_is_rejected_rather_than_armed_with_nothing]] (calls)
- [[bench/swebench/tests/test_providers_tdad.py/test_the_caveat_is_reachable_from_the_provider]] (calls)

## Callers

- [[bench/swebench/run_arm.py/run_arm]] (calls)

## Callees

- [[bench/swebench/providers/__init__.py/UnknownArm]] (calls)
- [[bench/swebench/providers/rtdd.py/ensure_seed]] (calls)
- [[bench/swebench/providers/rtdd.py/install_seed]] (calls)
- [[bench/swebench/providers/rtdd.py/which_tool]] (calls)
- [[bench/swebench/providers/tdad.py/ensure_installed]] (calls)
- [[bench/swebench/providers/tdad.py/index]] (calls)
- [[bench/swebench/providers/tdad.py/impact_tool]] (calls)
