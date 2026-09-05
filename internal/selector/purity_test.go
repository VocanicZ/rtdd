package selector

import (
	"go/parser"
	"go/token"
	"io/fs"
	"strconv"
	"strings"
	"testing"
)

// impure lists the standard-library packages that can only be there to touch the world.
// Select's contract is that it makes no git call, touches no filesystem and spawns no
// process: everything it needs about the repository arrives through Inputs, which is why
// Distance, ImportOnly, Exists and ImportDistance are injected functions rather than
// resolvers this package could call itself.
var impure = map[string]string{
	"os":            "the filesystem and the environment",
	"os/exec":       "subprocesses",
	"path/filepath": "host filesystem paths (engine paths are repo-relative and slash-separated)",
	"io/ioutil":     "the filesystem",
	"net":           "the network",
	"net/http":      "the network",
	"syscall":       "the host",
}

// A structural guard, in the shape of TestOnlySeedCallsMapstoreReplace: a reviewer can
// miss one os.Stat added to answer "does this file exist", and that one call is what
// would turn a pure function into one whose answer depends on the working directory.
func TestSelectorPackageStaysPure(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parsing internal/selector: %v", err)
	}
	for _, pkg := range pkgs {
		for name, f := range pkg.Files {
			for _, imp := range f.Imports {
				p, uerr := strconv.Unquote(imp.Path.Value)
				if uerr != nil {
					t.Fatalf("%s: unquoting %s: %v", name, imp.Path.Value, uerr)
				}
				if why, bad := impure[p]; bad {
					t.Errorf("%s imports %q (%s); Select is pure — inject the answer "+
						"through Inputs instead, as Exists and ImportDistance are",
						name, p, why)
				}
			}
		}
	}
}
