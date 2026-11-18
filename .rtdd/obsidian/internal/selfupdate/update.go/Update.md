---
id: "internal/selfupdate/update.go::Update"
source: "internal/selfupdate/update.go"
start: 65
end: 160
kind: "func"
is-test: false
tags: [rtdd/code]
---
# Update

#rtdd/code

## Tests

- [[internal/selfupdate/update_test.go/TestUpdateReplacesTheBinaryWhenTheReleaseIsNewer]] (calls)
- [[internal/selfupdate/update_test.go/TestUpdateLeavesTheBinaryExecutable]] (calls)
- [[internal/selfupdate/update_test.go/TestUpdateDoesNothingWhenAlreadyOnTheLatestRelease]] (calls)
- [[internal/selfupdate/update_test.go/TestUpdateRefusesAnArchiveWhoseChecksumDoesNotMatch]] (calls)
- [[internal/selfupdate/update_test.go/TestCheckOnlyReportsWithoutWriting]] (calls)
- [[internal/selfupdate/update_test.go/TestUpdateRefusesToGuessFromASourceBuild]] (calls)
- [[internal/selfupdate/update_test.go/TestAnExplicitTagOverridesTheComparison]] (calls)
- [[internal/selfupdate/update_test.go/TestUpdateReportsAnUnwritableDestinationWithoutTouchingIt]] (calls)

## Callers

none

## Callees

- [[internal/selfupdate/asset.go/archiveName]] (calls)
- [[internal/selfupdate/asset.go/binaryName]] (calls)
- [[internal/selfupdate/extract.go/extractBinary]] (calls)
- [[internal/selfupdate/update.go/resolveTarget]] (calls)
- [[internal/selfupdate/update.go/swap]] (calls)
- [[internal/selfupdate/update.go/latestTag]] (calls)
- [[internal/selfupdate/update.go/fetch]] (calls)
- [[internal/selfupdate/update.go/verify]] (calls)
- [[internal/selfupdate/version.go/compare]] (calls)
