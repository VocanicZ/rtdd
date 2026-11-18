---
id: "cmd/rtdd/swarm.go::writeSwarmJob"
source: "cmd/rtdd/swarm.go"
start: 99
end: 143
kind: "func"
is-test: false
tags: [rtdd/code]
---
# writeSwarmJob

#rtdd/code

## Tests

- [[cmd/rtdd/which_swarm_test.go/TestWriteSwarmJobRefusesAPathWithATab]] (calls)
- [[cmd/rtdd/which_swarm_test.go/TestWriteSwarmJobErrors]] (calls)

## Callers

- [[cmd/rtdd/which.go/emitSwarmJob]] (calls)

## Callees

- [[cmd/rtdd/main_test.go/rtdd]] (calls)
- [[cmd/rtdd/swarm.go/swarmShardIDs]] (calls)
- [[cmd/rtdd/swarm.go/swarmJobName]] (calls)
- [[cmd/rtdd/swarm.go/shQuote]] (calls)
- [[cmd/rtdd/swarm.go/tomlString]] (calls)
