// Command db runs reviewed schema migrations and optional demo seed data.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"skillswap/backend/internal/config"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 2 {
		return fmt.Errorf("usage: go run ./cmd/db [check|status|up|seed]")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
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
	if os.Args[1] == "check" {
		return nil
	}
	if os.Args[1] == "seed" {
		return seed(ctx, cfg.DatabaseURL)
	}
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	switch os.Args[1] {
	case "status":
		return goose.StatusContext(ctx, db, "migrations")
	case "up":
		return goose.UpContext(ctx, db, "migrations")
	default:
		return fmt.Errorf("usage: go run ./cmd/db [check|status|up|seed]")
	}
}

func seed(ctx context.Context, databaseURL string) error {
	script, err := os.ReadFile("seeds/demo.sql")
	if err != nil {
		return err
	}
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("cannot connect to database: %w", err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, string(script), pgx.QueryExecModeSimpleProtocol); err != nil {
		return fmt.Errorf("demo seed failed: %w", err)
	}
	fmt.Println("Demo catalog seeded")
	return nil
}
