package scan

import (
	"reflect"
	"testing"

	"github.com/VocanicZ/rtdd/internal/graph"
)

func callsFrom(g graph.Graph, from string) []string {
	var out []string
	for _, e := range edgesOf(g, graph.RelCalls) {
		if e.From == from {
			out = append(out, e.To)
		}
	}
	return out
}

// Issue #579 rule 1: the by-name fallback never crosses language families.
func TestFallbackNeverCrossesLanguages(t *testing.T) {
	g, _ := assembleAll(
		[2]string{"tools/app.js", "function save(x) {\n  return x\n}\n"},
		[2]string{"tools/other.mjs", "function save(x) {\n  return x\n}\n"},
		[2]string{"scripts/run.py", "def main(img):\n    img.save('a.png')\n    save(img)\n"},
		[2]string{"tools/use.ts", "function go(o) {\n  save(o)\n}\n"},
	)
	if got := callsFrom(g, "scripts/run.py::main"); len(got) != 0 {
		t.Errorf("python main links %v, want none", got)
	}
	if got := callsFrom(g, "tools/use.ts::go"); len(got) != 2 {
		t.Errorf("ts go links %v, want both js-family saves", got)
	}
}

// Rule 2: a receiver imported from outside the repository links to nothing.
func TestExternalReceiverLinksNowhere(t *testing.T) {
	g, _ := assembleAll(
		[2]string{"tools/img.py", "def open(p):\n    return p\n"},
		[2]string{"scripts/run.py", "from PIL import Image\n\ndef main():\n    Image.open('a')\n"},
	)
	if got := callsFrom(g, "scripts/run.py::main"); len(got) != 0 {
		t.Errorf("links %v, want none", got)
	}
}

// Rule 3: a receiver that is a repository file's stem links to that file's member only,
// import visible or not (importlib-loaded).
func TestReceiverNamingAFileLinksToThatFile(t *testing.T) {
	g, _ := assembleAll(
		[2]string{"scripts/waitlib.py", "def pid_alive(p):\n    return p\n"},
		[2]string{"scripts/portable.py", "def pid_alive(p):\n    return p\n"},
		[2]string{"test/test_portable.py", "import importlib.util\n\ndef main():\n    portable = importlib.util.spec_from_file_location('portable', 'x')\n    portable.pid_alive(1)\n"},
	)
	want := []string{"scripts/portable.py::pid_alive"}
	if got := callsFrom(g, "test/test_portable.py::main"); !reflect.DeepEqual(got, want) {
		t.Errorf("links %v, want %v", got, want)
	}
}

// Rule 4: a call on a local or unnamed receiver stays in its file or imported files;
// elsewhere it is ambiguous, never repo-wide.
func TestLocalReceiverStaysLocal(t *testing.T) {
	g, st := assembleAll(
		[2]string{"scripts/waitlib.py", "def build(x):\n    return x\n"},
		[2]string{"test/test_a.py", "def main(canvas):\n    canvas.build(1)\n"},
		[2]string{"test/test_b.py", "def build(x):\n    return x\n\ndef main(canvas):\n    canvas.build(1)\n    make().build(2)\n"},
		[2]string{"test/test_c.py", "from scripts import waitlib\n\ndef main(canvas):\n    canvas.build(1)\n"},
	)
	if got := callsFrom(g, "test/test_a.py::main"); len(got) != 0 {
		t.Errorf("unrelated file: links %v, want none", got)
	}
	if want := []string{"test/test_b.py::build"}; !reflect.DeepEqual(callsFrom(g, "test/test_b.py::main"), want) {
		t.Errorf("same file: links %v, want %v", callsFrom(g, "test/test_b.py::main"), want)
	}
	if want := []string{"scripts/waitlib.py::build"}; !reflect.DeepEqual(callsFrom(g, "test/test_c.py::main"), want) {
		t.Errorf("imported file: links %v, want %v", callsFrom(g, "test/test_c.py::main"), want)
	}
	if st.AmbiguousCalls < 1 {
		t.Errorf("ambiguous = %d, want the unlinked local call counted", st.AmbiguousCalls)
	}
}

func TestFamilyGroupsOneLanguagesExtensions(t *testing.T) {
	for _, c := range []struct {
		a, b string
		same bool
	}{
		{".js", ".mjs", true}, {".ts", ".js", true}, {".sh", ".bash", true},
		{".py", ".js", false}, {".go", ".go", true}, {".go", ".lua", false},
	} {
		if (family(c.a) == family(c.b)) != c.same {
			t.Errorf("family(%s) vs family(%s): same = %v, want %v", c.a, c.b, !c.same, c.same)
		}
	}
}
