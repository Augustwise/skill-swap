package data

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound = errors.New("not found")

	ErrEmailTaken = errors.New("email is already registered")
)

type ITransaction interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type txKey struct{}

type Postgres struct {
	pool *pgxpool.Pool
}

var (
	_ ITransaction = (*Postgres)(nil)
	_ IAuthData    = (*Postgres)(nil)
	_ ICatalogData = (*Postgres)(nil)
)

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

func (p *Postgres) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return fn(ctx)
	}
	return pgx.BeginFunc(ctx, p.pool, func(tx pgx.Tx) error {
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
}

func (p *Postgres) conn(ctx context.Context) querier {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return p.pool
}

func (p *Postgres) Ready(ctx context.Context) error {
	var migrated bool
	if err := p.conn(ctx).QueryRow(ctx, `SELECT to_regclass('public.universities') IS NOT NULL`).Scan(&migrated); err != nil {
		return err
	}
	if !migrated {
		return errors.New("database schema is not ready")
	}
	return nil
}

func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
