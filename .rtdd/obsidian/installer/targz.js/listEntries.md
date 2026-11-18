---
id: "installer/targz.js::listEntries"
source: "installer/targz.js"
start: 47
end: 76
kind: "func"
is-test: false
tags: [rtdd/code]
---
# listEntries

#rtdd/code

## Tests

- [[installer/installer.test.js/listEntries stops at the end-of-archive marker rather than reading padding]] (calls)

## Callers

- [[installer/targz.js/extractFile]] (calls)

## Callees

- [[installer/targz.js/readString]] (calls)
- [[installer/targz.js/readOctal]] (calls)
- [[installer/targz.js/isZeroBlock]] (calls)
