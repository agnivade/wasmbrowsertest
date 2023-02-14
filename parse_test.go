package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
)

// The forms of these tests for the gentleParse function
// are generally that the args are presented to the testParse function
// and the expected results are presented as separate lines to the
// Expect result.

func TestParse(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		// Empty in, empty out.
		testParse(t).Expect(t,
			`cpuProfile: ""`,
			`passon    : []`,
		)
	})

	t.Run("Other", func(t *testing.T) {
		// Empty in, empty out, with an extra `other` variable that has a default.
		testParseOther(t).Expect(t,
			`cpuProfile: ""`,
			`other     : "default-other-value"`,
			`passon    : []`,
		)

		// Test parsing of a custom flag with a custom value
		testParseOther(t, "-test.v", "-test.cpuprofile", "cpu1.out", "-other=another").Expect(t,
			`cpuProfile: "cpu1.out"`,
			`other     : "another"`,
			`passon    : ["-test.v"]`,
		)
	})

	t.Run("Verbose", func(t *testing.T) {
		// One unrecognized in, same out.
		testParse(t, "-test.v").Expect(t,
			`cpuProfile: ""`,
			`passon    : ["-test.v"]`,
		)
	})

	t.Run("CPU1", func(t *testing.T) {
		// One unrecognized followed by ours.
		testParse(t, "-test.v", "-test.cpuprofile", "cpu1.out").Expect(t,
			`cpuProfile: "cpu1.out"`,
			`passon    : ["-test.v"]`,
		)
	})

	t.Run("CPU2", func(t *testing.T) {
		// Ours followed by one unrecognized.
		testParse(t, "-test.cpuprofile", "cpu2.out", "-test.v").Expect(t,
			`cpuProfile: "cpu2.out"`,
			`passon    : ["-test.v"]`,
		)
	})

	t.Run("CPU3", func(t *testing.T) {
		// Ours followed by one unrecognized that uses "=".
		testParse(t, "-test.cpuprofile", "cpu3.out", "-test.v=true").Expect(t,
			`cpuProfile: "cpu3.out"`,
			`passon    : ["-test.v=true"]`,
		)
	})

	t.Run("EqualCPU4", func(t *testing.T) {
		// Swapping order from Cpu3 test, the unrecognized first, followed by ours.
		testParse(t, "-test.v=true", "-test.cpuprofile", "cpu4.out").Expect(t,
			`cpuProfile: "cpu4.out"`,
			`passon    : ["-test.v=true"]`,
		)
	})

	t.Run("ExtraBool1", func(t *testing.T) {
		// Ours followed by two unrecognized.
		testParse(t, "-test.cpuprofile", "cpu.out", "-test.v", "-bool").Expect(t,
			`cpuProfile: "cpu.out"`,
			`passon    : ["-test.v" "-bool"]`,
		)
	})

	t.Run("ExtraBool2", func(t *testing.T) {
		// Ours between two unrecognized.
		testParse(t, "-bool", "-test.cpuprofile", "cpu.out", "-test.v").Expect(t,
			`cpuProfile: "cpu.out"`,
			`passon    : ["-bool" "-test.v"]`,
		)
	})

	t.Run("ExtraStringNoDDash1", func(t *testing.T) {
		// Ours pulled out from front.
		testParse(t, "-test.cpuprofile", "cpu.out", "-test.v", "-bool", "-string", "last").Expect(t,
			`cpuProfile: "cpu.out"`,
			`passon    : ["-test.v" "-bool" "-string" "last"]`,
		)
	})

	t.Run("ExtraStringNoDDash2", func(t *testing.T) {
		// Ours pulled out from middle.
		testParse(t, "-string", "first", "-test.cpuprofile", "cpu.out", "-test.v", "-bool").Expect(t,
			`cpuProfile: "cpu.out"`,
			`passon    : ["-string" "first" "-test.v" "-bool"]`,
		)
	})

	t.Run("DDash1ExtraString", func(t *testing.T) {
		// Ours pulled out from front and the -- appears afterwards.
		testParse(t, "-test.cpuprofile", "cpu.out", "-test.v", "--", "-bool", "-string", "abc").Expect(t,
			`cpuProfile: "cpu.out"`,
			`passon    : ["-test.v" "--" "-bool" "-string" "abc"]`,
		)
	})

	t.Run("DDash2ExtraString", func(t *testing.T) {
		// Ours pulled out from front and the -- appears afterwards.
		testParse(t, "-test.cpuprofile", "cpu.out", "--", "-test.v", "-bool", "-string", "abc").Expect(t,
			`cpuProfile: "cpu.out"`,
			`passon    : ["--" "-test.v" "-bool" "-string" "abc"]`,
		)
	})

	t.Run("DDash3UnprocessedProfile", func(t *testing.T) {
		// Ours *not* pulled out because it appears after a --, just as "go test" would handle it.
		testParse(t, "--", "-test.cpuprofile", "cpu.other", "-test.v", "-bool", "-string", "abc").Expect(t,
			`cpuProfile: ""`,
			`passon    : ["--" "-test.cpuprofile" "cpu.other" "-test.v" "-bool" "-string" "abc"]`,
		)
	})
}

