---
id: "bench/tests/test_replay.py::test_replay_over_the_synth_repo_reproduces_the_hand_computed_ground_truth"
source: "bench/tests/test_replay.py"
start: 104
end: 147
kind: "func"
is-test: true
tags: [rtdd/test]
---
# test_replay_over_the_synth_repo_reproduces_the_hand_computed_ground_truth

#rtdd/test

## Tests

none

## Callers

none

## Callees

- [[bench/replay/cache.py/Cache]] (calls)
- [[bench/replay/coldcache.py/probe]] (calls)
- [[bench/replay/config.py/RunConfig/digest]] (calls)
- [[bench/replay/hardware.py/probe]] (calls)
- [[bench/replay/records.py/CommitRecord/stratum]] (calls)
- [[bench/replay/replay.py/ReplayOptions]] (calls)
- [[bench/replay/replay.py/replay_repo]] (calls)
- [[bench/tests/synthrepo.py/SynthRepo/sha]] (calls)
- [[bench/tests/test_replay.py/_spec]] (calls)
- [[bench/tests/test_replay.py/_cfg]] (calls)
