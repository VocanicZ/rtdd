package adapter

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/VocanicZ/rtdd/internal/covfmt"
)

var unitPlaceholders = map[string]bool{"{unit}": true, "{dir}": true, "{name}": true, "{names}": true, "{tmp}": true}

func (a *Adapter) validateV3() error {
	if bad := unknownPlaceholder(a.UnitCmd, unitPlaceholders); bad != "" {
		return fmt.Errorf("unit_cmd %q: unknown placeholder %s (only {unit}, {dir}, {name}, {names}, {tmp})", a.UnitCmd, bad)
	}
	if !slices.Contains(covfmt.Formats, a.CoverageFormat) {
		return fmt.Errorf("coverage_format %q: unsupported (only %v)", a.CoverageFormat, covfmt.Formats)
	}
	// Every unit gets a private {tmp}; a coverage file anywhere else is shared between
	// parallel units and one unit would read another's coverage.
	if !strings.HasPrefix(a.CoverageFile, "{tmp}/") {
		return fmt.Errorf("coverage_file %q: must start with {tmp}/ so parallel units never share it", a.CoverageFile)
	}
	if a.UnitNames != "" {
		re, err := regexp.Compile("(?m)" + a.UnitNames)
		if err != nil {
			return fmt.Errorf("unit_names %q: %v", a.UnitNames, err)
		}
		if re.NumSubexp() < 1 {
			return fmt.Errorf("unit_names %q: needs one capture group naming the test", a.UnitNames)
		}
	}
	for k := range a.UnitFiles {
		if k == "" || path.IsAbs(k) || path.Clean(k) != k || strings.Contains(k, `\`) || k == ".." || strings.HasPrefix(k, "../") {
			return fmt.Errorf("unit_files key %q: must be a clean relative slash path inside {tmp}", k)
		}
	}
	if a.Jobs < 0 {
		return fmt.Errorf("jobs %d: must be 0 (CPU count) or positive", a.Jobs)
	}
	return nil
}

func (a *Adapter) unitVars(unit, tmp, names string) map[string]string {
	dir := path.Dir(unit)
	base := path.Base(unit)
	return map[string]string{
		"unit":  unit,
		"dir":   dir,
		"name":  strings.TrimSuffix(base, path.Ext(base)),
		"names": names,
		"tmp":   tmp,
	}
}

// UnitArgv renders unit_cmd for one test file. Split on whitespace BEFORE substitution,
// as every command template is: a path with a space stays one argv element.
func (a *Adapter) UnitArgv(unit, tmp, names string) ([]string, error) {
	return a.Expand(a.UnitCmd, a.unitVars(unit, tmp, names))
}

func (a *Adapter) CoveragePath(tmp string) string {
	return strings.ReplaceAll(a.CoverageFile, "{tmp}", tmp)
}

func (a *Adapter) UnitEnv(tmp string) map[string]string {
	out := make(map[string]string, len(a.Env))
	for k, v := range a.Env {
		out[k] = strings.ReplaceAll(v, "{tmp}", tmp)
	}
	return out
}

func (a *Adapter) UnitFileContents(tmp string) map[string]string {
	out := make(map[string]string, len(a.UnitFiles))
	for k, v := range a.UnitFiles {
		out[k] = strings.ReplaceAll(v, "{tmp}", tmp)
	}
	return out
}

// UnitNamesOf fills {names} from the test file's source: every first capture of
// unit_names, as ^(A|B)$. ok is false when unit_names is declared and matches nothing —
// the file holds no runnable test and the unit is skipped, not run.
func (a *Adapter) UnitNamesOf(body []byte) (string, bool) {
	if a.UnitNames == "" {
		return "", true
	}
	re := regexp.MustCompile("(?m)" + a.UnitNames)
	var names []string
	for _, m := range re.FindAllSubmatch(body, -1) {
		names = append(names, regexp.QuoteMeta(string(m[1])))
	}
	if len(names) == 0 {
		return "", false
	}
	return "^(" + strings.Join(names, "|") + ")$", true
}
