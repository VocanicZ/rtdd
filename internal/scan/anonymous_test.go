package scan

import (
	"testing"

	"github.com/VocanicZ/rtdd/internal/graph"
)

// Issue #466: an anonymous literal (Go `func(...) {`, JS `function (...) {`) is not a
// node named `func` or `function`, and a statement inside a body (`if n := len(x); n > 0 {`,
// a call's continuation line `x, len(y), z(w),`) is not a `<type> Name(` method. Both
// belong to the one named function that encloses them.
func TestGoAnonymousLiteralsAndStatementsBelongToTheEnclosingFunction(t *testing.T) {
	checkSpans(t, "pkg/process.go",
		"func Process(xs []int) int {\n"+
			"\ttotal := 0\n"+
			"\teach := func(x int) {\n"+
			"\t\ttotal += x\n"+
			"\t}\n"+
			"\tgo func() {\n"+
			"\t\tdone()\n"+
			"\t}()\n"+
			"\tcases := []struct {\n"+
			"\t\tsetup  func(t *testing.T, dir string)\n"+
			"\t\tmutate func(*Inputs)\n"+
			"\t}{\n"+
			"\t\t{mutate: func(in *Inputs) {\n"+
			"\t\t\tin.n++\n"+
			"\t\t}},\n"+
			"\t}\n"+
			"\tif n := len(xs); n > 0 {\n"+
			"\t\teach(n)\n"+
			"\t}\n"+
			"\treport(\"%d %s\",\n"+
			"\t\tres.Commit, len(res.Stale), plural(len(res.Stale), \"file\"),\n"+
			"\t\tcases)\n"+
			"\treturn total\n"+
			"}\n",
		[]span{
			{"pkg/process.go::Process", graph.KindFunc, 1, 24},
		})
}

func TestJavaScriptAnonymousFunctionsAndStatementsBelongToTheEnclosingFunction(t *testing.T) {
	checkSpans(t, "src/process.js",
		"function process(xs) {\n"+
			"  let total = 0;\n"+
			"  xs.forEach(function (x) {\n"+
			"    total += x;\n"+
			"  });\n"+
			"  const handlers = {\n"+
			"    onDone: function (err) {\n"+
			"      report(err);\n"+
			"    },\n"+
			"  };\n"+
			"  if ((n = len(xs)) > 0) {\n"+
			"    each(n);\n"+
			"  }\n"+
			"  return total;\n"+
			"}\n",
		[]span{
			{"src/process.js::process", graph.KindFunc, 1, 15},
		})
}
