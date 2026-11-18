---
id: "internal/gitctx/rawdiff.go::RawDiff"
source: "internal/gitctx/rawdiff.go"
start: 15
end: 26
kind: "func"
is-test: false
tags: [rtdd/code]
---
# RawDiff

#rtdd/code

## Tests

- [[internal/contract/contract_test.go/TestInterfaceContractCarriesEveryM2Addition]] (calls)
- [[internal/gitctx/rawdiff_test.go/TestRawDiff]] (calls)
- [[internal/gitctx/rawdiff_test.go/TestRawDiffCoversUnstagedWorkingTreeEdits]] (calls)
- [[internal/gitctx/rawdiff_test.go/TestRawDiffDetectsRenamesOnTheNewPath]] (calls)
- [[internal/gitctx/rawdiff_test.go/TestRawDiffPathsAreNeverQuotedOrReprefixed]] (calls)
- [[internal/gitctx/rawdiff_test.go/TestRawDiffEmptyBaseMeansHEAD]] (calls)
- [[internal/gitctx/rawdiff_test.go/TestRawDiffWrapsGitErrors]] (calls)

## Callers

none

## Callees

- [[internal/gitctx/gitctx.go/git]] (calls)
