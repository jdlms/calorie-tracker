// Package app wires the application's runtime dependencies and lifecycle.
package app

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/joshua-lamorey/calorie-counter/internal/config"
	"github.com/joshua-lamorey/calorie-counter/internal/handler"
	"github.com/joshua-lamorey/calorie-counter/internal/llm"
	"github.com/joshua-lamorey/calorie-counter/internal/service"
	"github.com/joshua-lamorey/calorie-counter/internal/store"
	"github.com/joshua-lamorey/calorie-counter/internal/telegram"
)

// App wires and runs the service.
type App struct {
	cfg    config.Config
	logger *slog.Logger
	store  *store.Store
	server *http.Server
	poller *telegram.Poller
	llm    llm.Client
	assets fs.FS
}

// New creates a new application.
func New(cfg config.Config, logger *slog.Logger, assets fs.FS) (*App, error) {
	st, err := store.Open(cfg.BoltDBPath)
	if err != nil {
		return nil, fmt.Errorf("opening store: %w", err)
	}

	llmClient, err := llm.New(cfg)
	if err != nil {
		_ = st.Close()
		return nil, fmt.Errorf("creating llm client: %w", err)
	}

	ingestService := service.NewIngestService(logger, llmClient, st)
	api := handler.NewAPI(cfg, logger, st, ingestService)
	frontend, err := handler.NewFrontendHandler(assets)
	if err != nil {
		_ = st.Close()
		return nil, fmt.Errorf("creating frontend handler: %w", err)
	}

	mux := http.NewServeMux()
	api.Register(mux)
	mux.Handle("/", frontend)

	return &App{
		cfg:    cfg,
		logger: logger,
		store:  st,
		server: &http.Server{
			Addr:         cfg.HTTPAddr,
			Handler:      mux,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		poller: telegram.NewPoller(logger, cfg.TelegramBotToken, cfg.TelegramAllowedChatID, st, service.NewTelegramHandler(ingestService)),
		llm:    llmClient,
		assets: assets,
	}, nil
}

// Run starts all application components and blocks until shutdown.
func (a *App) Run(ctx context.Context) error {
	serverErr := make(chan error, 1)
	pollerErr := make(chan error, 1)

	go func() {
		a.logger.InfoContext(ctx, "http server starting", "addr", a.cfg.HTTPAddr)
		err := a.server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			serverErr <- fmt.Errorf("serving http: %w", err)
			return
		}
		serverErr <- nil
	}()

	go func() {
		pollerErr <- a.poller.Run(ctx)
	}()

	select {
	case <-ctx.Done():
		a.logger.InfoContext(ctx, "shutdown requested")
	case err := <-serverErr:
		if err != nil {
			return err
		}
	case err := <-pollerErr:
		if err != nil {
			return fmt.Errorf("running telegram poller: %w", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := a.server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutting down http server: %w", err)
	}

	if err := a.store.Close(); err != nil {
		return fmt.Errorf("closing store: %w", err)
	}

	return nil
}
