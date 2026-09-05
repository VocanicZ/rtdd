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
	// <testcase>: a suite that could not run at all. Reporting zero tests from it would be
	// a false green.
	ErrSuiteFailure = errors.New("junit-xml: suite failed before any test ran")
)

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
				// <properties>, <system-out> and the rest carry nothing this parser reads.
				if err := d.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			return nil
		}
	}
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
		var wrapper struct {
			Suites []junitSuite `xml:"testsuite"`
		}
		if err := dec.DecodeElement(&wrapper, &root); err != nil {
			return nil, fmt.Errorf("report: %w: %s: %v", ErrMalformedReport, path, err)
		}
		suites = wrapper.Suites
	case "testsuite":
		var s junitSuite
		if err := dec.DecodeElement(&s, &root); err != nil {
			return nil, fmt.Errorf("report: %w: %s: %v", ErrMalformedReport, path, err)
		}
		suites = []junitSuite{s}
	default:
		return nil, fmt.Errorf("report: %w: %s: root element is <%s>, want <testsuites> or <testsuite>", ErrMalformedReport, path, root.Name.Local)
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

// walkSuite appends one suite's children depth first, in the order they were written.
//
// A suite with no children at all but a suite-level <failure> or <error> is the collection
// crash: the run blew up before any test was named, and dropping it would read as "this
// suite has no tests". A suite that is merely empty is not an error — a runner writes one
// for a file it collected and skipped entirely.
func walkSuite(s *junitSuite, path string, out []JUnitCase) ([]JUnitCase, error) {
	if len(s.Children) == 0 {
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

// secondsToMS converts a JUnit time= attribute, which is SECONDS, into the millisecond
// duration ReadReportLog records. A missing or unparseable value is 0, not an error: a
// duration is telemetry and an outcome is not.
func secondsToMS(attr string) int {
	sec, err := strconv.ParseFloat(attr, 64)
	if err != nil {
		return 0
	}
	return int(math.Round(sec * 1000))
}
