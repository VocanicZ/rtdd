---
id: "internal/selfupdate/update_test.go::installedBinary"
source: "internal/selfupdate/update_test.go"
start: 102
end: 109
kind: "func"
is-test: false
tags: [rtdd/code]
---
# installedBinary

#rtdd/code

## Tests

- [[internal/selfupdate/remove_test.go/TestRemoveInstalledDeletesTheBinaryAndNamesIt]] (calls)
- [[internal/selfupdate/remove_test.go/TestRemoveInstalledReportsAnUnwritableDirectory]] (calls)
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

- [[internal/selfupdate/asset.go/binaryName]] (calls)
