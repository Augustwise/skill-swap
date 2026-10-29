package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"skillswap/backend/internal/config"
)

const usage = "usage: go run ./cmd/db [check|status|up|seed|seed-load|counts|backup|restore FILE]"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return errors.New(usage)
	}
	command := os.Args[1]
	wantArgs := 2
	if command == "restore" {
		wantArgs = 3
	}
	if len(os.Args) != wantArgs {
		return errors.New(usage)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	timeout := 2 * time.Minute
	if command == "backup" || command == "restore" || command == "seed-load" {
		timeout = 30 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("cannot initialize database connection")
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("cannot connect to database: %w", err)
	}
	parsed, _ := url.Parse(cfg.DatabaseURL)
	var database, role string
	if err := db.QueryRowContext(ctx, "SELECT current_database(), current_user").Scan(&database, &role); err != nil {
		return fmt.Errorf("cannot identify database: %w", err)
	}
	fmt.Printf("Target: %s / %s (role %s)\n", parsed.Hostname(), database, role)
	switch command {
	case "check":
		return nil
	case "seed":
		return seed(ctx, cfg.DatabaseURL, "seeds/demo.sql", "Database cleared and test data seeded")
	case "seed-load":
		return seed(ctx, cfg.DatabaseURL, "seeds/load.sql", "Load test students seeded")
	case "counts":
		return printCounts(ctx, db)
	case "backup":
		return backup(ctx, cfg.DatabaseURL)
	case "restore":
		return restore(ctx, cfg.DatabaseURL, os.Args[2])
	}
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	switch command {
	case "status":
		return goose.StatusContext(ctx, db, "migrations")
	case "up":
		return goose.UpContext(ctx, db, "migrations")
	default:
		return errors.New(usage)
	}
}

func seed(ctx context.Context, databaseURL, file, done string) error {
	script, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("cannot connect to database: %w", err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, string(script), pgx.QueryExecModeSimpleProtocol); err != nil {
		return fmt.Errorf("seed %s failed: %w", file, err)
	}
	fmt.Println(done)
	return nil
}
