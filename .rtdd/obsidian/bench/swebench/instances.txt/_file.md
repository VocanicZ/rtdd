---
id: "bench/swebench/instances.txt"
source: "bench/swebench/instances.txt"
start: 1
end: 100
kind: "file"
is-test: false
tags: [rtdd/file]
---
# instances.txt

#rtdd/file

## Tests

- [[bench/swebench/tests/test_acceptance.py/test_the_shipped_preflight_cli_refuses_naming_the_missing_kill_criterion]] (references)
- [[bench/swebench/tests/test_acceptance.py/test_every_arm_refuses_from_the_command_line_before_any_model_call]] (references)
- [[bench/swebench/tests/test_acceptance.py/test_the_five_refusal_cases_each_get_their_own_informative_message]] (references)
- [[bench/swebench/tests/test_acceptance.py/test_a_signed_tagged_and_matching_pre_registration_is_the_one_state_that_passes]] (references)
- [[bench/swebench/tests/test_acceptance.py/test_the_dry_run_cli_builds_five_arms_over_the_frozen_list_with_no_network]] (references)
- [[bench/swebench/tests/test_acceptance.py/test_the_dry_run_prints_the_real_repository_root_the_same_for_every_arm]] (references)
- [[bench/swebench/tests/test_acceptance.py/test_the_dry_run_needs_no_signed_pre_registration_so_ci_can_run_it_here]] (references)
- [[bench/swebench/tests/test_acceptance.py/test_procedural_prose_inside_the_context_block_fails_the_imperative_lint]] (references)
- [[bench/swebench/tests/test_acceptance.py/test_procedural_prose_after_the_context_block_fails_the_byte_equality]] (references)
- [[bench/swebench/tests/test_acceptance.py/test_the_failing_fixtures_make_the_shipped_dry_run_cli_exit_non_zero]] (references)
- [[bench/swebench/tests/test_acceptance.py/test_the_pipeline_runs_end_to_end_from_raw_records_to_a_rendered_table]] (references)
- [[bench/swebench/tests/test_acceptance.py/test_every_published_regression_traces_to_pass_to_pass_failures]] (references)
- [[bench/swebench/tests/test_acceptance.py/test_the_denominator_comes_from_the_dataset_not_from_the_report]] (references)
- [[bench/swebench/tests/test_acceptance.py/test_the_rendered_table_carries_the_traced_numbers]] (references)
- [[bench/swebench/tests/test_acceptance.py/test_the_generated_table_carries_a_resolution_rate_in_every_row]] (references)
- [[bench/swebench/tests/test_acceptance.py/test_the_shipped_pre_registration_is_complete_except_the_humans_number]] (references)
- [[bench/swebench/tests/test_acceptance.py/test_the_pre_registration_pins_both_reference_implementations]] (references)
- [[bench/swebench/tests/test_acceptance.py/test_the_frozen_instance_list_is_the_one_the_pre_registration_describes]] (references)
- [[bench/swebench/tests/test_acceptance.py/test_one_ci_run_covers_the_suite_the_refusal_the_dry_run_and_the_lint]] (references)
- [[bench/swebench/tests/test_acceptance.py/test_the_docs_say_how_to_reproduce_the_harness_and_what_should_refuse]] (references)
- [[bench/swebench/tests/test_acceptance.py/test_the_docs_scope_the_human_in_the_loop_tasks_out_of_this_prd]] (references)
- [[bench/swebench/tests/test_preflight.py/test_missing_instance_list_is_a_clean_refusal]] (references)
- [[bench/swebench/tests/test_report.py/test_config_records_the_sample_seed_size_and_instance_list_sha256]] (references)
- [[bench/swebench/tests/test_report.py/test_the_published_digest_is_the_one_the_gate_checked]] (references)
- [[bench/swebench/tests/test_sample.py/test_empty_p2p_is_ineligible]] (references)
- [[bench/swebench/tests/test_sample.py/test_eligible_is_sorted]] (references)
- [[bench/swebench/tests/test_sample.py/test_eligible_accepts_a_list_valued_p2p_column]] (references)
- [[bench/swebench/tests/test_sample.py/test_draw_is_deterministic]] (references)
- [[bench/swebench/tests/test_sample.py/test_draw_changes_with_seed]] (references)
- [[bench/swebench/tests/test_sample.py/test_draw_never_returns_an_ineligible_instance]] (references)
- [[bench/swebench/tests/test_sample.py/test_draw_refuses_when_the_eligible_pool_is_too_small]] (references)
- [[bench/swebench/tests/test_sample.py/test_sha_is_stable_and_newline_terminated]] (references)
- [[bench/swebench/tests/test_sample.py/test_dataset_is_swebench_verified]] (references)
- [[bench/swebench/tests/test_sample.py/test_load_rows_keys_by_instance_id]] (references)
- [[bench/swebench/tests/test_sample.py/test_frozen_file_is_one_hundred_ids_one_per_line]] (references)
- [[bench/swebench/tests/test_sample.py/test_frozen_file_is_one_hundred_ids_one_per_line]] (references)
- [[bench/swebench/tests/test_sample.py/test_frozen_file_is_sorted_and_free_of_duplicates]] (references)
- [[bench/swebench/tests/test_sample.py/test_frozen_file_sha_is_reproducible_from_the_file_alone]] (references)
- [[bench/swebench/tests/test_sample.py/test_frozen_file_sha_is_reproducible_from_the_file_alone]] (references)
- [[bench/swebench/tests/test_sample.py/test_prereg_carries_the_frozen_files_sha]] (references)
- [[bench/swebench/tests/test_sample.py/test_preflight_passes_on_the_frozen_file]] (references)
- [[bench/swebench/tests/test_sample.py/test_preflight_refuses_when_one_line_of_the_frozen_file_is_edited]] (references)

## Callers

- [[DEVELOPMENT.md/_file]] (references)
- [[bench/PREREGISTRATION.md/_file]] (references)
- [[bench/swebench/evaluate.py/instance_ids]] (references)
- [[bench/swebench/preflight.py/preflight]] (references)
- [[bench/swebench/report.py/write_config]] (references)
- [[bench/swebench/run_arm.py/instance_ids]] (references)
- [[bench/swebench/sample.py/_file]] (references)
- [[bench/swebench/sample.py/sha256_ids]] (references)
- [[bench/swebench/tests/conftest.py/make_repo/build]] (references)
- [[bench/swebench/tests/test_acceptance.py/build_repo]] (references)
- [[bench/swebench/tests/test_analyze.py/make_repo]] (references)
- [[bench/swebench/tests/test_evaluate.py/make_repo]] (references)
- [[bench/swebench/tests/test_preflight.py/make_repo]] (references)
- [[bench/swebench/tests/test_report.py/make_repo]] (references)
- [[bench/swebench/tests/test_run_arm.py/test_the_arm_never_re_draws_its_own_sample/explode]] (references)
- [[bench/swebench/tests/test_sample.py/make_repo]] (references)
- [[docs/bench/swebench-harness.md/_file]] (references)
- [[docs/plans/05-m4-m5-swebench-release.md/_file]] (references)

## Callees

none
