package main

import (
	"flag"
	"time"
)

// This is just a copy of all the flags from the testing package.
// We need to do this to parse the -cpuprofile flag because no undefined flags are allowed,
var (
	_          = flag.Bool("test.short", false, "")
	_          = flag.Bool("test.failfast", false, "")
	_          = flag.String("test.outputdir", "", "")
	_          = flag.Bool("test.v", false, "")
	_          = flag.Uint("test.count", 1, "")
	_          = flag.String("test.coverprofile", "", "")
	_          = flag.String("test.list", "", "")
	_          = flag.String("test.run", "", "")
	_          = flag.String("test.memprofile", "", "")
	_          = flag.Int("test.memprofilerate", 0, "")
	cpuProfile = flag.String("test.cpuprofile", "", "")
	_          = flag.String("test.blockprofile", "", "")
	_          = flag.Int("test.blockprofilerate", 1, "")
	_          = flag.String("test.mutexprofile", "", "")
	_          = flag.Int("test.mutexprofilefraction", 1, "")
	_          = flag.String("test.trace", "", "")
	_          = flag.Duration("test.timeout", 0, "")
	_          = flag.String("test.cpu", "", "")
	_          = flag.Int("test.parallel", 0, "")
	_          = flag.String("test.testlogfile", "", "")
	_          = flag.String("test.bench", "", "")
	_          = flag.Bool("test.benchmem", false, "")
	_          = flag.Duration("test.benchtime", time.Second, "")
)
