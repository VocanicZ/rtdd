---
id: "bench/swebench/budget.py::Budget"
source: "bench/swebench/budget.py"
start: 32
end: 109
kind: "class"
is-test: false
tags: [rtdd/code]
---
# Budget

#rtdd/code

## Tests

- [[bench/swebench/tests/test_budget.py/test_usd_is_the_two_per_million_prices_applied_to_the_two_counts]] (calls)
- [[bench/swebench/tests/test_budget.py/test_add_accumulates_across_calls]] (calls)
- [[bench/swebench/tests/test_budget.py/test_add_raises_when_the_spend_cap_is_crossed]] (calls)
- [[bench/swebench/tests/test_budget.py/test_the_tokens_that_crossed_the_cap_are_still_counted]] (calls)
- [[bench/swebench/tests/test_budget.py/test_add_raises_when_the_wall_clock_cap_is_crossed]] (calls)
- [[bench/swebench/tests/test_budget.py/test_snapshot_records_the_assumed_prices_beside_the_actual_counts]] (calls)
- [[bench/swebench/tests/test_budget.py/test_snapshot_is_json_serialisable]] (calls)
- [[bench/swebench/tests/test_budget.py/test_restore_adopts_a_previous_runs_spend_so_a_resume_tops_it_up]] (calls)
- [[bench/swebench/tests/test_budget.py/test_restore_carries_the_previous_runs_wall_clock]] (calls)
- [[bench/swebench/tests/test_budget.py/test_a_restored_budget_snapshots_the_whole_benchmarks_spend]] (calls)
- [[bench/swebench/tests/test_budget.py/test_restore_of_an_empty_snapshot_changes_nothing]] (calls)
- [[bench/swebench/tests/test_budget.py/test_check_refuses_a_resume_that_is_already_over_the_cap]] (calls)
- [[bench/swebench/tests/test_budget.py/test_check_is_silent_while_both_ceilings_hold]] (calls)
- [[bench/swebench/tests/test_run_arm.py/test_an_unsigned_preregistration_is_refused_before_a_single_token]] (calls)
- [[bench/swebench/tests/test_run_arm.py/test_an_untagged_preregistration_is_refused_before_a_single_token]] (calls)
- [[bench/swebench/tests/test_run_arm.py/test_an_arm_outside_the_preregistered_five_is_refused]] (calls)
- [[bench/swebench/tests/test_run_arm.py/test_instances_come_from_the_frozen_list_in_file_order]] (calls)
- [[bench/swebench/tests/test_run_arm.py/test_the_arm_never_re_draws_its_own_sample]] (calls)
- [[bench/swebench/tests/test_run_arm.py/test_the_record_carries_the_arm_the_model_and_the_provider_provenance]] (calls)
- [[bench/swebench/tests/test_run_arm.py/test_the_prompt_is_the_arm_prompt_with_the_instance_substituted]] (calls)
- [[bench/swebench/tests/test_run_arm.py/test_an_instance_with_a_record_is_skipped_so_a_crashed_run_resumes]] (calls)
- [[bench/swebench/tests/test_run_arm.py/test_a_second_run_with_every_record_present_spends_nothing_new]] (calls)
- [[bench/swebench/tests/test_run_arm.py/test_arms_do_not_share_a_raw_directory]] (calls)
- [[bench/swebench/tests/test_run_arm.py/test_a_failed_instance_is_recorded_as_failed_not_skipped]] (calls)
- [[bench/swebench/tests/test_run_arm.py/test_an_empty_patch_is_a_prediction_not_an_error]] (calls)
- [[bench/swebench/tests/test_run_arm.py/test_a_clean_run_writes_the_cost_file]] (calls)
- [[bench/swebench/tests/test_run_arm.py/test_crossing_the_spend_cap_stops_the_arm_and_writes_the_cost_file]] (calls)
- [[bench/swebench/tests/test_run_arm.py/test_the_instance_that_crossed_the_cap_keeps_its_record]] (calls)
- [[bench/swebench/tests/test_run_arm.py/test_the_dry_run_assembles_against_the_root_a_real_run_uses]] (calls)
- [[bench/swebench/tests/test_run_arm.py/test_two_arms_hand_the_model_prompts_that_differ_by_exactly_the_block]] (calls)
- [[bench/swebench/tests/test_run_arm.py/test_a_composition_violation_stops_the_arm_instead_of_being_recorded]] (calls)
- [[bench/swebench/tests/test_run_arm.py/test_a_resume_tops_up_the_recorded_spend_instead_of_erasing_it]] (calls)
- [[bench/swebench/tests/test_run_arm.py/test_the_spend_cap_is_a_per_benchmark_ceiling_not_a_per_invocation_one]] (calls)

## Callers

- [[bench/swebench/run_arm.py/main]] (calls)

## Callees

- [[bench/swebench/budget.py/Budget/start]] (method)
- [[bench/swebench/budget.py/Budget/restore]] (method)
- [[bench/swebench/budget.py/Budget/check]] (method)
- [[bench/swebench/budget.py/Budget/add]] (method)
- [[bench/swebench/budget.py/Budget/usd]] (method)
- [[bench/swebench/budget.py/Budget/wall_hours]] (method)
- [[bench/swebench/budget.py/Budget/snapshot]] (method)
