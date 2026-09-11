// release_artifacts_test.go enforces PRD #6 acceptance criterion 5 - "GoReleaser produces
// static binaries for linux/darwin x amd64/arm64. Every artifact verified `statically
// linked`" - for *every* artifact GoReleaser is configured to publish, not just the
// linux/amd64 host build that .github/workflows/ci.yml checks on its own.
//
// The release matrix is read out of .goreleaser.yaml rather than hard-coded here, so
// adding a goos/goarch to the release config cannot quietly add an unverified artifact.
//
// "statically linked" is an ELF-only phrase, so the property is asserted per executable
// format:
//
//   - ELF (linux/*) - no PT_INTERP, no dynamic section, no imported libraries. That is
//     exactly the condition under which `file` prints "statically linked".
//   - Mach-O (darwin/*) - Go on darwin always links libSystem, because Apple exposes no
//     stable raw-syscall ABI, so the literal string is unobtainable there and asserting it
//     would be a check that can only ever fail. The property the criterion is actually
//     after is "depends on nothing beyond the base system", so every LC_LOAD_DYLIB entry
//     must name a dylib macOS itself ships. `otool` is not on a Linux runner; debug/macho
//     reads the load commands directly and needs no extra tooling.
//   - PE (windows/*) - the same property, expressed as imported DLLs that are all
//     OS-provided.
//
// Every format additionally has to be cgo-free, read from the binary's own build info -
// the same data `go version -m` prints, and the authoritative record of whether the
// artifact was linked with cgo.
package installtest

import (
	"debug/buildinfo"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// goreleaserConfig is the slice of .goreleaser.yaml this file needs: which artifacts get
// built, with what environment and flags, and what the archives ship alongside them.
type goreleaserConfig struct {
	Builds []struct {
		ID     string   `yaml:"id"`
		Main   string   `yaml:"main"`
		Binary string   `yaml:"binary"`
		Env    []string `yaml:"env"`
		Flags  []string `yaml:"flags"`
		Goos   []string `yaml:"goos"`
		Goarch []string `yaml:"goarch"`
		Ignore []struct {
			Goos   string `yaml:"goos"`
			Goarch string `yaml:"goarch"`
		} `yaml:"ignore"`
	} `yaml:"builds"`
	Archives []struct {
		ID              string   `yaml:"id"`
		Files           []string `yaml:"files"`
		FormatOverrides []struct {
			Goos    string   `yaml:"goos"`
			Formats []string `yaml:"formats"`
		} `yaml:"format_overrides"`
	} `yaml:"archives"`
}

func loadGoreleaserConfig(t *testing.T) goreleaserConfig {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(findRepoRootForTest(t), ".goreleaser.yaml"))
	if err != nil {
		t.Fatalf("read .goreleaser.yaml: %v", err)
	}
	var cfg goreleaserConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("parse .goreleaser.yaml: %v", err)
	}
	if len(cfg.Builds) == 0 {
		t.Fatal(".goreleaser.yaml declares no builds")
	}
	return cfg
}

// releaseTarget is one artifact GoReleaser publishes.
type releaseTarget struct {
	goos, goarch string
	main         string
	env, flags   []string
}

func (tg releaseTarget) String() string { return tg.goos + "/" + tg.goarch }

// releaseMatrix expands each build's goos x goarch cross product minus its ignore list -
// the same expansion GoReleaser does - so the set under test is derived from the release
// config instead of duplicating it.
func releaseMatrix(cfg goreleaserConfig) []releaseTarget {
	var out []releaseTarget
	for _, b := range cfg.Builds {
		for _, goos := range b.Goos {
			for _, goarch := range b.Goarch {
				ignored := false
				for _, ig := range b.Ignore {
					if ig.Goos == goos && ig.Goarch == goarch {
						ignored = true
					}
				}
				if ignored {
					continue
				}
				out = append(out, releaseTarget{goos: goos, goarch: goarch, main: b.Main, env: b.Env, flags: b.Flags})
			}
		}
	}
	return out
}

// buildReleaseArtifact cross-compiles one release target with the environment and flags
// .goreleaser.yaml specifies, and returns the path to the binary.
func buildReleaseArtifact(t *testing.T, dir string, tg releaseTarget) string {
	t.Helper()
	out := filepath.Join(dir, "rtdd-"+tg.goos+"-"+tg.goarch)
	args := append([]string{"build"}, tg.flags...)
	args = append(args, "-o", out, tg.main)
	cmd := exec.Command("go", args...)
	cmd.Env = append(os.Environ(), "GOOS="+tg.goos, "GOARCH="+tg.goarch)
	cmd.Env = append(cmd.Env, tg.env...)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("cross-build %s: %v\n%s", tg, err, b)
	}
	return out
}

