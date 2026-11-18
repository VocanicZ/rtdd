---
id: "bench/swebench/tests/conftest.py::make_repo::build"
source: "bench/swebench/tests/conftest.py"
start: 43
end: 59
kind: "func"
is-test: false
tags: [rtdd/code]
---
# build

#rtdd/code

## Tests

- [[bench/swebench/tests/conftest.py/_file_]] (calls)
- [[bench/swebench/tests/test_prompts.py/test_rtdd_arm_is_vanilla_plus_one_context_block_byte_for_byte]] (calls)
- [[bench/swebench/tests/test_prompts.py/test_rtdd_tdd_arm_is_the_tdd_arm_plus_one_context_block]] (calls)
- [[bench/swebench/tests/test_prompts.py/test_tdad_arm_is_the_tdd_arm_plus_one_context_block]] (calls)
- [[bench/swebench/tests/test_prompts.py/test_the_identities_are_driven_by_control_for]] (calls)
- [[bench/swebench/tests/test_prompts.py/test_rtdd_arm_contains_no_tdd_prose]] (calls)
- [[bench/swebench/tests/test_prompts.py/test_the_tdd_arm_is_vanilla_plus_exactly_the_prose]] (calls)
- [[bench/swebench/tests/test_prompts.py/test_vanilla_is_exactly_base]] (calls)
- [[bench/swebench/tests/test_prompts.py/test_every_arm_is_base_plus_only_the_shared_parts]] (calls)
- [[bench/swebench/tests/test_prompts.py/test_each_context_arm_carries_exactly_one_block]] (calls)
- [[bench/swebench/tests/test_prompts.py/test_all_arms_build]] (calls)
- [[bench/swebench/tests/test_prompts.py/test_unknown_arm_is_an_error]] (calls)
- [[bench/swebench/tests/test_prompts.py/test_build_never_returns_a_default_prompt_for_a_near_miss]] (calls)
- [[bench/swebench/tests/test_prompts.py/test_an_extra_sentence_in_the_rtdd_arm_breaks_byte_equality]] (calls)
- [[bench/swebench/tests/test_prompts.py/test_extract_context_returns_the_block_of_every_context_arm]] (calls)
- [[bench/swebench/tests/test_prompts.py/test_extract_context_returns_none_for_a_control_arm]] (calls)
- [[bench/swebench/tests/test_prompts.py/test_two_blocks_are_an_error_rather_than_a_best_guess]] (calls)
- [[bench/swebench/tests/test_prompts.py/test_a_stray_delimiter_is_an_error]] (calls)

## Callers

- [[bench/replay/cache.py/Cache/json_or_build]] (calls)
- [[bench/replay/replay.py/_cached_selection/build_dict]] (calls)
- [[bench/replay/runner.py/cached_run]] (calls)
- [[bench/replay/session.py/run_drift]] (calls)

## Callees

- [[bench/PREREGISTRATION.md/_file]] (references)
- [[bench/swebench/instances.txt/_file]] (references)
- [[bench/swebench/tests/conftest.py/_git]] (calls)
