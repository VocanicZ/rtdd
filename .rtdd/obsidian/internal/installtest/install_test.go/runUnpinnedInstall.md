---
id: "internal/installtest/install_test.go::runUnpinnedInstall"
source: "internal/installtest/install_test.go"
start: 220
end: 236
kind: "func"
is-test: false
tags: [rtdd/code]
---
# runUnpinnedInstall

#rtdd/code

## Tests

- [[internal/installtest/install_test.go/TestInstallResolvesTheLatestReleaseWhenVersionIsUnset]] (calls)
- [[internal/installtest/install_test.go/TestInstallFailsWhenTheLatestReleaseCannotBeResolved]] (calls)
- [[internal/installtest/install_test.go/TestInstallResolvesATagRegardlessOfJSONWhitespace]] (calls)

## Callers

none

## Callees

- [[internal/installtest/install_test.go/archiveName]] (calls)
- [[internal/installtest/install_test.go/buildFixtureBinary]] (calls)
- [[internal/installtest/install_test.go/buildArchive]] (calls)
- [[internal/installtest/install_test.go/sha256Hex]] (calls)
- [[internal/installtest/install_test.go/serveFixture]] (calls)
- [[internal/installtest/install_test.go/runInstaller]] (calls)
- [[internal/installtest/install_test.go/fixtureOSArch]] (calls)
