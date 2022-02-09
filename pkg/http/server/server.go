package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const defaultShutdownTimeout = 10 // seconds

type serverConfig struct {
	shutdownTimeout time.Duration
}

type Option func(*serverConfig)

func WithShutdownTimeout(timeout time.Duration) Option {
	return func(c *serverConfig) {
		c.shutdownTimeout = timeout
	}
}

func Start(server *http.Server, opts ...Option) error {
	cfg := serverConfig{
		shutdownTimeout: time.Second * defaultShutdownTimeout,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	shutdown := make(chan os.Signal, 1)
	errors := make(chan error, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errors <- err
		}
	}()
	log.Printf("Server started at %s with settings:\n%+v\n", server.Addr, cfg)

	select {
	case err := <-errors:
		return fmt.Errorf("failed to listen and serve due to: %s", err)
	case <-shutdown:
		return stopGracefully(server, &cfg)
	}
}

func stopGracefully(server *http.Server, cfg *serverConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.shutdownTimeout)
	defer cancel()

	log.Print("Stopping the server...")
	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown failed due to: %s", err)
	}
	log.Print("Stopped the server successfully")
	return nil
}
