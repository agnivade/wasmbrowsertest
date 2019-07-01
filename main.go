package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/cdproto/inspector"
	"github.com/chromedp/cdproto/profiler"
	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

func main() {
	logger := log.New(os.Stderr, "[wasmbrowsertest]: ", log.LstdFlags|log.Lshortfile)
	if len(os.Args) < 2 {
		logger.Fatal("Please pass a wasm file as a parameter")
	}

	initFlags()

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
	// We create a copy of the args to pass to NewWASMServer, because flag.Parse needs the
	// 2nd argument (the binary name) removed before being called.
	// This is an effect of "go test" passing all the arguments _after_ the binary name.
	argsCopy := append([]string(nil), os.Args...)
	// Remove the 2nd argument.
	os.Args = append(os.Args[:1], os.Args[2:]...)
	flag.Parse()

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
	handler, err := NewWASMServer(wasmFile, filterCPUProfile(argsCopy[1:]), logger)
	if err != nil {
		logger.Fatal(err)
	}
	httpServer := &http.Server{
		Handler: handler,
	}

	// create chrome instance
	ctx, cancel := chromedp.NewContext(
		context.Background(),
	)
	defer cancel()

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		handleEvent(ctx, ev, logger)
	})

	done := make(chan struct{})
	go func() {
		err := httpServer.Serve(l)
		if err != http.ErrServerClosed {
			logger.Println(err)
		}
		done <- struct{}{}
	}()

	exitCode := 0
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
				err := outF.Close()
				if err != nil {
					logger.Println(err)
				}
			}()

			// funcMap, err := getFuncMap(wasmFile)

			return WriteProfile(profile, outF)
		}))
	}

	err = chromedp.Run(ctx, tasks...)
	if err != nil {
		logger.Println(err)
	}
	if exitCode != 0 {
		defer os.Exit(1)
	}
	// create a timeout
	ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// Close shop
	err = httpServer.Shutdown(ctx)
	if err != nil {
		logger.Println(err)
	}
	<-done
}

// filterCPUProfile removes the cpuprofile argument if passed.
// CPUProfile is taken from the chromedp driver.
// So it is valid to pass such an argument, but the wasm binary will throw an error
// since file i/o is not supported inside the browser.
func filterCPUProfile(args []string) []string {
	tmp := args[:0]
	for _, x := range args {
		if strings.Contains(x, "test.cpuprofile") {
			continue
		}
		tmp = append(tmp, x)
	}
	return tmp
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
		if ev.ExceptionDetails != nil && ev.ExceptionDetails.Exception != nil {
			fmt.Printf("%s\n", ev.ExceptionDetails.Exception.Description)
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
