---
id: "internal/gitctx/helper_test.go::newRepo"
source: "internal/gitctx/helper_test.go"
start: 14
end: 25
kind: "func"
is-test: false
tags: [rtdd/code]
---
# newRepo

#rtdd/code

## Tests

- [[internal/gitctx/changedset_test.go/TestChangedSetIncludesUntrackedFiles]] (calls)
- [[internal/gitctx/changedset_test.go/TestChangedSetRetainsDeletions]] (calls)
- [[internal/gitctx/changedset_test.go/TestChangedSetHandlesRenames]] (calls)
- [[internal/gitctx/changedset_test.go/TestChangedSetLineRanges]] (calls)
- [[internal/gitctx/changedset_test.go/TestChangedSetUnionsStagedWorktreeAndUntracked]] (calls)
- [[internal/gitctx/changedset_test.go/TestChangedSetAgainstAnEarlierBase]] (calls)
- [[internal/gitctx/changedset_test.go/TestChangedSetIsSortedAndEmptyOnACleanTree]] (calls)
- [[internal/gitctx/changedset_test.go/TestChangedSetOnARepoWithNoCommits]] (calls)
- [[internal/gitctx/changedset_test.go/TestChangedSetDefaultBaseIsHEAD]] (calls)
- [[internal/gitctx/changedset_test.go/TestChangedSetUnknownBaseIsAnError]] (calls)
- [[internal/gitctx/changedset_test.go/TestChangedSetSkipsAnUntrackedNestedRepository]] (calls)
- [[internal/gitctx/gitctx_test.go/TestRepoRoot]] (calls)
- [[internal/gitctx/gitctx_test.go/TestHeadSHA]] (calls)
- [[internal/gitctx/gitctx_test.go/TestHeadSHAOnUnbornHeadIsAnError]] (calls)
- [[internal/gitctx/helper_test.go/_file_]] (calls)
- [[internal/gitctx/history_test.go/TestCommitDistance]] (calls)
- [[internal/gitctx/history_test.go/TestCommitDistanceUnreachableIsMinusOne]] (calls)
- [[internal/gitctx/history_test.go/TestCommitDistanceAfterAnAmendMakesTheOldShaUnreachable]] (calls)
- [[internal/gitctx/history_test.go/TestIsMergeCommit]] (calls)
- [[internal/gitctx/history_test.go/TestOlder]] (calls)
- [[internal/gitctx/history_test.go/TestOlderIsUsableAsAPlainComparator]] (calls)
- [[internal/gitctx/listfiles_test.go/TestListFilesIsTrackedPlusUntrackedMinusIgnoredAndDeleted]] (calls)
- [[internal/gitctx/parsehunks_test.go/TestChangedSetContentLineLookingLikeAFileHeader]] (calls)
- [[internal/gitctx/parsehunks_test.go/TestChangedSetQuotedPathsKeepTheirRanges]] (calls)
- [[internal/gitctx/parsehunks_test.go/TestChangedSetIgnoresDiffPrefixConfig]] (calls)
- [[internal/gitctx/rawdiff_test.go/TestRawDiff]] (calls)
- [[internal/gitctx/rawdiff_test.go/TestRawDiffCoversUnstagedWorkingTreeEdits]] (calls)
- [[internal/gitctx/rawdiff_test.go/TestRawDiffDetectsRenamesOnTheNewPath]] (calls)
- [[internal/gitctx/rawdiff_test.go/TestRawDiffPathsAreNeverQuotedOrReprefixed]] (calls)
- [[internal/gitctx/rawdiff_test.go/TestRawDiffEmptyBaseMeansHEAD]] (calls)
- [[internal/gitctx/rawdiff_test.go/TestRawDiffWrapsGitErrors]] (calls)

## Callers

none

## Callees

- [[internal/gitctx/helper_test.go/gitRun]] (calls)
