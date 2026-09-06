// This file is the SECOND parser in internal/report. The pytest-reportlog path in
// reportlog.go is not touched by it and is not refactored into a shared abstraction: one
// accumulates phases per test id, the other reads a tree, and the only thing they share is
// the outcome vocabulary they eventually produce.
package report

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
)

// The four ways a JUnit report can fail to be one. Each is a sentinel so a caller can
// distinguish them with errors.Is, and each is WRAPPED with the offending path so a human
// reading exit 1 knows which file to open. None of them is ever traded for zero outcomes
// and a nil error: a silent zero-test success reads exactly like a passing run.
var (
	// ErrNoReport is a report_path that does not exist after the runner ran. It usually
	// means the adapter's `requires` was unmet — the reporter package was never installed —
	// which doctor and init already warn about.
	ErrNoReport = errors.New("junit-xml: no report file")
	// ErrEmptyReport is a zero-byte report: the runner created the file and wrote nothing.
	ErrEmptyReport = errors.New("junit-xml: empty report file")
	// ErrMalformedReport is XML that does not parse, or a root element that is neither
	// <testsuites> nor <testsuite>.
	ErrMalformedReport = errors.New("junit-xml: malformed report")
	// ErrSuiteFailure is a <testsuite> carrying a suite-level <failure> or <error> and no
	// <testcase> of its own: a suite that could not run at all. Nested <testsuite> children
	// do not count — cases inside one belong to that suite, so a parent that failed at
	// import time is still a suite nothing ran in. Reporting zero tests from it would be a
	// false green.
	ErrSuiteFailure = errors.New("junit-xml: suite failed before any test ran")
	// ErrNoTestcases is a well-formed report that names no <testcase> at all. It is NOT
	// ErrEmptyReport — the runner wrote a real report — and it is distinct so a caller can
	// tell "the runner wrote nothing" from "the runner wrote a report naming no test",
	// which is the shape a selection the runner matched nothing in produces. See
	// ReadJUnitReport, which decides it for the report as a whole rather than per file.
	ErrNoTestcases = errors.New("junit-xml: report names no test")
	// ErrUnknownElement is an element inside a <testsuites> or <testsuite> that this
	// parser neither models nor allowlists as carrying nothing. It is its own sentinel
	// because it is a parser gap, not a broken runner: the report is well-formed XML and
	// the runner is entitled to its shape — rtdd is the side that has not learned it. It
	// exists because the blanket skip that used to stand here was the single mechanism
	// behind a whole family of silent drops (a nested suite's failure, a zero-testcase
	// suite, a root-level testcase): encoding/xml discards what the struct does not model,
	// so an element nobody thought about disappeared and took its result with it.
	ErrUnknownElement = errors.New("junit-xml: unrecognised element")
)

// skippableElements are the elements known to carry nothing this parser reads. They are
// skipped BY NAME rather than by "anything unrecognised", so a shape nobody has thought
// about is named instead of dropped. Add to this list deliberately, with the reason:
//
//   - properties/property — runner and environment metadata (java.version, seeds). No
//     outcome, no id, no duration lives here.
//   - system-out/system-err — the captured stdout/stderr of the suite or case. rtdd reads
//     outcomes, not logs; the failure text it does report comes from <failure>/<error>.
var skippableElements = map[string]bool{
	"properties": true,
	"property":   true,
	"system-out": true,
	"system-err": true,
}

// unknownElement is the error junitSuite.UnmarshalXML returns for an element that is
// neither modelled nor skippable. It carries the two names the decoder knows and the
// caller does not; ReadJUnitFile adds the third, the report path, on the way out.
type unknownElement struct {
	element string
	parent  string
}

func (e *unknownElement) Error() string {
	return fmt.Sprintf("%v: <%s> inside %s", ErrUnknownElement, e.element, e.parent)
}

func (e *unknownElement) Unwrap() error { return ErrUnknownElement }

// JUnitCase is one <testcase> as the runner wrote it, before any id rendering. It keeps
// the attributes id_template may name plus the enclosing suite, which every error message
// in this package uses to say WHICH case it is talking about.
type JUnitCase struct {
	Suite      string // the innermost enclosing <testsuite name=>
	Classname  string // the classname= attribute; "" when absent
	Name       string // the name= attribute
	File       string // the file= attribute; "" when the runner does not emit one
	Status     string // "pass" | "fail" | "skip" | "error" — report.Outcome's vocabulary
	DurationMS int    // from time=, which JUnit writes in SECONDS
}