// baseSystemDylibs are the dylibs macOS provides to every process. libSystem is
// unavoidable (Apple exposes no stable raw-syscall ABI); libresolv is pulled in by the
// standard library's net package on darwin even under CGO_ENABLED=0 - a hello-world binary
// links only libSystem, one that imports net links both. Neither makes the artifact depend
// on anything a user has to install.
var baseSystemDylibs = map[string]bool{
	"/usr/lib/libSystem.B.dylib": true,
	"/usr/lib/libresolv.9.dylib": true,
	// CoreFoundation and Security arrive with crypto/x509, which `rtdd update` reaches
	// through net/http: on darwin the standard library verifies a server certificate
	// against the system trust store, and the trust store is those two frameworks. They
	// are in the same category as libresolv above - Apple ships them inside macOS, in
	// /System/Library/Frameworks, and no user installs or can remove them. A binary
	// loading them still depends on nothing beyond the base system, which is the property
	// PRD #6 criterion 5 is actually after.
	"/System/Library/Frameworks/CoreFoundation.framework/Versions/A/CoreFoundation": true,
	"/System/Library/Frameworks/Security.framework/Versions/A/Security":             true,
}

// foreignDylibs returns the entries of libs that macOS does not ship. It is separate from
// the Mach-O reader so the classification can be tested against a synthetic list, which is
// what keeps the allowlist above from being widened into meaninglessness.
func foreignDylibs(libs []string) []string {
	var foreign []string
	for _, lib := range libs {
		if !baseSystemDylibs[lib] {
			foreign = append(foreign, lib)
		}
	}
	return foreign
}

// baseSystemDLLs are Windows DLLs shipped with the OS. A Go binary built without cgo
// imports only kernel32 up front and loads the rest lazily through LoadLibrary.
var baseSystemDLLs = map[string]bool{
	"kernel32.dll":         true,
	"advapi32.dll":         true,
	"ntdll.dll":            true,
	"ws2_32.dll":           true,
	"user32.dll":           true,
	"bcryptprimitives.dll": true,
}

// verifyArtifactLinkage returns nil when the binary at path is a self-contained, cgo-free
// artifact for the given target, and an error naming the specific violation otherwise. It
// returns an error rather than calling t.Error so that
// TestReleaseLinkageCheckRejectsADynamicallyLinkedBinary can prove it actually rejects
// something: a check that can never fail is not a check, which is how three of the four
// artifacts came to be unverified in the first place.
func verifyArtifactLinkage(path string, tg releaseTarget) error {
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read build info: %w", err)
	}
	settings := map[string]string{}
	for _, s := range info.Settings {
		settings[s.Key] = s.Value
	}
	if got := settings["CGO_ENABLED"]; got != "0" {
		return fmt.Errorf("built with CGO_ENABLED=%q, want \"0\": a cgo artifact is not self-contained", got)
	}
	if got := settings["GOOS"]; got != tg.goos {
		return fmt.Errorf("build info says GOOS=%q, want %q", got, tg.goos)
	}
	if got := settings["GOARCH"]; got != tg.goarch {
		return fmt.Errorf("build info says GOARCH=%q, want %q", got, tg.goarch)
	}

	switch tg.goos {
	case "linux":
		return verifyELFIsStaticallyLinked(path)
	case "darwin":
		return verifyMachOUsesOnlyBaseSystemDylibs(path)
	case "windows":
		return verifyPEUsesOnlyBaseSystemDLLs(path)
	default:
		return fmt.Errorf("no linkage assertion for GOOS=%q: add one before releasing that platform", tg.goos)
	}
}

// verifyELFIsStaticallyLinked asserts the three properties that together make `file` print
// "statically linked": no interpreter, no dynamic section, no imported libraries.
func verifyELFIsStaticallyLinked(path string) error {
	f, err := elf.Open(path)
	if err != nil {
		return fmt.Errorf("open as ELF: %w", err)
	}
	defer f.Close()

	for _, p := range f.Progs {
		if p.Type == elf.PT_INTERP {
			return fmt.Errorf("has a PT_INTERP program header: dynamically linked, not statically linked")
		}
	}
	for _, s := range f.Sections {
		if s.Type == elf.SHT_DYNAMIC || s.Name == ".interp" {
			return fmt.Errorf("has the dynamic-linking section %q: dynamically linked, not statically linked", s.Name)
		}
	}
	libs, err := f.ImportedLibraries()
	if err != nil {
		return fmt.Errorf("read imported libraries: %w", err)
	}
	if len(libs) > 0 {
		return fmt.Errorf("imports shared libraries %v: dynamically linked, not statically linked", libs)
	}
	return nil
}

