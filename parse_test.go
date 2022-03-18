package main

import (
	"flag"
	"fmt"
	"os"
)

func ExampleParseEmpty() {
	testFlagSet()

	// Empty in, empty out.

	// Output:
	// cpuProfile: ""
	// passon    : []
}

func ExampleParseOther() {
	testFlagSetOther()

	// Empty in, empty out.

	// Output:
	// cpuProfile: ""
	// other     : "default-other-value"
	// passon    : []
}

func ExampleParseVerbose() {
	testFlagSet("-test.v")

	// One unrecognized in, same out.

	// Output:
	// cpuProfile: ""
	// passon    : ["-test.v"]
}

func ExampleParseCpu1() {
	testFlagSet("-test.v", "-test.cpuprofile", "cpu1.out")

	// One unrecognized followed by ours.

	// Output:
	// cpuProfile: "cpu1.out"
	// passon    : ["-test.v"]
}

func ExampleParseCpu2() {
	testFlagSet("-test.cpuprofile", "cpu2.out", "-test.v")

	// Ours followed by one unrecognized.

	// Output:
	// cpuProfile: "cpu2.out"
	// passon    : ["-test.v"]
}

func ExampleParseEqualCpu3() {
	testFlagSet("-test.cpuprofile", "cpu3.out", "-test.v=true")

	// Ours followed by one unrecognized that uses "=".

	// Output:
	// cpuProfile: "cpu3.out"
	// passon    : ["-test.v=true"]
}

func ExampleParseEqualCpu4() {
	testFlagSet("-test.v=true", "-test.cpuprofile", "cpu4.out")

	// Swapping order from Cpu3 test, the unrecognized first, followed by ours.

	// Output:
	// cpuProfile: "cpu4.out"
	// passon    : ["-test.v=true"]
}

func ExampleParseExtraBool1() {
	testFlagSet("-test.cpuprofile", "cpu.out", "-test.v", "-bool")

	// Ours followed by two unrecognized.

	// Output:
	// cpuProfile: "cpu.out"
	// passon    : ["-test.v" "-bool"]
}

func ExampleParseExtraBool2() {
	testFlagSet("-bool", "-test.cpuprofile", "cpu.out", "-test.v")

	// Ours between two unrecognized.

	// Output:
	// cpuProfile: "cpu.out"
	// passon    : ["-bool" "-test.v"]
}

func ExampleParseExtraStringNoDDash1() {
	testFlagSet("-test.cpuprofile", "cpu.out", "-test.v", "-bool", "-string", "last")

	// Ours pulled out from front.

	// Output:
	// cpuProfile: "cpu.out"
	// passon    : ["-test.v" "-bool" "-string" "last"]
}

func ExampleParseExtraStringNoDDash2() {
	testFlagSet("-string", "first", "-test.cpuprofile", "cpu.out", "-test.v", "-bool")

	// Ours pulled out from middle.

	// Output:
	// cpuProfile: "cpu.out"
	// passon    : ["-string" "first" "-test.v" "-bool"]
}

func ExampleParseDDash1ExtraString() {
	testFlagSet("-test.cpuprofile", "cpu.out", "-test.v", "--", "-bool", "-string", "abc")

	// Ours pulled out from front and the -- appears afterwards.

	// Output:
	// cpuProfile: "cpu.out"
	// passon    : ["-test.v" "--" "-bool" "-string" "abc"]
}

func ExampleParseDDash2ExtraString() {
	testFlagSet("-test.cpuprofile", "cpu.out", "--", "-test.v", "-bool", "-string", "abc")

	// Ours pulled out from front and the -- appears afterwards.

	// Output:
	// cpuProfile: "cpu.out"
	// passon    : ["--" "-test.v" "-bool" "-string" "abc"]
}

func ExampleParseDDash3UnprocessedProfile() {
	testFlagSet("--", "-test.cpuprofile", "cpu.other", "-test.v", "-bool", "-string", "abc")

	// Ours *not* pulled out because it appears after a --, just as "go test" would handle it.

	// Output:
	// cpuProfile: ""
	// passon    : ["--" "-test.cpuprofile" "cpu.other" "-test.v" "-bool" "-string" "abc"]
}

func testFlagSet(args ...string) {
	var (
		cpuProfile string
	)

	flagset := flag.NewFlagSet("binname", flag.ExitOnError)
	flagset.SetOutput(os.Stdout) // For Examples to catch as output.

	flagset.StringVar(&cpuProfile, "test.cpuprofile", "", "")

	passon := gentleParse(flagset, args)

	fmt.Printf("cpuProfile: %q\npasson    : %q\n", cpuProfile, passon)
}

func testFlagSetOther(args ...string) {
	var (
		cpuProfile string
		other      string
	)

	flagset := flag.NewFlagSet("binname", flag.ExitOnError)
	flagset.SetOutput(os.Stdout) // For Examples to catch as output.

	flagset.StringVar(&cpuProfile, "test.cpuprofile", "", "")
	flagset.StringVar(&other, "other", "default-other-value", "")

	passon := gentleParse(flagset, args)

	fmt.Printf("cpuProfile: %q\nother     : %q\npasson    : %q\n", cpuProfile, other, passon)
}
