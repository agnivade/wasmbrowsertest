package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path"
	"runtime"
	"strconv"
	"strings"

	"github.com/chromedp/cdproto/inspector"
	"github.com/chromedp/cdproto/profiler"
	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	err := run(ctx, os.Args, os.Stderr, flag.CommandLine)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, errOutput io.Writer, flagSet *flag.FlagSet) (returnedErr error) {
	logger := log.New(errOutput, "[wasmbrowsertest]: ", log.LstdFlags|log.Lshortfile)
	defer func() {
		r := recover()
		if r != nil {
			returnedErr = fmt.Errorf("Panicked: %+v", r)
		}
	}()

	if len(args) < 2 {
		return errors.New("Please pass a wasm file as a parameter")
	}

	// Parse flags first
	// We need to parse flags from args[1:] because args[0] is the program name
	// However, the flag package expects to parse from os.Args (or a slice provided to Parse).
	// gentleParse effectively parses flags. We should use it to extract flags and finding the non-flag args.

	cpuProfile := flagSet.String("test.cpuprofile", "", "")
	coverageProfile := flagSet.String("test.coverprofile", "", "")
	quiet := flagSet.Bool("quiet", false, "disable printing of passing test logs")

	// Separate flags and other args
	var wasmFile string
	var passon []string

	// manually parse valid arguments to bypass flagSet.Parse error on unknown flags
	// We need to find the wasm file first to separate "args before wasm file" (flags for us)
	// and "args after/including wasm file" (flags for wasm binary? or gentleParse handles it?)

	// Revert to a simpler strategy: identifying wasm file first, then handle flags.
	// We scan for the first argument that ends in .wasm or .test and doesn't start with -

	wasmFileIndex := -1
	for i, arg := range args {
		if i == 0 {
			continue
		} // skip program name
		if !strings.HasPrefix(arg, "-") && (strings.HasSuffix(arg, ".wasm") || strings.HasSuffix(arg, ".test")) {
			wasmFileIndex = i
			break
		}
	}

	if wasmFileIndex == -1 {
		// Fallback or error?
		return errors.New("Please pass a wasm file as a parameter")
	}

	wasmFile = args[wasmFileIndex]

	// Handle our flags manually in the args before wasmFile?
	// Or just clean args to remove our flags and parse?

	var cleanArgs []string
	cleanArgs = append(cleanArgs, args[0])

	foundQuiet := false
	for i, arg := range args {
		if i == 0 {
			continue
		}
		if arg == "-quiet" || arg == "--quiet" {
			foundQuiet = true
			continue
		}
		// Also strip cpuprofile/coverprofile if handled manually?
		// No, let gentleParse and flagSet handle standard go test flags.
		cleanArgs = append(cleanArgs, arg)
	}
	*quiet = foundQuiet

	args = cleanArgs

	// Recalculate index in clean args
	wasmFileIndex = -1
	for i, arg := range cleanArgs {
		if i == 0 {
			continue
		} // skip program name
		if !strings.HasPrefix(arg, "-") && (strings.HasSuffix(arg, ".wasm") || strings.HasSuffix(arg, ".test")) {
			wasmFileIndex = i
			break
		}
	}
	if wasmFileIndex == -1 {
		return errors.New("Please pass a wasm file as a parameter")
	}

	wasmFile = cleanArgs[wasmFileIndex]

	ext := path.Ext(wasmFile)
	if ext == ".test" {
		newWasmFile := strings.Replace(wasmFile, ext, ".wasm", -1)
		err := copyFile(wasmFile, newWasmFile)
		if err != nil {
			return err
		}
		defer os.Remove(newWasmFile)
		wasmFile = newWasmFile
	}

	// gentleParse expects (flagSet, args).
	// Usage in original: passon, err := gentleParse(flagSet, args[2:])
	// Original assumed args[1] is file.

	// We should pass everything AFTER the wasm file to gentleParse?
	// OR does gentleParse handle flags that come BEFORE the wasm file too?
	// "args[2:]" in original implied everything after the file.
	// If wasmFileIndex is where the file is, then arguments for the WASM binary (flags) are likely after it.

	passonArgs := cleanArgs[wasmFileIndex+1:]
	var err error // Declare err here to avoid shadowing
	passon, err = gentleParse(flagSet, passonArgs)

	if err != nil {
		return err
	}
	passon = append([]string{wasmFile}, passon...)

	// We need to verify if cpuProfile/coverageProfile were set by gentleParse?
	// gentleParse calls flagSet.Parse().
	// If our flags are in passonArgs, gentleParse will see them.
	// BUT if `go test` passes them BEFORE the wasm file?

	// `go test -exec` behavior:
	// `wasmbrowsertest [exec-flags] <test-binary> [test-flags]`
	// If we run `go test -exec 'wasmbrowsertest -quiet'`, it becomes:
	// `wasmbrowsertest -quiet <binary> -test.v ...`
	// So `-quiet` is before the binary. `-test.v` is after.

	// We already extracted `quiet`.
	// now we need to make sure `test.cpuprofile` etc (which are standard Go flags) are caught.
	// They usually come AFTER the binary.

	// So `gentleParse` on `passonArgs` (args after binary) should work.

	if *coverageProfile != "" {
		passon = append(passon, "-test.coverprofile="+*coverageProfile)
	}
	if *cpuProfile != "" {
		passon = append(passon, "-test.cpuprofile="+*cpuProfile)
	}

	// Setup web server.
	handler, err := NewWASMServer(wasmFile, passon, *coverageProfile, logger)
	if err != nil {
		return err
	}
	url, shutdownHTTPServer, err := startHTTPServer(ctx, handler, logger)
	if err != nil {
		return err
	}
	defer shutdownHTTPServer()

	opts := chromedp.DefaultExecAllocatorOptions[:]
	if os.Getenv("WASM_HEADLESS") == "off" {
		opts = append(opts,
			chromedp.Flag("headless", false),
		)
	}

	// WSL needs the GPU disabled. See issue #10
	if runtime.GOOS == "linux" && isWSL() {
		opts = append(opts,
			chromedp.DisableGPU,
		)
	}

	// create chrome instance
	allocCtx, cancelAllocCtx := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAllocCtx()
	ctx, cancelCtx := chromedp.NewContext(allocCtx)
	defer cancelCtx()

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		handleEvent(ctx, ev, logger, *quiet)
	})

	var exitCode int
	tasks := []chromedp.Action{
		chromedp.Navigate(url),
		chromedp.WaitEnabled(`#doneButton`),
		chromedp.Evaluate(`exitCode;`, &exitCode),
	}
	if *cpuProfile != "" {
		// Prepend and append profiling tasks
		tasks = append([]chromedp.Action{
			profiler.Enable(),
			profiler.Start(),
		}, tasks...)
		tasks = append(tasks, chromedp.ActionFunc(func(ctx context.Context) error {
			profile, err := profiler.Stop().Do(ctx)
			if err != nil {
				return err
			}
			outF, err := os.Create(*cpuProfile)
			if err != nil {
				return err
			}
			defer func() {
				err = outF.Close()
				if err != nil {
					logger.Println(err)
				}
			}()

			funcMap, err := getFuncMap(wasmFile)
			if err != nil {
				return err
			}

			return WriteProfile(profile, outF, funcMap)
		}))
	}

	err = chromedp.Run(ctx, tasks...)
	if *quiet {
		if consoleFilter == nil {
			// Initialize if not done (though handleEvent should have done it if events fired)
			consoleFilter = NewConsoleFilter(*quiet, func(s string) {
				fmt.Printf("%s\n", s)
			})
		}
		consoleFilter.Flush()
	}
	if err != nil {
		// Browser did not exit cleanly. Likely failed with an uncaught error.
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("exit with status %d", exitCode)
	}
	return nil
}

