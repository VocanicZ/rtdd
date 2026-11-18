---
id: "internal/gitctx/reporoot.go::FindRepoRoot"
source: "internal/gitctx/reporoot.go"
start: 32
end: 47
kind: "func"
is-test: false
tags: [rtdd/code]
---
# FindRepoRoot

#rtdd/code

## Tests

- [[internal/gitctx/reporoot_test.go/TestFindRepoRootAgreesWithGitOnARealRepository]] (calls)
- [[internal/gitctx/reporoot_test.go/TestFindRepoRootIgnoresAStrayGitAncestor]] (calls)

## Callers

- [[cmd/rtdd-gen/main.go/findRepoRoot]] (calls)
- [[cmd/rtdd/init.go/findRepoRoot]] (calls)

## Callees

- [[internal/gitctx/reporoot.go/IsRepoRoot]] (calls)
