package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/cdproto/inspector"
	"github.com/chromedp/cdproto/profiler"
	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

var (
	cpuProfile *string
)

func main() {
	// NOTE: Since `os.Exit` will cause the process to exit, this defer
	// must be at the bottom of the defer stack to allow all other defer calls to
	// be called first.
	exitCode := 0
	defer func() {
		if exitCode != 0 {
			os.Exit(exitCode)
		}
	}()

	logger := log.New(os.Stderr, "[wasmbrowsertest]: ", log.LstdFlags|log.Lshortfile)
	if len(os.Args) < 2 {
		logger.Fatal("Please pass a wasm file as a parameter")
	}

	cpuProfile = flag.String("test.cpuprofile", "", "")

	wasmFile := os.Args[1]
	ext := path.Ext(wasmFile)
	// net/http code does not take js/wasm path if it is a .test binary.
	if ext == ".test" {
		wasmFile = strings.Replace(wasmFile, ext, ".wasm", -1)
		err := copyFile(os.Args[1], wasmFile)
		if err != nil {
			logger.Fatal(err)
		}
		defer os.Remove(wasmFile)
		os.Args[1] = wasmFile
	}

	passon := gentleParse(flag.CommandLine, os.Args[2:])
	passon = append([]string{wasmFile}, passon...)

	// Need to generate a random port every time for tests in parallel to run.
	l, err := net.Listen("tcp", "localhost:")
	if err != nil {
		logger.Fatal(err)
	}
	tcpL, ok := l.(*net.TCPListener)
	if !ok {
		logger.Fatal("net.Listen did not return a TCPListener")
	}
	_, port, err := net.SplitHostPort(tcpL.Addr().String())
	if err != nil {
		logger.Fatal(err)
	}

	// Setup web server.
	handler, err := NewWASMServer(wasmFile, passon, logger)
	if err != nil {
		logger.Fatal(err)
	}
	httpServer := &http.Server{
		Handler: handler,
	}

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
	allocCtx, cancelAllocCtx := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancelAllocCtx()
	ctx, cancelCtx := chromedp.NewContext(allocCtx)
	defer cancelCtx()

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		handleEvent(ctx, ev, logger)
	})

	done := make(chan struct{})
	go func() {
		err = httpServer.Serve(l)
		if err != http.ErrServerClosed {
			logger.Println(err)
		}
		done <- struct{}{}
	}()
	tasks := []chromedp.Action{
		chromedp.Navigate(`http://localhost:` + port),
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
	if err != nil {
		logger.Println(err)
	}
	if exitCode != 0 {
		exitCode = 1
	}
	// create a timeout
	ctx, cancelHTTPCtx := context.WithTimeout(ctx, 5*time.Second)
	defer cancelHTTPCtx()
	// Close shop
	err = httpServer.Shutdown(ctx)
	if err != nil {
		logger.Println(err)
	}
	<-done
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

// gentleParse takes a flag.FlagSet, calls Parse to get its flags parsed,
// and collects the arguments the FlagSet does not recognize, returning
// the collected list.
func gentleParse(flagset *flag.FlagSet, args []string) []string {
	if len(args) == 0 {
		return nil
	}

	r := make([]string, 0, len(args))

	flagset.Init(flagset.Name(), flag.ContinueOnError)
	w := flagset.Output()
	flagset.SetOutput(ioutil.Discard)

	// Put back the flagset's output, the flagset's Usage might be called later.
	defer flagset.SetOutput(w)

	next := args

	for len(next) > 0 {
		if next[0] == "--" {
			r = append(r, next...) // include the "--" for the wasm image, it's what "go test" does.
			break
		}
		if !strings.HasPrefix(next[0], "-") {
			r, next = append(r, next[0]), next[1:]
			continue
		}
		if err := flagset.Parse(next); err != nil {
			const prefix = "flag provided but not defined: "
			if strings.HasPrefix(err.Error(), prefix) {
				pull := strings.TrimPrefix(err.Error(), prefix)
				for next[0] != pull {
					next = next[1:]
					if len(next) == 0 {
						panic("odd: pull not found: " + pull)
					}
				}
				r, next = append(r, next[0]), next[1:]
				continue
			}
			fmt.Fprintf(w, "%s\n", err)
			flagset.SetOutput(w)
			flag.Usage()
			os.Exit(1)
		}

		next = flag.Args()
	}
	return r
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
