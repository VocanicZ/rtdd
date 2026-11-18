---
id: "cmd/rtdd/graphenv.go::graphRoot"
source: "cmd/rtdd/graphenv.go"
start: 14
end: 24
kind: "func"
is-test: false
tags: [rtdd/code]
---
# graphRoot

#rtdd/code

## Tests

- [[cmd/rtdd/obsidian_test.go/TestObsidianNoOpenWritesOneNotePerNode]] (calls)
- [[cmd/rtdd/obsidian_test.go/TestObsidianRerunRemovesStaleNotesKeepsSettings]] (calls)
- [[cmd/rtdd/obsidian_test.go/TestObsidianNotePathsAreSafeAndUnique/TestObsidianRerunRewritesOnlyChangedNotes]] (calls)

## Callers

- [[cmd/rtdd/doctor.go/cmdDoctor]] (calls)
- [[cmd/rtdd/explain.go/cmdExplain]] (calls)
- [[cmd/rtdd/obsidian.go/cmdObsidian]] (calls)
- [[cmd/rtdd/which.go/cmdWhich]] (calls)

## Callees

- [[internal/gitctx/gitctx.go/RepoRoot]] (calls)
