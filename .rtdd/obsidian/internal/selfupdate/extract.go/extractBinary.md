---
id: "internal/selfupdate/extract.go::extractBinary"
source: "internal/selfupdate/extract.go"
start: 23
end: 32
kind: "func"
is-test: false
tags: [rtdd/code]
---
# extractBinary

#rtdd/code

## Tests

- [[internal/selfupdate/extract_test.go/TestExtractBinaryTakesOnlyTheBinaryFromATarball]] (calls)
- [[internal/selfupdate/extract_test.go/TestExtractBinaryReadsAZipForWindows]] (calls)
- [[internal/selfupdate/extract_test.go/TestExtractBinaryRefusesAnArchiveWithoutTheBinary]] (calls)
- [[internal/selfupdate/extract_test.go/TestExtractBinaryRefusesAnArchiveInTheWrongFormat]] (calls)

## Callers

- [[internal/selfupdate/update.go/Update]] (calls)

## Callees

- [[internal/selfupdate/extract.go/fromTarGz]] (calls)
- [[internal/selfupdate/extract.go/fromZip]] (calls)
