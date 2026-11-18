---
id: "internal/gitctx/changedset.go::ChangedSet"
source: "internal/gitctx/changedset.go"
start: 27
end: 153
kind: "func"
is-test: false
tags: [rtdd/code]
---
# ChangedSet

#rtdd/code

## Tests

- [[cmd/rtdd/changed_test.go/TestOnlyChangedGoCallsGitctxChangedSet]] (calls)
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
- [[internal/gitctx/parsehunks_test.go/TestChangedSetContentLineLookingLikeAFileHeader]] (calls)
- [[internal/gitctx/parsehunks_test.go/TestChangedSetQuotedPathsKeepTheirRanges]] (calls)
- [[internal/gitctx/parsehunks_test.go/TestChangedSetIgnoresDiffPrefixConfig]] (calls)
- [[internal/graphbuild/overlay_test.go/TestStaleSetSources]] (calls)

## Callers

- [[cmd/rtdd/changed.go/changedSet]] (calls)
- [[internal/graphbuild/build.go/changedSet]] (calls)

## Callees

- [[internal/gitctx/changedset.go/statusFromDiffCode]] (calls)
- [[internal/gitctx/changedset.go/statusFromPorcelain]] (calls)
- [[internal/gitctx/changedset.go/splitZ]] (calls)
- [[internal/gitctx/changedset.go/wholeFile]] (calls)
- [[internal/gitctx/changedset.go/parseHunks]] (calls)
- [[internal/gitctx/changedset.go/mergeRanges]] (calls)
- [[internal/gitctx/gitctx.go/git]] (calls)
