---
id: "internal/graphify/load.go::Load"
source: "internal/graphify/load.go"
start: 53
end: 133
kind: "func"
is-test: false
tags: [rtdd/code]
---
# Load

#rtdd/code

## Tests

- [[internal/graphify/load_test.go/TestLoadKeepsCodeNodesAndDropsNonCodeRelations]] (calls)
- [[internal/graphify/load_test.go/TestLoadHonoursGraphifyPath]] (calls)
- [[internal/graphify/load_test.go/TestLoadAbsentIsErrAbsentAndMalformedIsAnError]] (calls)
- [[internal/graphify/load_test.go/TestLoadDropsEachNonCodeRelation]] (calls)
- [[internal/graphify/load_test.go/TestLoadWithoutManifestHasNilManifest]] (calls)
- [[internal/scan/resolve_test.go/TestACallPrefersTheDefinitionInAnImportedFile]] (calls)

## Callers

- [[internal/graphbuild/build.go/Build]] (calls)

## Callees

- [[internal/graph/graph.go/ParseRelation]] (calls)
- [[internal/graph/graph.go/Sort]] (calls)
- [[internal/graphify/load.go/nameOf]] (calls)
- [[internal/graphify/load.go/readManifest]] (calls)
