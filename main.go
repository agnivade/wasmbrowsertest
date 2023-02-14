package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
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

	cpuProfile := flagSet.String("test.cpuprofile", "", "")
	coverageProfile := flagSet.String("test.coverprofile", "", "")

	wasmFile := args[1]
	ext := path.Ext(wasmFile)
	// net/http code does not take js/wasm path if it is a .test binary.
	if ext == ".test" {
		wasmFile = strings.Replace(wasmFile, ext, ".wasm", -1)
		err := copyFile(args[1], wasmFile)
		if err != nil {
			return err
		}
		defer os.Remove(wasmFile)
		args[1] = wasmFile
	}

	passon, err := gentleParse(flagSet, args[2:])
	if err != nil {
		return err
	}
	passon = append([]string{wasmFile}, passon...)
	if *coverageProfile != "" {
		passon = append(passon, "-test.coverprofile="+*coverageProfile)
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
		handleEvent(ctx, ev, logger)
	})

	var exitCode int
	var coverageProfileContents string
	tasks := []chromedp.Action{
		chromedp.Navigate(url),
		chromedp.WaitEnabled(`#doneButton`),
		chromedp.Evaluate(`exitCode;`, &exitCode),
		chromedp.Evaluate(`coverageProfileContents;`, &coverageProfileContents),
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
	if *coverageProfile != "" {
		tasks = append(tasks, chromedp.ActionFunc(func(ctx context.Context) error {
			return os.WriteFile(*coverageProfile, []byte(coverageProfileContents), 0644)
		}))
	}

	err = chromedp.Run(ctx, tasks...)
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
func handleEvent(ctx context.Context, ev interface{}, logger *log.Logger) {
	switch ev := ev.(type) {
	case *cdpruntime.EventConsoleAPICalled:
		for _, arg := range ev.Args {
			line := string(arg.Value)
			if line == "" { // If Value is not found, look for Description.
				line = arg.Description
			}
			// Any string content is quoted with double-quotes.
			// So need to treat it specially.
			s, err := strconv.Unquote(line)
			if err != nil {
				// Probably some numeric content, print it as is.
				fmt.Printf("%s\n", line)
				continue
			}
			fmt.Printf("%s\n", s)
		}
	case *cdpruntime.EventExceptionThrown:
		if ev.ExceptionDetails != nil {
			details := ev.ExceptionDetails
			fmt.Printf("%s:%d:%d %s\n", details.URL, details.LineNumber, details.ColumnNumber, details.Text)
			if details.Exception != nil {
				fmt.Printf("%s\n", details.Exception.Description)
			}
			err := chromedp.Cancel(ctx)
			if err != nil {
				logger.Printf("error in cancelling context: %v\n", err)
			}
		}
	case *target.EventTargetCrashed:
		fmt.Printf("target crashed: status: %s, error code:%d\n", ev.Status, ev.ErrorCode)
		err := chromedp.Cancel(ctx)
		if err != nil {
			logger.Printf("error in cancelling context: %v\n", err)
		}
	case *inspector.EventDetached:
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
	buf, err := ioutil.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return false
	}
	// if there was an error opening the file it must not be WSL, so ignore the error
	return bytes.Contains(buf, []byte("Microsoft"))
}
