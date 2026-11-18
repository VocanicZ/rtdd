---
id: "internal/gitctx/gitctx.go::git"
source: "internal/gitctx/gitctx.go"
start: 50
end: 61
kind: "func"
is-test: false
tags: [rtdd/code]
---
# git

#rtdd/code

## Tests

- [[internal/contract/contract_test.go/TestEveryRepoScriptIsExecutable]] (calls)
- [[internal/scan/refs_test.go/TestPathTokensAreTheRunsThatMayNameAFile]] (calls)

## Callers

- [[internal/gitctx/changedset.go/ChangedSet]] (calls)
- [[internal/gitctx/gitctx.go/RepoRoot]] (calls)
- [[internal/gitctx/gitctx.go/HeadSHA]] (calls)
- [[internal/gitctx/graphgit.go/DiffNamesSince]] (calls)
- [[internal/gitctx/graphgit.go/BlobIDs]] (calls)
- [[internal/gitctx/graphgit.go/CommitKnown]] (calls)
- [[internal/gitctx/history.go/CommitDistance]] (calls)
- [[internal/gitctx/history.go/IsMergeCommit]] (calls)
- [[internal/gitctx/history.go/reachable]] (calls)
- [[internal/gitctx/listfiles.go/ListFiles]] (calls)
- [[internal/gitctx/rawdiff.go/RawDiff]] (calls)

## Callees

- [[internal/gitctx/gitctx.go/String]] (calls)
