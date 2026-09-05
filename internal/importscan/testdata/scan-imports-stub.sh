#!/usr/bin/env bash
# A fixture scanner: answers from a canned graph, ignoring the request's targets so the
# test asserts the wire contract rather than a scanning algorithm.
cat >/dev/null
echo '{"src/auth/token.ts":{"src/auth/session.test.ts":1,"src/api/gateway.test.ts":3}}'
