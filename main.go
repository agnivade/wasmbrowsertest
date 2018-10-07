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

	"github.com/sclevine/agouti"
)

func main() {
	chromeDriverPath := os.Getenv("CHROME_DRIVER") // "drivers/chromedriver-linux64-2.35"
	if chromeDriverPath == "" {
		log.Fatal("env var CHROME_DRIVER is not set.")
	}

	assetFolder := os.Getenv("ASSET_FOLDER") // "./assets"
	if assetFolder == "" {
		log.Fatal("env var ASSET_FOLDER is not set.")
	}

	if len(os.Args) < 2 {
		log.Fatal("Please pass a wasm file as a parameter")
	}
	wasmFile := os.Args[1]

	// Need to generate a random port every time for tests in parallel to run.
	port, err := rand.Int(rand.Reader, big.NewInt(2000))
	if err != nil {
		log.Fatal(err)
	}
	portStr := ":" + strconv.Itoa(int(port.Int64())+5000)

	// Setup web server.
	handler, err := getHandler(assetFolder, wasmFile, os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
	httpServer := &http.Server{
		Addr:    portStr,
		Handler: handler,
	}

	go func() {
		err := httpServer.ListenAndServe()
		if err != http.ErrServerClosed {
			log.Println(err)
		}
	}()

	opts := []agouti.Option{
		agouti.ChromeOptions(
			"args", []string{
				"headless",
				"verbose",
				"unlimited-quota-for-files",
			},
		),
		agouti.Desired(
			agouti.Capabilities{
				"loggingPrefs": map[string]string{
					"browser": "INFO",
				},
			},
		),
	}

	driver := agouti.NewWebDriver("http://{{.Address}}", []string{chromeDriverPath, "--port={{.Port}}"}, opts...)
	if err := driver.Start(); err != nil {
		log.Fatal("Failed to start Selenium:", err)
	}
	page, err := driver.NewPage(agouti.Browser("chrome"))
	if err != nil {
		log.Fatal("Failed to open page:", err)
	}

	if err := page.Navigate("http://localhost" + portStr); err != nil {
		log.Fatal("Failed to navigate:", err)
	}

	// Wait to finish.
	goExited := false
	for !goExited {
		page.RunScript("return go.exited;", nil, &goExited)
		if !goExited {
			time.Sleep(500 * time.Millisecond)
		}
	}

	// Capture logs.
	logs, err := page.ReadAllLogs("browser")
	if err != nil {
		log.Fatal(err)
	}
	for _, l := range logs {
		fmt.Println(l.Message)
	}

	exitCode := 0
	page.RunScript("return go.exitCode;", nil, &exitCode)
	if exitCode != 0 {
		// Test has failed. Set the exit code to 1 on the way out after closing all resources.
		defer os.Exit(1)
	}

	// Close shop.
	if err := driver.Stop(); err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	err = httpServer.Shutdown(ctx)
	if err != nil {
		log.Println(err)
	}
	cancel()
}
