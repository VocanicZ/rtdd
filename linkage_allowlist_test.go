// linkage_allowlist_test.go guards the allowlist that TestReleaseArtifactsAreStaticallyLinked
// checks artifacts against. That list is the one place where "self-contained" is defined,
// so widening it is how the acceptance criterion would be quietly lost: an entry nobody
// questions turns a failing artifact into a passing one.
package installtest

import (
	"strings"
	"testing"
)

// applePrefixes are the two places macOS itself ships libraries. Anything outside them was
// installed by somebody, which is the definition of not self-contained.
var applePrefixes = []string{"/usr/lib/", "/System/Library/Frameworks/"}

// TestBaseSystemDylibsOnlyNamesLibrariesMacOSShips fails if an entry is added that a user
// would have to install - a Homebrew path, /usr/local, anything vendored.
func TestBaseSystemDylibsOnlyNamesLibrariesMacOSShips(t *testing.T) {
	for lib := range baseSystemDylibs {
		ok := false
		for _, p := range applePrefixes {
			if strings.HasPrefix(lib, p) {
				ok = true
				break
			}
		}
		if !ok {
			t.Errorf("baseSystemDylibs allows %q, which is not under %v: macOS does not ship it, so an artifact loading it is not self-contained", lib, applePrefixes)
		}
	}
}

// TestForeignDylibsFlagsAnythingOutsideTheBaseSystem proves the classification can still
// fail after the allowlist grew. Without it, widening the list far enough to accept
// everything would leave every artifact passing and no test complaining.
func TestForeignDylibsFlagsAnythingOutsideTheBaseSystem(t *testing.T) {
	got := foreignDylibs([]string{
		"/usr/lib/libSystem.B.dylib",
		"/opt/homebrew/lib/libpcre2-8.0.dylib",
		"/usr/local/lib/libsqlite3.dylib",
	})
	if len(got) != 2 {
		t.Fatalf("foreignDylibs returned %v, want the two libraries macOS does not ship", got)
	}
	for _, want := range []string{"/opt/homebrew/lib/libpcre2-8.0.dylib", "/usr/local/lib/libsqlite3.dylib"} {
		found := false
		for _, g := range got {
			if g == want {
				found = true
			}
		}
		if !found {
			t.Errorf("foreignDylibs did not flag %q", want)
		}
	}
}

// TestForeignDylibsAcceptsTheBaseSystem - the positive half, so the check above cannot be
// satisfied by a function that simply flags everything.
func TestForeignDylibsAcceptsTheBaseSystem(t *testing.T) {
	if got := foreignDylibs([]string{"/usr/lib/libSystem.B.dylib", "/usr/lib/libresolv.9.dylib"}); len(got) != 0 {
		t.Errorf("foreignDylibs flagged base-system libraries: %v", got)
	}
}
