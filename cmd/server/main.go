package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/joshua-lamorey/calorie-counter/internal/app"
	"github.com/joshua-lamorey/calorie-counter/internal/config"
	"github.com/joshua-lamorey/calorie-counter/web"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{}))

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	application, err := app.New(cfg, logger, web.Assets)
	if err != nil {
		return fmt.Errorf("creating app: %w", err)
	}

	if err := application.Run(ctx); err != nil {
		return fmt.Errorf("running app: %w", err)
	}

	return nil
}
