---
id: "cmd/rtdd/graphenv.go::buildGraph"
source: "cmd/rtdd/graphenv.go"
start: 32
end: 41
kind: "func"
is-test: false
tags: [rtdd/code]
---
# buildGraph

#rtdd/code

## Tests

- [[cmd/rtdd/obsidian_test.go/TestObsidianNoOpenWritesOneNotePerNode]] (calls)

## Callers

- [[cmd/rtdd/doctor.go/cmdDoctor]] (calls)
- [[cmd/rtdd/explain.go/cmdExplain]] (calls)
- [[cmd/rtdd/obsidian.go/cmdObsidian]] (calls)
- [[cmd/rtdd/which.go/cmdWhich]] (calls)

## Callees

- [[cmd/rtdd/event.go/recordGraph]] (calls)
- [[internal/graph/config.go/LoadConfig]] (calls)
- [[internal/graphbuild/build.go/Build]] (calls)
