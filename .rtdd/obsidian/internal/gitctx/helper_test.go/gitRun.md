---
id: "internal/gitctx/helper_test.go::gitRun"
source: "internal/gitctx/helper_test.go"
start: 47
end: 50
kind: "func"
is-test: false
tags: [rtdd/code]
---
# gitRun

#rtdd/code

## Tests

- [[internal/gitctx/changedset_test.go/TestChangedSetHandlesRenames]] (calls)
- [[internal/gitctx/changedset_test.go/TestChangedSetUnionsStagedWorktreeAndUntracked]] (calls)
- [[internal/gitctx/changedset_test.go/TestChangedSetSkipsAnUntrackedNestedRepository]] (calls)
- [[internal/gitctx/helper_test.go/_file_]] (calls)
- [[internal/gitctx/history_test.go/TestCommitDistanceAfterAnAmendMakesTheOldShaUnreachable]] (calls)
- [[internal/gitctx/history_test.go/TestIsMergeCommit]] (calls)
- [[internal/gitctx/parsehunks_test.go/TestChangedSetQuotedPathsKeepTheirRanges]] (calls)
- [[internal/gitctx/parsehunks_test.go/TestChangedSetIgnoresDiffPrefixConfig]] (calls)
- [[internal/gitctx/rawdiff_test.go/TestRawDiffCoversUnstagedWorkingTreeEdits]] (calls)
- [[internal/gitctx/rawdiff_test.go/TestRawDiffDetectsRenamesOnTheNewPath]] (calls)
- [[internal/gitctx/rawdiff_test.go/TestRawDiffPathsAreNeverQuotedOrReprefixed]] (calls)

## Callers

- [[internal/gitctx/helper_test.go/newRepo]] (calls)
- [[internal/gitctx/helper_test.go/commit]] (calls)

## Callees

- [[internal/gitctx/helper_test.go/run]] (calls)
