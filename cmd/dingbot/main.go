package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"realtime-message/internal/core"
	"realtime-message/internal/logging"
)

func main() {
	configPath := flag.String("config", "config.yaml", "config file path")
	flag.Parse()

	logger := logging.New(false)
	mgr := core.NewManager(*configPath, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		cancel()
	}()

	if err := mgr.Start(ctx); err != nil {
		logger.Error("startup failed", logging.Field{Key: "err", Val: err})
		os.Exit(1)
	}
}
