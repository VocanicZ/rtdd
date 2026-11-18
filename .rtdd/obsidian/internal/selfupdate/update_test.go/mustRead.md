---
id: "internal/selfupdate/update_test.go::mustRead"
source: "internal/selfupdate/update_test.go"
start: 111
end: 118
kind: "func"
is-test: false
tags: [rtdd/code]
---
# mustRead

#rtdd/code

## Tests

- [[internal/selfupdate/update_test.go/TestUpdateReplacesTheBinaryWhenTheReleaseIsNewer]] (calls)
- [[internal/selfupdate/update_test.go/TestUpdateDoesNothingWhenAlreadyOnTheLatestRelease]] (calls)
- [[internal/selfupdate/update_test.go/TestUpdateRefusesAnArchiveWhoseChecksumDoesNotMatch]] (calls)
- [[internal/selfupdate/update_test.go/TestCheckOnlyReportsWithoutWriting]] (calls)
- [[internal/selfupdate/update_test.go/TestUpdateRefusesToGuessFromASourceBuild]] (calls)
- [[internal/selfupdate/update_test.go/TestAnExplicitTagOverridesTheComparison]] (calls)
- [[internal/selfupdate/update_test.go/TestUpdateReportsAnUnwritableDestinationWithoutTouchingIt]] (calls)

## Callers

none

## Callees

none
