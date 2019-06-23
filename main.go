package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/cdproto/inspector"
	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

func main() {
	logger := log.New(os.Stderr, "[wasmbrowsertest]: ", log.LstdFlags|log.Lshortfile)
	if len(os.Args) < 2 {
		logger.Fatal("Please pass a wasm file as a parameter")
	}
	wasmFile := os.Args[1]
	ext := path.Ext(wasmFile)
	// net/http code does not take js/wasm path if it is a .test binary.
	if ext == ".test" {
		wasmFile = strings.Replace(wasmFile, ext, ".wasm", -1)
		err := os.Rename(os.Args[1], wasmFile)
		if err != nil {
			logger.Fatal(err)
		}
		os.Args[1] = wasmFile
	}

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
	handler, err := NewWASMServer(wasmFile, os.Args[1:], logger)
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
		switch ev := ev.(type) {
		case *cdpruntime.EventConsoleAPICalled:
			for _, arg := range ev.Args {
				line := string(arg.Value)
				if ev.StackTrace != nil && len(ev.StackTrace.CallFrames) > 0 {
					topFrame := ev.StackTrace.CallFrames[0]
					if strings.HasSuffix(topFrame.URL, "wasm_exec.js") {
						// Output from the test is quoted with double-quotes and whitespace-escaped.
						// So need to treat it specially.
						s, err := strconv.Unquote(line)
						if err != nil {
							logger.Printf("malformed string: %s\n", line)
							continue
						}
						line = s
					}
				}
				fmt.Printf("%s\n", line)
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
	err = chromedp.Run(ctx,
		chromedp.Navigate(`http://localhost:`+port),
		chromedp.WaitEnabled(`#doneButton`),
		chromedp.Evaluate(`exitCode;`, &exitCode),
	)
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
