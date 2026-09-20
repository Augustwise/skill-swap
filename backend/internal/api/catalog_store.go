package api

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

var errSchemaNotReady = errors.New("database schema is not ready")

type catalogStore interface {
	Ready(context.Context) error
	Universities(context.Context) ([]university, error)
	SkillCategories(context.Context) ([]skillCategory, error)
	Skills(context.Context, string, int) ([]skill, error)
}

type postgresCatalogStore struct {
	db *pgxpool.Pool
}

func newPostgresCatalogStore(db *pgxpool.Pool) postgresCatalogStore {
	return postgresCatalogStore{db: db}
}

func (s postgresCatalogStore) Ready(ctx context.Context) error {
	var migrated bool
	if err := s.db.QueryRow(ctx, `SELECT to_regclass('public.universities') IS NOT NULL`).Scan(&migrated); err != nil {
		return err
	}
	if !migrated {
		return errSchemaNotReady
	}
	return nil
}

func (s postgresCatalogStore) Universities(ctx context.Context) ([]university, error) {
	rows, err := s.db.Query(ctx, `SELECT id, name, COALESCE(short_name, ''), email_domain, COALESCE(city, '')
		FROM universities ORDER BY name, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]university, 0)
	for rows.Next() {
		var item university
		if err := rows.Scan(&item.ID, &item.Name, &item.ShortName, &item.EmailDomain, &item.City); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s postgresCatalogStore) SkillCategories(ctx context.Context) ([]skillCategory, error) {
	rows, err := s.db.Query(ctx, `SELECT id, name, slug FROM skill_categories
		WHERE is_active = true ORDER BY sort_order, name, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]skillCategory, 0)
	for rows.Next() {
		var item skillCategory
		if err := rows.Scan(&item.ID, &item.Name, &item.Slug); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s postgresCatalogStore) Skills(ctx context.Context, query string, limit int) ([]skill, error) {
	rows, err := s.db.Query(ctx, `SELECT s.id, s.category_id, s.name, s.slug FROM skills s
		JOIN skill_categories c ON c.id = s.category_id
		WHERE s.is_active = true AND c.is_active = true
		AND ($1 = '' OR strpos(lower(s.name), lower($1)) > 0 OR strpos(lower(s.slug), lower($1)) > 0)
		ORDER BY s.name, s.id LIMIT $2`, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]skill, 0)
	for rows.Next() {
		var item skill
		if err := rows.Scan(&item.ID, &item.CategoryID, &item.Name, &item.Slug); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
