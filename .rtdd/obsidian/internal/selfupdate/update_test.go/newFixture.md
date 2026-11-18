---
id: "internal/selfupdate/update_test.go::newFixture"
source: "internal/selfupdate/update_test.go"
start: 39
end: 68
kind: "func"
is-test: false
tags: [rtdd/code]
---
# newFixture

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
- [[internal/selfupdate/update_test.go/archiveBytes]] (calls)
- [[internal/selfupdate/update_test.go/buildArchive]] (calls)