type testParseGot struct {
	got []string
}

func makeParseGot(lines ...string) testParseGot {
	return testParseGot{got: lines}
}

func (g testParseGot) failure(expect []string, format string, args ...interface{}) string {
	buf := new(strings.Builder)
	fmt.Fprintf(buf, format+"\n", args...)
	fmt.Fprintf(buf, "  Got:\n")
	for i := range g.got {
		fmt.Fprintf(buf, "    %s\n", g.got[i])
	}
	fmt.Fprintf(buf, "  Expected:\n")
	for i := range expect {
		fmt.Fprintf(buf, "    %s\n", expect[i])
	}
	return buf.String()
}

func (g testParseGot) Expect(t testing.TB, expect ...string) {
	if len(g.got) != len(expect) {
		t.Helper()
		t.Errorf("%s",
			g.failure(expect, "got %d lines, expected %d", len(g.got), len(expect)))
		return
	}
	for i := range g.got {
		if g.got[i] != expect[i] {
			t.Helper()
			t.Errorf("%s",
				g.failure(expect, "at least line %d of got and expected don't match", i+1))
			return
		}
	}
}

func testParse(t *testing.T, args ...string) testParseGot {
	t.Helper()
	var (
		cpuProfile string
	)

	flagset := flag.NewFlagSet("binname", flag.ExitOnError)
	flagset.SetOutput(os.Stdout) // For Examples to catch as output.

	flagset.StringVar(&cpuProfile, "test.cpuprofile", "", "")

	passon, err := gentleParse(flagset, args)
	if err != nil {
		t.Error(err)
	}

	return makeParseGot(
		fmt.Sprintf("cpuProfile: %q", cpuProfile),
		fmt.Sprintf("passon    : %q", passon),
	)
}

// This one acts more like an example of how to perform a different type of test.
// It was perhaps useful in early stages of building unit tests but then seems
// to have gone unused except for the default, empty, case.
func testParseOther(t *testing.T, args ...string) testParseGot {
	t.Helper()
	var (
		cpuProfile string
		other      string
	)

	flagset := flag.NewFlagSet("binname", flag.ExitOnError)
	flagset.SetOutput(os.Stdout) // For Examples to catch as output.

	flagset.StringVar(&cpuProfile, "test.cpuprofile", "", "")
	flagset.StringVar(&other, "other", "default-other-value", "")

	passon, err := gentleParse(flagset, args)
	if err != nil {
		t.Error(err)
	}

	return makeParseGot(
		fmt.Sprintf("cpuProfile: %q", cpuProfile),
		fmt.Sprintf("other     : %q", other),
		fmt.Sprintf("passon    : %q", passon),
	)
}
