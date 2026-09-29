package covfmt

import (
	"reflect"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		format, in string
		want       Lines
	}{
		{"lcov", "TN:\nSF:/r/src/a.py\nDA:1,1\nDA:2,0\nDA:3,4,abc\nend_of_record\nSF:src/b.py\nDA:7,1\nend_of_record\n",
			Lines{"/r/src/a.py": {1, 3}, "src/b.py": {7}}},
		{"gocover", "mode: set\nexample.com/m/calc/calc.go:3.20,5.2 1 1\nexample.com/m/calc/calc.go:7.20,9.2 1 0\nexample.com/m/util.go:1.1,2.2 1 3\n",
			Lines{"example.com/m/calc/calc.go": {3, 4, 5}, "example.com/m/util.go": {1, 2}}},
		{"cobertura", `<?xml version="1.0"?><coverage><sources><source>/r/src</source></sources><packages><package><classes>
<class filename="Calc.cs"><lines><line number="4" hits="2"/><line number="5" hits="0"/></lines></class>
</classes></package></packages></coverage>`,
			Lines{"/r/src/Calc.cs": {4}}},
		{"jacoco", `<?xml version="1.0"?><!DOCTYPE report PUBLIC "-//JACOCO//DTD Report 1.1//EN" "report.dtd"><report name="x"><package name="com/foo">
<sourcefile name="Bar.java"><line nr="3" mi="0" ci="2" mb="0" cb="0"/><line nr="4" mi="1" ci="0" mb="0" cb="0"/></sourcefile>
</package></report>`,
			Lines{"com/foo/Bar.java": {3}}},
	}
	for _, c := range cases {
		t.Run(c.format, func(t *testing.T) {
			got, err := Parse(c.format, strings.NewReader(c.in))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("Parse(%s) = %v, want %v", c.format, got, c.want)
			}
		})
	}
}

func TestParseRejectsUnknownFormatAndGarbage(t *testing.T) {
	if _, err := Parse("sqlite", strings.NewReader("")); err == nil {
		t.Error("unknown format parsed without error")
	}
	if _, err := Parse("cobertura", strings.NewReader("<not-xml")); err == nil {
		t.Error("malformed cobertura parsed without error")
	}
	if _, err := Parse("gocover", strings.NewReader("no mode line\n")); err == nil {
		t.Error("gocover without a mode line parsed without error")
	}
}

func TestParseJacocoDefaultPackage(t *testing.T) {
	// JaCoCo with empty package name produces "Foo.java", not "/Foo.java".
	in := `<?xml version="1.0"?><!DOCTYPE report PUBLIC "-//JACOCO//DTD Report 1.1//EN" "report.dtd"><report name="x"><package name="">
<sourcefile name="Foo.java"><line nr="1" mi="0" ci="1" mb="0" cb="0"/></sourcefile>
</package></report>`
	got, err := Parse("jacoco", strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	want := Lines{"Foo.java": {1}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Parse(jacoco, default package) = %v, want %v", got, want)
	}
}

func TestParseLcovCRLF(t *testing.T) {
	// LCOV with CRLF line endings should be handled by TrimSpace.
	in := "TN:\r\nSF:/r/src/a.py\r\nDA:1,1\r\nDA:2,0\r\nend_of_record\r\n"
	got, err := Parse("lcov", strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	want := Lines{"/r/src/a.py": {1}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Parse(lcov, CRLF) = %v, want %v", got, want)
	}
}

func TestParseGocoverHugeSpan(t *testing.T) {
	// gocover with a span > 100000 lines should be skipped.
	in := "mode: set\nexample.com/m/huge.go:1.1,100001.1 1 1\n"
	got, err := Parse("gocover", strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	want := Lines{} // huge span skipped, no lines hit
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Parse(gocover, huge span) = %v, want %v (empty)", got, want)
	}
}
