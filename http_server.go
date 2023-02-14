package main

import (
	"context"
	"log"
	"net"
	"net/http"
	neturl "net/url"
	"time"
)

func startHTTPServer(ctx context.Context, handler http.Handler, logger *log.Logger) (url string, shutdown context.CancelFunc, err error) {
	// Need to generate a random port every time for tests in parallel to run.
	l, err := net.Listen("tcp", "localhost:")
	if err != nil {
		return "", nil, err
	}

	server := &http.Server{
		Handler: handler,
	}
	go func() { // serves HTTP
		err := server.Serve(l)
		if err != http.ErrServerClosed {
			logger.Println(err)
		}
	}()

	shutdownCtx, startShutdown := context.WithCancel(ctx)
	shutdownComplete := make(chan struct{}, 1)
	go func() { // waits for canceled ctx or triggered shutdown, then shuts down HTTP
		<-shutdownCtx.Done()
		shutdownTimeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := server.Shutdown(shutdownTimeoutCtx)
		if err != nil {
			logger.Println(err)
		}
		shutdownComplete <- struct{}{}
	}()

	shutdown = func() {
		startShutdown()
		<-shutdownComplete
	}
	url = (&neturl.URL{
		Scheme: "http",
		Host:   l.Addr().String(),
	}).String()
	return url, shutdown, nil
}
