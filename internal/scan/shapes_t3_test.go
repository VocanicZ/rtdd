package scan

import (
	"reflect"
	"testing"

	"github.com/VocanicZ/rtdd/internal/graph"
)

// Lua names the last segment of a dotted or colon path (spec §4.2).
func TestLuaNamesTheLastSegment(t *testing.T) {
	checkSpans(t, "lua/m.lua",
		"local M = {}\n\nfunction M.load(path)\n  return path\nend\n\nfunction M:push(n)\n  self.n = n\nend\n\nreturn M\n",
		[]span{
			{"lua/m.lua::load", graph.KindFunc, 3, 4},
			{"lua/m.lua::push", graph.KindFunc, 7, 8},
		})
}

// Ruby: `it "label" do` is a test node; `end` closes by indentation (the end line itself
// is outside the span, spec §4.3); a paren-less call in command position is a call.
func TestRubyBlockTestsAndCommandCalls(t *testing.T) {
	src := "class Acc\n  def push(n)\n    log n\n  end\n\n  def log(n)\n    puts n\n  end\nend\n\ndescribe Acc do\n  it \"pushes\" do\n    Acc.new.push 1\n  end\nend\n"
	checkSpans(t, "spec/acc_spec.rb", src, []span{
		{"spec/acc_spec.rb::Acc", graph.KindClass, 1, 8},
		{"spec/acc_spec.rb::Acc::push", graph.KindMethod, 2, 3},
		{"spec/acc_spec.rb::Acc::log", graph.KindMethod, 6, 7},
		{"spec/acc_spec.rb::pushes", graph.KindTest, 12, 13},
	})
	r := ScanFile("spec/acc_spec.rb", []byte(src))
	want := []graph.Edge{{From: "spec/acc_spec.rb::Acc::push", To: "spec/acc_spec.rb::Acc::log", Relation: graph.RelCalls}}
	if got := Link(r.Nodes, r.Calls); !reflect.DeepEqual(got, want) {
		t.Errorf("calls = %+v, want %+v", got, want)
	}
}

// Shell calls a function by naming it first in a statement: `add 1 2`, `x=$(add 1 2)`.
// An assignment (`x=1`, `x = f()`) is not a call.
func TestShellCommandFormCalls(t *testing.T) {
	src := "add() {\n  echo $(( $1 + $2 ))\n}\n\nfunction total {\n  local x\n  x=$(add 1 2)\n  add 3 4 | cat\n}\n"
	checkSpans(t, "bin/calc.sh", src, []span{
		{"bin/calc.sh::add", graph.KindFunc, 1, 3},
		{"bin/calc.sh::total", graph.KindFunc, 5, 9},
	})
	r := ScanFile("bin/calc.sh", []byte(src))
	want := []graph.Edge{{From: "bin/calc.sh::total", To: "bin/calc.sh::add", Relation: graph.RelCalls}}
	if got := Link(r.Nodes, r.Calls); !reflect.DeepEqual(got, want) {
		t.Errorf("calls = %+v, want %+v", got, want)
	}
}

// The command form never reads a declaration's leading word (`let c = …`, `Calc c = …`)
// or a literal's type (`Calc { total: 0 }`) as a call; a bare `helper x` still is one.
func TestCommandFormSkipsDeclarationsAndLiterals(t *testing.T) {
	r := ScanFile("lib.rs", []byte("fn make() {\n    let c: Calc = x;\n    Calc c = y;\n    Calc { total: 0 }\n    helper c\n}\n"))
	if got, want := r.Calls["lib.rs::make"], []string{"helper"}; !reflect.DeepEqual(got, want) {
		t.Errorf("make calls %v, want %v", got, want)
	}
}