func copyFile(src, dst string) error {
	srdFd, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("error in copying %s to %s: %v", src, dst, err)
	}
	defer srdFd.Close()

	dstFd, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("error in copying %s to %s: %v", src, dst, err)
	}
	defer dstFd.Close()
	_, err = io.Copy(dstFd, srdFd)
	if err != nil {
		return fmt.Errorf("error in copying %s to %s: %v", src, dst, err)
	}
	return nil
}

// handleEvent responds to different events from the browser and takes
// appropriate action.
var (
	consoleFilter *ConsoleFilter
)

func handleEvent(ctx context.Context, ev interface{}, logger *log.Logger, quiet bool) {
	if consoleFilter == nil {
		consoleFilter = NewConsoleFilter(quiet, func(s string) {
			fmt.Printf("%s\n", s)
		})
	}

	switch ev := ev.(type) {
	case *cdpruntime.EventConsoleAPICalled:
		for _, arg := range ev.Args {
			line := string([]byte(arg.Value))
			if line == "" { // If Value is not found, look for Description.
				line = arg.Description
			}
			// Any string content is quoted with double-quotes.
			// So need to treat it specially.
			s, err := strconv.Unquote(line)
			if err != nil {
				// Probably some numeric content, print it as is.
				// fmt.Printf("%s\n", line)
				s = line
			}

			consoleFilter.Add(s)
		}
	case *cdpruntime.EventExceptionThrown:
		if ev.ExceptionDetails != nil {
			consoleFilter.Flush()
			details := ev.ExceptionDetails
			fmt.Printf("%s:%d:%d %s\n", details.URL, details.LineNumber, details.ColumnNumber, details.Text)
			if details.Exception != nil {
				fmt.Printf("%s\n", details.Exception.Description)
			}
		}
	case *target.EventTargetCrashed:
		consoleFilter.Flush()
		fmt.Printf("target crashed: status: %s, error code:%d\n", ev.Status, ev.ErrorCode)
		err := chromedp.Cancel(ctx)
		if err != nil {
			logger.Printf("error in cancelling context: %v\n", err)
		}
	case *inspector.EventDetached:
		consoleFilter.Flush()
		fmt.Println("inspector detached: ", ev.Reason)
		err := chromedp.Cancel(ctx)
		if err != nil {
			logger.Printf("error in cancelling context: %v\n", err)
		}
	}
}

// isWSL returns true if the OS is WSL, false otherwise.
// This method of checking for WSL has worked since mid 2016:
// https://github.com/microsoft/WSL/issues/423#issuecomment-328526847
func isWSL() bool {
	buf, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return false
	}
	// if there was an error opening the file it must not be WSL, so ignore the error
	return bytes.Contains(buf, []byte("Microsoft"))
}
