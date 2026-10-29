package data

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"skillswap/backend/internal/expect"
	"skillswap/backend/migrations"
)

type readinessRow struct {
	value any
	err   error
}

func (r readinessRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	switch target := dest[0].(type) {
	case *bool:
		*target = r.value.(bool)
	case *[]int64:
		*target = r.value.([]int64)
	}
	return nil
}

type readinessReader struct {
	rows  []readinessRow
	calls int
}

func (r *readinessReader) QueryRow(context.Context, string, ...any) pgx.Row {
	row := r.rows[r.calls]
	r.calls++
	return row
}

func TestSchemaReadiness(t *testing.T) {
	required, err := migrations.Versions()
	expect.NoError(t, "read embedded migration versions", err)
	failure := errors.New("database unavailable")
	for _, tc := range []struct {
		name    string
		rows    []readinessRow
		wantErr error
	}{
		{"uninitialized database", []readinessRow{{value: false}}, ErrSchemaOutdated},
		{"connection failed", []readinessRow{{err: failure}}, failure},
		{"cannot read migration history", []readinessRow{{value: true}, {err: failure}}, failure},
		{"empty migration history", []readinessRow{{value: true}, {value: []int64{}}}, ErrSchemaOutdated},
		{"sprint seven schema", []readinessRow{{value: true}, {value: []int64{1, 2, 3, 4, 5, 6, 7}}}, ErrSchemaOutdated},
		{"missing intermediate migration", []readinessRow{{value: true}, {value: slices.Delete(slices.Clone(required), 1, 2)}}, ErrSchemaOutdated},
		{"complete schema", []readinessRow{{value: true}, {value: required}}, nil},
		{"newer schema", []readinessRow{{value: true}, {value: append(slices.Clone(required), 9999)}}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := &readinessReader{rows: tc.rows}
			err := schemaReady(context.Background(), reader)
			if tc.wantErr == nil {
				expect.NoError(t, "readiness", err)
			} else {
				expect.Error(t, "readiness", err, tc.wantErr)
			}
			expect.Equal(t, "read queries", reader.calls, len(tc.rows))
		})
	}
}

func TestReadinessAgainstPostgres(t *testing.T) {
	inRollback(t, func(ctx context.Context, store *Postgres) {
		expect.NoError(t, "create temporary migration history", func() error {
			_, err := store.conn(ctx).Exec(ctx, `CREATE TEMP TABLE goose_db_version (
				id bigserial PRIMARY KEY, version_id bigint NOT NULL, is_applied boolean NOT NULL) ON COMMIT DROP`)
			return err
		}())
		required, err := migrations.Versions()
		expect.NoError(t, "read required migrations", err)
		for _, version := range required {
			_, err = store.conn(ctx).Exec(ctx, `INSERT INTO goose_db_version (version_id, is_applied) VALUES ($1, true)`, version)
			expect.NoError(t, "record applied migration", err)
		}
		expect.NoError(t, "fully migrated readiness", store.Ready(ctx))
		_, err = store.conn(ctx).Exec(ctx, `INSERT INTO goose_db_version (version_id, is_applied) VALUES (8, false)`)
		expect.NoError(t, "record a rolled back migration", err)
		err = store.Ready(ctx)
		expect.Error(t, "rolled back migration readiness", err, ErrSchemaOutdated)
		expect.Equal(t, "missing migration is explained", strings.Contains(err.Error(), "missing: [8]"), true)
		_, err = store.conn(ctx).Exec(ctx, `INSERT INTO goose_db_version (version_id, is_applied) VALUES (8, true)`)
		expect.NoError(t, "record a reapplied migration", err)
		expect.NoError(t, "reapplied migration readiness", store.Ready(ctx))
	})
}
