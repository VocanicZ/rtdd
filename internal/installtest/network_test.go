// network_test.go pins the blast radius of the one feature that reaches the internet.
// Before `rtdd update` existed, nothing in this module made a network call at all; that
// was a property of the tool worth keeping true on every other code path, and a property
// is only kept by something that fails when it stops being true.
package installtest

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// networkPackages are the import paths that let a program open a connection.
var networkPackages = map[string]bool{
	"net":      true,
	"net/http": true,
	"net/url":  false, // parsing a URL reaches nothing
}

// selfupdatePkg is the one directory allowed to import them.
const selfupdatePkg = "internal/selfupdate"

// TestOnlySelfupdateReachesTheNetwork walks every non-test source file in the module and
// fails if one outside internal/selfupdate imports a package that can open a connection.
//
// Test files are exempt: httptest servers on 127.0.0.1 are how the rest of this suite
// proves nothing escapes.
func TestOnlySelfupdateReachesTheNetwork(t *testing.T) {
	root := findRepoRootForTest(t)
	fset := token.NewFileSet()

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "build", "bench", "dist", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if strings.HasPrefix(filepath.ToSlash(rel), selfupdatePkg+"/") {
			return nil
		}

		f, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return parseErr
		}
		for _, imp := range f.Imports {
			p, uErr := strconv.Unquote(imp.Path.Value)
			if uErr != nil {
				return uErr
			}
			if networkPackages[p] {
				t.Errorf("%s imports %q. Only %s may reach the network: rtdd talks to the internet when a person types `rtdd update`, and on no other code path", rel, p, selfupdatePkg)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
}
