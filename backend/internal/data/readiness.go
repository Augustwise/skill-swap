package data

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"

	"skillswap/backend/migrations"
)

var ErrSchemaOutdated = errors.New("database schema is outdated; apply pending migrations with go run ./cmd/db up from backend")

type schemaReader interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (p *Postgres) Ready(ctx context.Context) error {
	return schemaReady(ctx, p.conn(ctx))
}

func schemaReady(ctx context.Context, conn schemaReader) error {
	var initialized bool
	if err := conn.QueryRow(ctx, `SELECT to_regclass('public.universities') IS NOT NULL
		AND to_regclass('public.goose_db_version') IS NOT NULL`).Scan(&initialized); err != nil {
		return err
	}
	if !initialized {
		return ErrSchemaOutdated
	}
	var applied []int64
	if err := conn.QueryRow(ctx, `SELECT COALESCE(array_agg(version_id ORDER BY version_id)
		FILTER (WHERE is_applied AND version_id > 0), '{}'::bigint[])
		FROM (SELECT DISTINCT ON (version_id) version_id, is_applied
			FROM goose_db_version ORDER BY version_id, id DESC) latest`).Scan(&applied); err != nil {
		return err
	}
	required, err := migrations.Versions()
	if err != nil {
		return err
	}
	var missing []int64
	for _, version := range required {
		if !slices.Contains(applied, version) {
			missing = append(missing, version)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w (missing: %v)", ErrSchemaOutdated, missing)
	}
	return nil
}
