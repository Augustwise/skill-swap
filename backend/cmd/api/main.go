package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"skillswap/backend/internal/api"
	"skillswap/backend/internal/auth"
	"skillswap/backend/internal/config"
	"skillswap/backend/internal/core"
	"skillswap/backend/internal/data"
	"skillswap/backend/internal/mailer"
	"skillswap/backend/internal/profile"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("API stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	store := data.NewPostgres(db)
	access := auth.NewService(store, store, mailer.NewSMTP(cfg.SMTP), auth.Config{
		FrontendOrigin:      cfg.FrontendURL,
		AllowedEmailDomains: cfg.AllowedEmailDomains,
	}, logger)
	app := core.NewApplication(profile.NewService(store))
	server := &http.Server{
		Addr:              cfg.APIAddr,
		Handler:           api.New(app, access, store, api.Settings{FrontendOrigin: cfg.FrontendURL}, logger),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	result := make(chan error, 1)
	go func() { result <- server.ListenAndServe() }()
	logger.Info("API listening", "address", cfg.APIAddr)
	select {
	case err := <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	}
}
