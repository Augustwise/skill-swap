package data

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type University struct {
	ID          string
	Name        string
	ShortName   string
	EmailDomain string
	City        string
}

type SkillCategory struct {
	ID   string
	Name string
	Slug string
}

type Skill struct {
	ID         string
	CategoryID string
	Name       string
	Slug       string
}

type ICatalogData interface {
	Universities(ctx context.Context) ([]University, error)
	SkillCategories(ctx context.Context) ([]SkillCategory, error)
	Skills(ctx context.Context, query string, limit int) ([]Skill, error)
}

func (p *Postgres) Universities(ctx context.Context) ([]University, error) {
	rows, err := p.conn(ctx).Query(ctx, `SELECT id, name, COALESCE(short_name, ''), email_domain, COALESCE(city, '')
		FROM universities ORDER BY name, id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (University, error) {
		var item University
		err := row.Scan(&item.ID, &item.Name, &item.ShortName, &item.EmailDomain, &item.City)
		return item, err
	})
}

func (p *Postgres) SkillCategories(ctx context.Context) ([]SkillCategory, error) {
	rows, err := p.conn(ctx).Query(ctx, `SELECT id, name, slug FROM skill_categories
		WHERE is_active = true ORDER BY sort_order, name, id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (SkillCategory, error) {
		var item SkillCategory
		err := row.Scan(&item.ID, &item.Name, &item.Slug)
		return item, err
	})
}

func (p *Postgres) Skills(ctx context.Context, query string, limit int) ([]Skill, error) {
	rows, err := p.conn(ctx).Query(ctx, `SELECT s.id, s.category_id, s.name, s.slug FROM skills s
		JOIN skill_categories c ON c.id = s.category_id
		WHERE s.is_active = true AND c.is_active = true
		AND ($1 = '' OR strpos(lower(s.name), lower($1)) > 0 OR strpos(lower(s.slug), lower($1)) > 0)
		ORDER BY s.name, s.id LIMIT $2`, query, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Skill, error) {
		var item Skill
		err := row.Scan(&item.ID, &item.CategoryID, &item.Name, &item.Slug)
		return item, err
	})
}
