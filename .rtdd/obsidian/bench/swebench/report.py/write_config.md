---
id: "bench/swebench/report.py::write_config"
source: "bench/swebench/report.py"
start: 269
end: 321
kind: "func"
is-test: false
tags: [rtdd/code]
---
# write_config

#rtdd/code

## Tests

- [[bench/swebench/tests/test_report.py/test_write_config_writes_into_the_swebench_results_directory]] (calls)
- [[bench/swebench/tests/test_report.py/test_config_refuses_a_model_that_is_not_the_pre_registered_one]] (calls)
- [[bench/swebench/tests/test_report.py/test_the_model_guard_reads_the_records_not_the_agent_default]] (calls)
- [[bench/swebench/tests/test_report.py/test_a_run_whose_records_name_two_models_is_refused]] (calls)

## Callers

- [[bench/swebench/report.py/main]] (calls)
- [[bench/swebench/tests/test_report.py/config_of]] (calls)

## Callees

- [[bench/results/flask/config.json/_file]] (references)
- [[bench/results/httpie/config.json/_file]] (references)
- [[bench/results/i508/flask/config.json/_file]] (references)
- [[bench/results/n4/flask/config.json/_file]] (references)
- [[bench/results/n4/httpie/config.json/_file]] (references)
- [[bench/results/paired/flask/config.json/_file]] (references)
- [[bench/results/sqlfluff/config.json/_file]] (references)
- [[bench/results/swebench/config.json/_file]] (references)
- [[bench/swebench/agent.py/AgentConfig]] (calls)
- [[bench/swebench/instances.txt/_file]] (references)
- [[bench/swebench/preflight.py/instance_list_sha256]] (calls)
- [[bench/swebench/preflight.py/preflight]] (calls)
- [[bench/swebench/report.py/ReportError]] (calls)
- [[bench/swebench/report.py/seed_counts]] (calls)
- [[bench/swebench/report.py/observed_models]] (calls)
- [[bench/swebench/report.py/_git]] (calls)
