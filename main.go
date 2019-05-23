package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"time"

	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

func main() {
	logger := log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile)
	if len(os.Args) < 2 {
		logger.Fatal("Please pass a wasm file as a parameter")
	}
	wasmFile := os.Args[1]

	// Need to generate a random port every time for tests in parallel to run.
	port, err := rand.Int(rand.Reader, big.NewInt(2000))
	if err != nil {
		logger.Fatal(err)
	}
	// Generate a port between 5000 to 7000.
	portStr := ":" + strconv.Itoa(int(port.Int64())+5000)

	// Setup web server.
	handler, err := NewWasmServer(wasmFile, os.Args[1:], logger)
	if err != nil {
		logger.Fatal(err)
	}
	httpServer := &http.Server{
		Addr:    portStr,
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
				s, err := strconv.Unquote(string(arg.Value))
				if err != nil {
					logger.Println(err)
					continue
				}
				fmt.Printf("%s\n", s)
			}
		}
	})

	done := make(chan struct{})
	go func() {
		err := httpServer.ListenAndServe()
		if err != http.ErrServerClosed {
			logger.Println(err)
		}
		done <- struct{}{}
	}()

	exitCode := 0
	err = chromedp.Run(ctx,
		chromedp.Navigate(`http://localhost`+portStr),
		chromedp.WaitEnabled(`#doneButton`),
		chromedp.Evaluate(`exitCode;`, &exitCode),
	)
	if err != nil {
		logger.Fatal(err)
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
