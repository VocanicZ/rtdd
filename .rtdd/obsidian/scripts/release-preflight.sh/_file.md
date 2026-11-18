---
id: "scripts/release-preflight.sh"
source: "scripts/release-preflight.sh"
start: 1
end: 198
kind: "file"
is-test: false
tags: [rtdd/file]
---
# release-preflight.sh

#rtdd/file

## Tests

- [[internal/contract/releasing_doc_test.go/TestReleasingDocNamesTheHumanDecisionsAndThePreflight]] (references)
- [[internal/installtest/readme_install_test.go/TestREADMEsCarryNoPlaceholders]] (references)
- [[internal/installtest/release_preflight_test.go/TestReleasePreflightReportsPlaceholdersAndMissingUvExplicitly]] (references)
- [[internal/installtest/release_preflight_test.go/TestReleasePreflightNeverMutatesRepositoryState]] (references)
- [[internal/installtest/release_preflight_test.go/TestReleasePreflightIsDocumentedInDevelopmentMd]] (references)
- [[internal/installtest/release_preflight_test.go/TestReleasePreflightOnThisRepoReportsFourPasses]] (references)
- [[internal/installtest/release_preflight_test.go/TestLinkageCheckEntrypointsNameTheTestsRealPackage]] (references)

## Callers

- [[DEVELOPMENT.md/_file]] (references)
- [[docs/RELEASING.md/_file]] (references)
- [[docs/plans/07-m7-release-readiness.md/_file]] (references)
- [[internal/installtest/release_preflight_test.go/buildFixtureRepo]] (references)
- [[internal/installtest/release_preflight_test.go/runPreflight]] (references)

## Callees

- [[DEVELOPMENT.md/_file]] (references)
- [[README.md/_file]] (references)
- [[bench/PREREGISTRATION.md/_file]] (references)
- [[bench/results/README.md/_file]] (references)
- [[bench/results/paired/flask/README.md/_file]] (references)
- [[bench/results/replay/tables.md/_file]] (references)
- [[bench/swebench/preflight.py/_file]] (references)
- [[docs/outcomes/README.md/_file]] (references)
- [[docs/outcomes/SELECTED/_file]] (references)
- [[protocol/PROTOCOL.md/_file]] (references)