// verifyMachOUsesOnlyBaseSystemDylibs reads the LC_LOAD_DYLIB load commands directly, the
// way `otool -L` would, and requires every one of them to name a dylib macOS itself ships.
func verifyMachOUsesOnlyBaseSystemDylibs(path string) error {
	f, err := macho.Open(path)
	if err != nil {
		return fmt.Errorf("open as Mach-O: %w", err)
	}
	defer f.Close()

	libs, err := f.ImportedLibraries()
	if err != nil {
		return fmt.Errorf("read load commands: %w", err)
	}
	if foreign := foreignDylibs(libs); len(foreign) > 0 {
		sort.Strings(foreign)
		return fmt.Errorf("loads non-base-system dylibs %v: the artifact is not self-contained", foreign)
	}
	return nil
}

// verifyPEUsesOnlyBaseSystemDLLs applies the same "nothing beyond the base system" rule to
// the Windows artifact. debug/pe's ImportedLibraries comes back empty for Go binaries, so
// the DLL names are taken from the imported symbols, which carry them as a ":dll" suffix.
func verifyPEUsesOnlyBaseSystemDLLs(path string) error {
	f, err := pe.Open(path)
	if err != nil {
		return fmt.Errorf("open as PE: %w", err)
	}
	defer f.Close()

	syms, err := f.ImportedSymbols()
	if err != nil {
		return fmt.Errorf("read imported symbols: %w", err)
	}
	seen := map[string]bool{}
	var foreign []string
	for _, s := range syms {
		i := strings.LastIndex(s, ":")
		if i < 0 {
			continue
		}
		dll := strings.ToLower(s[i+1:])
		if seen[dll] {
			continue
		}
		seen[dll] = true
		if !baseSystemDLLs[dll] {
			foreign = append(foreign, dll)
		}
	}
	if len(foreign) > 0 {
		sort.Strings(foreign)
		return fmt.Errorf("imports non-base-system DLLs %v: the artifact is not self-contained", foreign)
	}
	return nil
}

// TestReleaseArtifactsAreStaticallyLinked is the acceptance check itself: every artifact in
// .goreleaser.yaml's matrix is cross-built and inspected. scripts/release-preflight.sh and
// both CI entrypoints run this test by name, so renaming it means updating them too.
func TestReleaseArtifactsAreStaticallyLinked(t *testing.T) {
	matrix := releaseMatrix(loadGoreleaserConfig(t))

	// The four artifacts PRD #6 criterion 5 names by hand must all be in the matrix. A
	// .goreleaser.yaml that quietly stopped building one of them would otherwise let this
	// test stay green by checking less.
	required := map[string]bool{"linux/amd64": false, "linux/arm64": false, "darwin/amd64": false, "darwin/arm64": false}
	for _, tg := range matrix {
		if _, ok := required[tg.String()]; ok {
			required[tg.String()] = true
		}
	}
	names := make([]string, 0, len(required))
	for name := range required {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if !required[name] {
			t.Errorf(".goreleaser.yaml no longer builds %s, which PRD #6 criterion 5 requires", name)
		}
	}

	dir := t.TempDir()
	for _, tg := range matrix {
		t.Run(tg.goos+"_"+tg.goarch, func(t *testing.T) {
			bin := buildReleaseArtifact(t, dir, tg)
			if err := verifyArtifactLinkage(bin, tg); err != nil {
				t.Errorf("%s artifact is not a self-contained static binary: %v", tg, err)
			}
		})
	}
}

