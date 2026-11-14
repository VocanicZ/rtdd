package scan

import (
	"testing"

	"github.com/VocanicZ/rtdd/internal/graph"
)

// #572: a " that nothing closes on its line (here after a multi-line python3 -c '...'
// program) must not open a string that swallows the function's closing brace, or the
// node runs to the end of the file and every later function nests under it.
func TestShellUnclosedDoubleQuoteKeepsTheClosingBrace(t *testing.T) {
	src := `off(){ local d ids="" k
  while IFS= read -r d; do
    k="$(F="$d/p.json" python3 -c 'import json, os
for k in (json.load(open(os.environ["F"])).get("plugins") or {}):
    if k.split("@")[0] == "ralph-loop":
        print(k)' 2>/dev/null)"
    [[ -n "$k" ]] && ids="$ids"$'\n'"$k"
  done < <(dirs)
  [[ -n "${ids//[$'\n']/}" ]] || return 0
  printf -- '--settings=%q' "$(printf '%s\n' "$ids" | sort -u | python3 -c 'import json, sys
print(json.dumps({"enabledPlugins": {k.strip(): False for k in sys.stdin if k.strip()}}, separators=(",", ":")))')"; }
lean(){ echo lean; }
`
	checkSpans(t, "c.sh", src, []span{
		{"c.sh::off", graph.KindFunc, 1, 11},
		{"c.sh::lean", graph.KindFunc, 12, 12},
	})
}
