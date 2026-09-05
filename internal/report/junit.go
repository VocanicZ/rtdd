// This file is the SECOND parser in internal/report. The pytest-reportlog path in
// reportlog.go is not touched by it and is not refactored into a shared abstraction: one
// accumulates phases per test id, the other reads a tree, and the only thing they share is
// the outcome vocabulary they eventually produce.
package report

import (
	"bytes"
	"encoding/xml"
	"math"
	"os"
	"strconv"
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
		return nil, err
	}
	dec := xml.NewDecoder(bytes.NewReader(b))
	var root xml.StartElement
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
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
			return nil, err
		}
		suites = wrapper.Suites
	case "testsuite":
		var s junitSuite
		if err := dec.DecodeElement(&s, &root); err != nil {
			return nil, err
		}
		suites = []junitSuite{s}
	}

	var out []JUnitCase
	for i := range suites {
		out = walkSuite(&suites[i], out)
	}
	return out, nil
}

// walkSuite appends one suite's children depth first, in the order they were written.
func walkSuite(s *junitSuite, out []JUnitCase) []JUnitCase {
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
			out = walkSuite(child.Suite, out)
		}
	}
	return out
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
