package profile

import (
	"context"

	"skillswap/backend/internal/data"
)

type Service struct {
	catalog data.ICatalogData
}

func NewService(catalog data.ICatalogData) *Service {
	return &Service{catalog: catalog}
}

func (s *Service) Universities(ctx context.Context) ([]data.University, error) {
	return s.catalog.Universities(ctx)
}

func (s *Service) SkillCategories(ctx context.Context) ([]data.SkillCategory, error) {
	return s.catalog.SkillCategories(ctx)
}

func (s *Service) Skills(ctx context.Context, query string, limit int) ([]data.Skill, error) {
	return s.catalog.Skills(ctx, query, limit)
}
