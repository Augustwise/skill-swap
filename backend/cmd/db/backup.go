package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	keepBackups      = 7
	backupPrefix     = "skillswap-"
	backupExtension  = ".dump"
	defaultBackupDir = ".local/backups"
)

// backup saves the database with pg_dump into BACKUP_DIR and keeps only the newest copies.
func backup(ctx context.Context, databaseURL string) error {
	dir := envOr("BACKUP_DIR", defaultBackupDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("cannot create backup directory: %w", err)
	}
	file := filepath.Join(dir, backupPrefix+time.Now().Format("20060102-150405")+backupExtension)
	start := time.Now()
	cmd, err := pgCommand(ctx, "pg_dump", databaseURL, "--format=custom", "--no-owner", "--no-privileges", "--file="+file)
	if err != nil {
		return err
	}
	if err := cmd.Run(); err != nil {
		_ = os.Remove(file)
		return fmt.Errorf("pg_dump failed: %w", err)
	}
	info, err := os.Stat(file)
	if err != nil {
		return err
	}
	fmt.Printf("Backup saved: %s (%d KB, %s)\n", file, info.Size()/1024, time.Since(start).Round(time.Millisecond))
	return removeOldBackups(dir)
}

// removeOldBackups deletes backup files beyond the newest keepBackups. The file names
// contain the creation time, so sorting them by name sorts them by age.
func removeOldBackups(dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, backupPrefix+"*"+backupExtension))
	if err != nil {
		return err
	}
	slices.Sort(files)
	for len(files) > keepBackups {
		if err := os.Remove(files[0]); err != nil {
			return fmt.Errorf("cannot remove old backup: %w", err)
		}
		fmt.Printf("Old backup removed: %s\n", files[0])
		files = files[1:]
	}
	return nil
}

// restore loads a backup into the separate local database RESTORE_DATABASE_URL and compares
// its row counts with the main database.
func restore(ctx context.Context, sourceURL, file string) error {
	targetURL := strings.TrimSpace(os.Getenv("RESTORE_DATABASE_URL"))
	if targetURL == "" {
		return errors.New("set RESTORE_DATABASE_URL to a separate local database for the restore")
	}
	target, err := url.Parse(targetURL)
	if err != nil || target.Hostname() == "" {
		return errors.New("RESTORE_DATABASE_URL must be a complete PostgreSQL URL")
	}
	if !isLoopback(target.Hostname()) {
		return errors.New("RESTORE_DATABASE_URL must point to a local database")
	}
	source, _ := url.Parse(sourceURL)
	if databaseKey(source) == databaseKey(target) {
		return errors.New("RESTORE_DATABASE_URL must differ from DATABASE_URL: the restore replaces all data in the target")
	}
	if _, err := os.Stat(file); err != nil {
		return fmt.Errorf("cannot read backup: %w", err)
	}
	start := time.Now()
	cmd, err := pgCommand(ctx, "pg_restore", targetURL,
		"--clean", "--if-exists", "--no-owner", "--no-privileges", "--single-transaction", "--exit-on-error", file)
	if err != nil {
		return err
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pg_restore failed: %w", err)
	}
	fmt.Printf("Restored %s into %s%s in %s\n", file, target.Hostname(), target.Path, time.Since(start).Round(time.Millisecond))
	return compareCounts(ctx, sourceURL, targetURL)
}

// compareCounts prints the row count of every table in both databases.
func compareCounts(ctx context.Context, sourceURL, targetURL string) error {
	sourceCounts, err := countRowsAt(ctx, sourceURL)
	if err != nil {
		return err
	}
	targetCounts, err := countRowsAt(ctx, targetURL)
	if err != nil {
		return err
	}
	differ := 0
	fmt.Printf("%-36s %10s %10s\n", "table", "source", "restored")
	for _, table := range sourceCounts.tables {
		mark := ""
		if sourceCounts.rows[table] != targetCounts.rows[table] {
			mark = "  differs"
			differ++
		}
		fmt.Printf("%-36s %10d %10d%s\n", table, sourceCounts.rows[table], targetCounts.rows[table], mark)
	}
	if differ == 0 && len(sourceCounts.tables) == len(targetCounts.tables) {
		fmt.Println("All row counts match")
	} else {
		fmt.Printf("%d tables differ; the main database may have changed after the backup\n", differ)
	}
	return nil
}

type rowCounts struct {
	tables []string
	rows   map[string]int64
}

func countRowsAt(ctx context.Context, databaseURL string) (rowCounts, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return rowCounts{}, err
	}
	defer db.Close()
	return countRows(ctx, db)
}

// countRows counts the rows of every table in the public schema.
func countRows(ctx context.Context, db *sql.DB) (rowCounts, error) {
	rows, err := db.QueryContext(ctx, `SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'public' AND table_type = 'BASE TABLE' ORDER BY table_name`)
	if err != nil {
		return rowCounts{}, fmt.Errorf("cannot list tables: %w", err)
	}
	result := rowCounts{rows: map[string]int64{}}
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			rows.Close()
			return rowCounts{}, err
		}
		result.tables = append(result.tables, table)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return rowCounts{}, err
	}
	for _, table := range result.tables {
		var n int64
		query := "SELECT count(*) FROM " + pgx.Identifier{"public", table}.Sanitize()
		if err := db.QueryRowContext(ctx, query).Scan(&n); err != nil {
			return rowCounts{}, fmt.Errorf("cannot count %s: %w", table, err)
		}
		result.rows[table] = n
	}
	return result, nil
}

func printCounts(ctx context.Context, db *sql.DB) error {
	counts, err := countRows(ctx, db)
	if err != nil {
		return err
	}
	for _, table := range counts.tables {
		fmt.Printf("%-36s %10d\n", table, counts.rows[table])
	}
	return nil
}

// pgCommand prepares a PostgreSQL client program for databaseURL. The password is passed
// through PGPASSWORD so that it does not appear in the process list.
func pgCommand(ctx context.Context, tool, databaseURL string, args ...string) (*exec.Cmd, error) {
	path, err := pgToolPath(tool)
	if err != nil {
		return nil, err
	}
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		return nil, errors.New("database URL is invalid")
	}
	env := os.Environ()
	if parsed.User != nil {
		if password, ok := parsed.User.Password(); ok {
			env = append(env, "PGPASSWORD="+password)
			parsed.User = url.User(parsed.User.Username())
		}
	}
	cmd := exec.CommandContext(ctx, path, append(args, "--dbname="+parsed.String())...)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd, nil
}

// pgToolPath finds a PostgreSQL client program in PG_BIN, or in PATH when PG_BIN is not set.
func pgToolPath(tool string) (string, error) {
	name := tool
	if dir := strings.TrimSpace(os.Getenv("PG_BIN")); dir != "" {
		name = filepath.Join(dir, tool)
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%s was not found: add the PostgreSQL bin directory to PATH or set PG_BIN", tool)
	}
	return path, nil
}

// databaseKey identifies a database by host, port and name; all loopback addresses are the same host.
func databaseKey(u *url.URL) string {
	host := strings.ToLower(u.Hostname())
	if isLoopback(host) {
		host = "loopback"
	}
	port := u.Port()
	if port == "" {
		port = "5432"
	}
	return host + ":" + port + u.Path
}

func isLoopback(host string) bool {
	ip := net.ParseIP(host)
	return strings.EqualFold(host, "localhost") || (ip != nil && ip.IsLoopback())
}

func envOr(key, fallback string) string {
	if text := strings.TrimSpace(os.Getenv(key)); text != "" {
		return text
	}
	return fallback
}
