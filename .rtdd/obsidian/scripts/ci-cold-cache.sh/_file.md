---
id: "scripts/ci-cold-cache.sh"
source: "scripts/ci-cold-cache.sh"
start: 1
end: 127
kind: "file"
is-test: false
tags: [rtdd/file]
---
# ci-cold-cache.sh

#rtdd/file

## Tests

- [[internal/contract/cold_cache_gate_test.go/TestColdCacheGateRunsEveryBenchCommandTheWorkflowRuns]] (references)
- [[internal/contract/cold_cache_gate_test.go/TestColdCacheGateProvesColdnessBeforeItRunsAnyBenchCommand]] (references)
- [[internal/contract/cold_cache_gate_test.go/TestColdCacheGateRefusesToPublishOrToDisturbTheWarmStore]] (references)

## Callers

- [[DEVELOPMENT.md/_file]] (references)

## Callees

- [[DEVELOPMENT.md/_file]] (references)
- [[bench/swebench/run_arm.py/_file]] (references)
- [[scripts/ci-prereg.sh/_file]] (references)
