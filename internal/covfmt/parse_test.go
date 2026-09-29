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
