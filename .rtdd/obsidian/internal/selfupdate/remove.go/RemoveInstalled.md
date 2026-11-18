---
id: "internal/selfupdate/remove.go::RemoveInstalled"
source: "internal/selfupdate/remove.go"
start: 18
end: 30
kind: "func"
is-test: false
tags: [rtdd/code]
---
# RemoveInstalled

#rtdd/code

## Tests

- [[internal/selfupdate/remove_test.go/TestRemoveInstalledDeletesTheBinaryAndNamesIt]] (calls)
- [[internal/selfupdate/remove_test.go/TestRemoveInstalledReportsAnUnwritableDirectory]] (calls)
- [[internal/selfupdate/remove_test.go/TestRemoveInstalledOnAMissingBinaryIsNotAnError]] (calls)

## Callers

- [[cmd/rtdd/uninstall.go/cmdUninstall]] (calls)

## Callees

- [[internal/selfupdate/update.go/resolveTarget]] (calls)
