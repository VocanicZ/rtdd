package scan

import (
	"testing"

	"github.com/VocanicZ/rtdd/internal/graph"
)

// TS/JS class members carry no type tokens before their name: `plus(x) {` is a method.
// A callback opener (`useEffect(() => {`) and a control statement (`if (x) {`) are not.
func TestTypeScriptClassMembersAreMethods(t *testing.T) {
	checkSpans(t, "src/calc.ts",
		"export class Calc {\n  private memory = 0;\n\n  plus(x: number): number {\n    if (x > 0) {\n      log(x);\n    }\n    return x;\n  }\n\n  async load(path) {\n    return read(path);\n  }\n}\n\nuseEffect(() => {\n  run();\n});\n",
		[]span{
			{"src/calc.ts::Calc", graph.KindClass, 1, 14},
			{"src/calc.ts::Calc::plus", graph.KindMethod, 4, 9},
			{"src/calc.ts::Calc::load", graph.KindMethod, 11, 13},
		})
}

// Java: a generic method is a method; a `;`-terminated declaration and a control
// statement are not nodes at all.
func TestJavaMethodsButNotDeclarationsOrControlStatements(t *testing.T) {
	checkSpans(t, "src/Calc.java",
		"public class Calc implements Op {\n    public <T> List<T> wrap(T x) {\n        return List.of(x);\n    }\n\n    int apply(int a, int b);\n\n    void run() {\n        if (ready()) {\n            while (busy()) {\n                step();\n            }\n        }\n    }\n}\n",
		[]span{
			{"src/Calc.java::Calc", graph.KindClass, 1, 15},
			{"src/Calc.java::Calc::wrap", graph.KindMethod, 2, 4},
			{"src/Calc.java::Calc::run", graph.KindMethod, 8, 14},
		})
}

// Rust: `impl<T> Trait for Type` names Type; struct and impl of one type are two class
// nodes (the second gets the @<start> suffix); `fn new` is a method, not a keyword.
func TestRustImplForNamesTheTypeAndNewIsAMethod(t *testing.T) {
	checkSpans(t, "src/calc.rs",
		"pub struct Calc {\n    total: i32,\n}\n\nimpl<T: Copy> Summer for Calc<T> {\n    fn new() -> Self {\n        Calc { total: 0 }\n    }\n}\n",
		[]span{
			{"src/calc.rs::Calc", graph.KindClass, 1, 3},
			{"src/calc.rs::Calc@5", graph.KindClass, 5, 9},
			{"src/calc.rs::Calc::new", graph.KindMethod, 6, 8},
		})
}