// junitSuite is one <testsuite>. A suite may contain both <testcase> children and further
// <testsuite> children, so the type is recursive.
type junitSuite struct {
	Name string
	// Children holds the <testcase> and nested <testsuite> elements INTERLEAVED, in the
	// order the runner wrote them. Two typed slices would read every case before every
	// nested suite, which reorders any suite that writes a nested suite first.
	Children []junitChild
	// Failure is a suite-level <failure> — a suite that could not run at all.
	Failure *junitDetail
	Error   *junitDetail
}

// junitChild is one of a suite's two child kinds; exactly one field is non-nil.
type junitChild struct {
	Case  *junitCase
	Suite *junitSuite
}

// UnmarshalXML reads a <testsuite> child by child so document order survives the decode.
// encoding/xml's struct tags cannot express "these two element names share one ordering",
// so the walk is explicit.
func (s *junitSuite) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, a := range start.Attr {
		if a.Name.Local == "name" {
			s.Name = a.Value
		}
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "testsuite":
				var child junitSuite
				if err := d.DecodeElement(&child, &t); err != nil {
					return err
				}
				s.Children = append(s.Children, junitChild{Suite: &child})
			case "testcase":
				var c junitCase
				if err := d.DecodeElement(&c, &t); err != nil {
					return err
				}
				s.Children = append(s.Children, junitChild{Case: &c})
			case "failure":
				var det junitDetail
				if err := d.DecodeElement(&det, &t); err != nil {
					return err
				}
				if s.Failure == nil {
					s.Failure = &det
				}
			case "error":
				var det junitDetail
				if err := d.DecodeElement(&det, &t); err != nil {
					return err
				}
				if s.Error == nil {
					s.Error = &det
				}
			default:
				if !skippableElements[t.Name.Local] {
					return &unknownElement{element: t.Name.Local, parent: describeSuite(start.Name.Local, s.Name)}
				}
				if err := d.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			return nil
		}
	}
}

// describeSuite names the element an unknown child was found in, the way the file spells
// it: the element itself plus its name= when it has one. An unnamed root — jest-junit
// writes <testsuites> with no attributes — still says something a human can search for.
func describeSuite(element, name string) string {
	if name == "" {
		return "<" + element + ">"
	}
	return fmt.Sprintf("<%s name=%q>", element, name)
}

type junitCase struct {
	Classname string       `xml:"classname,attr"`
	Name      string       `xml:"name,attr"`
	File      string       `xml:"file,attr"`
	Time      string       `xml:"time,attr"` // string: an absent attribute must be 0, not an error
	Failure   *junitDetail `xml:"failure"`
	Error     *junitDetail `xml:"error"`
	Skipped   *junitDetail `xml:"skipped"`
}

type junitDetail struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Text    string `xml:",chardata"`
}

