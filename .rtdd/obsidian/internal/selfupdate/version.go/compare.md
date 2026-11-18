---
id: "internal/selfupdate/version.go::compare"
source: "internal/selfupdate/version.go"
start: 50
end: 76
kind: "func"
is-test: false
tags: [rtdd/code]
---
# compare

#rtdd/code

## Tests

- [[internal/selfupdate/version_test.go/TestCompareOrdersReleaseVersions]] (calls)
- [[internal/selfupdate/version_test.go/TestCompareRefusesVersionsItCannotOrder]] (calls)

## Callers

- [[internal/selfupdate/update.go/Update]] (calls)

## Callees

- [[internal/selfupdate/version.go/parseVersion]] (calls)
