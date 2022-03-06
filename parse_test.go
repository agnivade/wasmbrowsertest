package main

import (
	"flag"
	"fmt"
	"os"
)

func ExampleEmpty() {
	testFlagSet()

	// Empty in, empty out.

	// Output:
	// cpuProfile: ""
	// passon    : []
}

func ExampleVerbose() {
	testFlagSet("-test.v")

	// One unrecognized in, same out.

	// Output:
	// cpuProfile: ""
	// passon    : ["-test.v"]
}

func ExampleCpu1() {
	testFlagSet("-test.v", "-test.cpuprofile", "cpu1.out")

	// One unrecognized followed by ours.

	// Output:
	// cpuProfile: "cpu1.out"
	// passon    : ["-test.v"]
}

func ExampleCpu2() {
	testFlagSet("-test.cpuprofile", "cpu2.out", "-test.v")

	// Ours followed by one unrecognized.

	// Output:
	// cpuProfile: "cpu2.out"
	// passon    : ["-test.v"]
}

func ExampleExtraBool1() {
	testFlagSet("-test.cpuprofile", "cpu.out", "-test.v", "-bool")

	// Ours followed by two unrecognized.

	// Output:
	// cpuProfile: "cpu.out"
	// passon    : ["-test.v" "-bool"]
}

func ExampleExtraBool2() {
	testFlagSet("-bool", "-test.cpuprofile", "cpu.out", "-test.v")

	// Ours between two unrecognized.

	// Output:
	// cpuProfile: "cpu.out"
	// passon    : ["-bool" "-test.v"]
}

func ExampleExtraStringNoDDash1() {
	testFlagSet("-test.cpuprofile", "cpu.out", "-test.v", "-bool", "-string", "last")

	// Ours pulled out from front.

	// Output:
	// cpuProfile: "cpu.out"
	// passon    : ["-test.v" "-bool" "-string" "last"]
}

func ExampleExtraStringNoDDash2() {
	testFlagSet("-string", "first", "-test.cpuprofile", "cpu.out", "-test.v", "-bool")

	// Ours pulled out from middle.

	// Output:
	// cpuProfile: "cpu.out"
	// passon    : ["-string" "first" "-test.v" "-bool"]
}

func ExampleExtraStringWithDDash() {
	testFlagSet("-test.cpuprofile", "cpu.out", "-test.v", "--", "-bool", "-string", "abc")

	// Ours pulled out from front and the -- appears afterwards.

	// Output:
	// cpuProfile: "cpu.out"
	// passon    : ["-test.v" "--" "-bool" "-string" "abc"]
}

func ExampleUnprocessedProfile() {
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