// ReadJUnitFile parses one JUnit XML file into its cases, in document order.
func ReadJUnitFile(path string) ([]JUnitCase, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("report: %w: %s", ErrNoReport, path)
		}
		return nil, fmt.Errorf("report: reading %s: %w", path, err)
	}
	if len(bytes.TrimSpace(b)) == 0 {
		return nil, fmt.Errorf("report: %w: %s", ErrEmptyReport, path)
	}

	dec := xml.NewDecoder(bytes.NewReader(b))
	// The root is decided by the first StartElement rather than by decoding one shape and
	// falling back to the other, so a document that is not a report at all produces
	// ErrMalformedReport instead of a mislabelled empty parse.
	var root xml.StartElement
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("report: %w: %s: %v", ErrMalformedReport, path, err)
		}
		if se, ok := tok.(xml.StartElement); ok {
			root = se
			break
		}
	}
	var suites []junitSuite
	switch root.Name.Local {
	case "testsuites":
		// The root is read by the SAME explicit walk a <testsuite> gets, and for the same
		// reason: struct tags name the children they want and encoding/xml discards every
		// other one without a word. Decoding <testsuite>, <failure> and <error> alone
		// therefore dropped a <testcase> written directly under the root — go-junit-report
		// and jest-junit both emit one for a test belonging to no suite — and took its
		// result with it. In a single-file report ErrNoTestcases caught the fallout; in a
		// DIRECTORY report a sibling file supplies the tests, so a failing case simply
		// disappeared behind a green run. Reusing junitSuite keeps the root's cases in
		// document order beside its suites, so the case is reported rather than refused.
		var wrapper junitSuite
		if err := dec.DecodeElement(&wrapper, &root); err != nil {
			return nil, decodeError(err, path)
		}
		// An unnamed root still has to say something a human can act on, so it falls back
		// to the element itself — as the suite of its own cases as well as in its errors.
		if wrapper.Name == "" {
			wrapper.Name = "<" + root.Name.Local + ">"
		}
		// A root-level <failure> is decided here rather than left to walkSuite's gate,
		// which spares a suite carrying cases of its own on the "this is a summary, the
		// cases carry the detail" reading. A root gets no such reading: the schema does
		// not permit it a <testcase> at all, so one written there says nothing about
		// whether the run the root declares dead actually ran. Deciding it before the walk
		// also keeps the outer cause winning over any inner symptom, and keeps the suites
		// that did report from being handed back as the run — they are the part that
		// happened to survive.
		if detail := wrapper.Failure; detail != nil {
			return nil, suiteFailure(path, wrapper.Name, detail)
		}
		if detail := wrapper.Error; detail != nil {
			return nil, suiteFailure(path, wrapper.Name, detail)
		}
		suites = []junitSuite{wrapper}
	case "testsuite":
		var s junitSuite
		if err := dec.DecodeElement(&s, &root); err != nil {
			return nil, decodeError(err, path)
		}
		suites = []junitSuite{s}
	default:
		return nil, fmt.Errorf("report: %w: %s: root element is <%s>, want <testsuites> or <testsuite>", ErrMalformedReport, path, root.Name.Local)
	}

	if err := refuseContentAfterRoot(dec, path); err != nil {
		return nil, err
	}

	out := []JUnitCase{}
	for i := range suites {
		out, err = walkSuite(&suites[i], path, out)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// decodeError labels a decode failure with the report path. An unrecognised element keeps
// its own sentinel rather than collapsing into ErrMalformedReport: the file parses, so
// calling it malformed would send a human looking for broken XML that is not there.
func decodeError(err error, path string) error {
	var unknown *unknownElement
	if errors.As(err, &unknown) {
		return fmt.Errorf("report: %w: %s: <%s> inside %s is neither modelled nor known to carry nothing; it would have been dropped silently", ErrUnknownElement, path, unknown.element, unknown.parent)
	}
	return fmt.Errorf("report: %w: %s: %v", ErrMalformedReport, path, err)
}

// refuseContentAfterRoot drains what is left of the document once the root element has
// been decoded. XML permits exactly ONE root, so anything of substance after it means the
// file is not a well-formed document — and stopping at the root's end tag, as a reader that
// never drains does, turns that into a partial success with a nil error.
//
// The shape that bites is not trailing junk but a report whose runner opened it in APPEND
// mode, or wrote it twice: two concatenated <testsuite> roots. Everything after the first
// root is dropped silently, so a failing test in the second suite is not merely unreported,
// it is invisible behind a full-looking report — the false green ErrMalformedReport names.
//
// A comment, a processing instruction, a directive and whitespace are all legal after a
// root and stay accepted: Surefire and RSpec end their reports with a newline.
func refuseContentAfterRoot(dec *xml.Decoder, path string) error {
	for {
		tok, err := dec.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("report: %w: %s: %v", ErrMalformedReport, path, err)
		}
		switch t := tok.(type) {
		case xml.CharData:
			if len(bytes.TrimSpace(t)) != 0 {
				return fmt.Errorf("report: %w: %s: character data after the root element", ErrMalformedReport, path)
			}
		case xml.Comment, xml.ProcInst, xml.Directive:
			// Legal after a root element; a report may sign itself off with either.
		case xml.StartElement:
			return fmt.Errorf("report: %w: %s: a second root element <%s>: XML permits exactly one, and every suite after the first would be dropped", ErrMalformedReport, path, t.Name.Local)
		default:
			return fmt.Errorf("report: %w: %s: unexpected %T after the root element", ErrMalformedReport, path, tok)
		}
	}
}

