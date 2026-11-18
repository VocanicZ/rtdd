---
id: "cmd/rtdd-gen/main.go::findRepoRoot"
source: "cmd/rtdd-gen/main.go"
start: 59
end: 61
kind: "func"
is-test: false
tags: [rtdd/code]
---
# findRepoRoot

#rtdd/code

## Tests

- [[cmd/rtdd-gen/main_test.go/TestFindRepoRootRejectsAnEmptyGitDirectory]] (calls)
- [[cmd/rtdd-gen/main_test.go/TestFindRepoRootRejectsAGitDirectoryWithoutHEAD]] (calls)
- [[cmd/rtdd-gen/main_test.go/TestFindRepoRootRejectsAWorktreeFileWithADeadGitdir]] (calls)
- [[cmd/rtdd-gen/main_test.go/TestFindRepoRootAcceptsAWorktreeFile]] (calls)

## Callers

- [[cmd/rtdd-gen/main.go/loadDoc]] (calls)

## Callees

- [[internal/gitctx/reporoot.go/FindRepoRoot]] (calls)