// TestReleaseLinkageCheckRejectsADynamicallyLinkedBinary proves the checks above can fail.
// The counterexample is a two-line cgo module rather than a cgo build of ./cmd/rtdd: it
// links against libc exactly the same way (PT_INTERP, .dynamic, an imported libc) while
// compiling in seconds instead of half a minute.
func TestReleaseLinkageCheckRejectsADynamicallyLinkedBinary(t *testing.T) {
	if goEnv(t, "GOOS") != "linux" {
		t.Skipf("the counterexample is an ELF; host GOOS is %s", goEnv(t, "GOOS"))
	}
	cc := goEnv(t, "CC")
	if _, err := exec.LookPath(cc); err != nil {
		t.Skipf("no C compiler (%s) on PATH, cannot build a dynamically linked counterexample", cc)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module cgocounterexample\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nimport \"C\"\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "counterexample")
	cmd := exec.Command("go", "build", "-o", out, ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1", "GOFLAGS=-mod=mod")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("cgo build unavailable in this environment: %v\n%s", err, b)
	}

	host := releaseTarget{goos: "linux", goarch: goEnv(t, "GOARCH")}
	err := verifyArtifactLinkage(out, host)
	if err == nil {
		t.Fatal("verifyArtifactLinkage accepted a cgo, dynamically linked binary: the check is vacuous")
	}
	if !strings.Contains(err.Error(), "CGO_ENABLED") {
		t.Errorf("expected the rejection to name CGO_ENABLED, got: %v", err)
	}

	// And the ELF check on its own must reject it too, so the cgo setting is not the only
	// thing standing between a dynamically linked artifact and a green release.
	elfErr := verifyELFIsStaticallyLinked(out)
	if elfErr == nil {
		t.Fatal("verifyELFIsStaticallyLinked accepted a dynamically linked ELF")
	}
	if !strings.Contains(elfErr.Error(), "not statically linked") {
		t.Errorf("expected the ELF rejection to say it is not statically linked, got: %v", elfErr)
	}
}

// TestReleaseLinkageCheckRejectsAMismatchedArtifact covers the other way the check could go
// vacuous: inspecting a binary that is not the artifact it claims to be.
func TestReleaseLinkageCheckRejectsAMismatchedArtifact(t *testing.T) {
	dir := t.TempDir()
	darwin := releaseTarget{goos: "darwin", goarch: "arm64", main: "./cmd/rtdd", env: []string{"CGO_ENABLED=0"}, flags: []string{"-trimpath"}}
	bin := buildReleaseArtifact(t, dir, darwin)

	if err := verifyArtifactLinkage(bin, releaseTarget{goos: "linux", goarch: "arm64"}); err == nil {
		t.Error("a darwin binary was accepted as the linux artifact")
	}
	if err := verifyELFIsStaticallyLinked(bin); err == nil {
		t.Error("a Mach-O binary was accepted by the ELF check")
	}
}

func goEnv(t *testing.T, key string) string {
	t.Helper()
	out, err := exec.Command("go", "env", key).Output()
	if err != nil {
		t.Fatalf("go env %s: %v", key, err)
	}
	return strings.TrimSpace(string(out))
}

// TestReleaseBuildsAreConfiguredWithoutCgo guards the release config rather than its
// output: the artifacts can only come out static if GoReleaser builds them with cgo off.
func TestReleaseBuildsAreConfiguredWithoutCgo(t *testing.T) {
	for _, b := range loadGoreleaserConfig(t).Builds {
		found := false
		for _, e := range b.Env {
			if e == "CGO_ENABLED=0" {
				found = true
			}
		}
		if !found {
			t.Errorf("build %q must set CGO_ENABLED=0 in env, got %v", b.ID, b.Env)
		}
	}
}

// TestReleaseArchivesShipTheLicenseAndReadme covers plan Task 20's archive contents. A
// released tarball that carries three generated front-ends and no LICENSE ships the
// software's redistribution terms nowhere.
func TestReleaseArchivesShipTheLicenseAndReadme(t *testing.T) {
	cfg := loadGoreleaserConfig(t)
	if len(cfg.Archives) == 0 {
		t.Fatal(".goreleaser.yaml declares no archives")
	}
	want := []string{
		"README.md",
		"LICENSE",
		"dist/SKILL.md",
		"dist/AGENTS.md",
		"dist/cursor/rules/rtdd.mdc",
		"dist/GLOBAL-SKILL.md",
		"dist/GLOBAL-AGENTS.md",
	}
	for _, a := range cfg.Archives {
		have := map[string]bool{}
		for _, f := range a.Files {
			have[f] = true
		}
		for _, w := range want {
			if !have[w] {
				t.Errorf("archive %q must ship %q, has %v", a.ID, w, a.Files)
			}
		}
	}
}

// The installer always fetches a .tar.gz, on every platform, because it extracts in pure
// Node and tar is the format it can read without depending on anything the OS ships. The
// .zip exists for a human downloading from the releases page on Windows, where Explorer
// cannot open a .tar.gz. Both are published, so neither audience is stranded.
func TestReleaseShipsBothWindowsArchiveFormats(t *testing.T) {
	cfg := loadGoreleaserConfig(t)
	for _, a := range cfg.Archives {
		var windows []string
		for _, o := range a.FormatOverrides {
			if o.Goos == "windows" {
				windows = o.Formats
			}
		}
		if windows == nil {
			t.Fatalf("archive %q declares no windows format_overrides", a.ID)
		}
		for _, want := range []string{"zip", "tar.gz"} {
			found := false
			for _, f := range windows {
				if f == want {
					found = true
				}
			}
			if !found {
				t.Errorf("archive %q: windows formats = %v, must include %q", a.ID, windows, want)
			}
		}
	}
}