// walkSuite appends one suite's children depth first, in the order they were written.
//
// A suite that named no test of its own but carries a suite-level <failure> or <error> is
// the collection crash: the run blew up before any test was named, and dropping it would
// read as "this suite has no tests". "Named no test of its own" is about <testcase>
// children only, not children of every kind — PHPUnit and Surefire nest <testsuite> inside
// <testsuite>, so a parent that failed at import time while one child suite still reported
// carries a nested suite and zero testcases, and gating on "no children at all" let its
// failure through as a run of passing tests. A suite that is merely empty is not an error —
// a runner writes one for a file it collected and skipped entirely — and a suite failure
// ALONGSIDE the suite's own testcases stays a summary, because those cases carry the detail.
func walkSuite(s *junitSuite, path string, out []JUnitCase) ([]JUnitCase, error) {
	if !hasOwnCase(s) {
		if detail := s.Failure; detail != nil {
			return nil, suiteFailure(path, s.Name, detail)
		}
		if detail := s.Error; detail != nil {
			return nil, suiteFailure(path, s.Name, detail)
		}
	}
	for _, child := range s.Children {
		switch {
		case child.Case != nil:
			c := child.Case
			out = append(out, JUnitCase{
				Suite:      s.Name,
				Classname:  c.Classname,
				Name:       c.Name,
				File:       c.File,
				Status:     caseStatus(*c),
				DurationMS: secondsToMS(c.Time),
			})
		case child.Suite != nil:
			var err error
			out, err = walkSuite(child.Suite, path, out)
			if err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

// hasOwnCase reports whether the suite names a <testcase> of its own. Cases inside a nested
// <testsuite> belong to that suite — the flattened output attributes them to its name — so
// they say nothing about whether THIS suite ran.
func hasOwnCase(s *junitSuite) bool {
	for _, child := range s.Children {
		if child.Case != nil {
			return true
		}
	}
	return false
}

// suiteFailure names the suite and repeats the runner's own message: with no testcase to
// point at, those two strings are the whole diagnosis.
func suiteFailure(path, suite string, detail *junitDetail) error {
	msg := detail.Message
	if msg == "" {
		msg = strings.TrimSpace(detail.Text)
	}
	return fmt.Errorf("report: %w: %s: %s: %s", ErrSuiteFailure, path, suite, msg)
}

// caseStatus folds one <testcase>'s result children into the vocabulary reportlog.go
// already produces. Precedence is error → fail → skip → pass: a case carrying both an
// <error> and a <skipped> did not pass, and an <error> is a failure and never a skip — an
// errored test did not run to a verdict, so calling it a skip tells the map a test was
// deliberately excluded when in fact it blew up.
func caseStatus(c junitCase) string {
	switch {
	case c.Error != nil:
		return "error"
	case c.Failure != nil:
		return "fail"
	case c.Skipped != nil:
		return "skip"
	}
	return "pass"
}

// MaxDurationMS is the ceiling a single testcase duration is clamped to: 24 hours in
// milliseconds. Nothing a runner reports is longer, so a bigger number is a broken
// attribute rather than a long test, and clamping keeps it a duration instead of letting
// the float-to-int conversion wrap it into the int64 minimum.
const MaxDurationMS = 24 * 60 * 60 * 1000

// secondsToMS converts a JUnit time= attribute, which is SECONDS, into the millisecond
// duration ReadReportLog records. A missing or unusable value is 0, not an error: a
// duration is telemetry and an outcome is not.
//
// Unusable is wider than "ParseFloat said no". ParseFloat also accepts "NaN", "Inf" and
// magnitudes far past int64, none of which is a duration, and converting an out-of-range
// float to int is implementation-defined in Go — in practice the int64 minimum. A negative
// DurationMS is not a fast test; it is a value that makes every ordering and budgeting sum
// the ranker builds downstream meaningless, so non-finite and negative both collapse to 0
// and an absurd magnitude clamps to MaxDurationMS. The attribute is trimmed first, because
// a runner that pads time=" 0.5 " means half a second and should not lose its telemetry to
// two spaces.
func secondsToMS(attr string) int {
	sec, err := strconv.ParseFloat(strings.TrimSpace(attr), 64)
	if err != nil || math.IsNaN(sec) || math.IsInf(sec, 0) || sec <= 0 {
		return 0
	}
	ms := math.Round(sec * 1000)
	if ms > MaxDurationMS {
		return MaxDurationMS
	}
	return int(ms)
}
