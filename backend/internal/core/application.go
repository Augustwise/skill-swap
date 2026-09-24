package core

import (
	"context"

	"skillswap/backend/internal/data"
	"skillswap/backend/internal/profile"
)

type IApplication interface {
	Universities(ctx context.Context) ([]data.University, error)
	SkillCategories(ctx context.Context) ([]data.SkillCategory, error)
	Skills(ctx context.Context, query string, limit int) ([]data.Skill, error)
}

type Application struct {
	profiles *profile.Service
}

var _ IApplication = (*Application)(nil)

func NewApplication(profiles *profile.Service) *Application {
	return &Application{profiles: profiles}
}

func (a *Application) Universities(ctx context.Context) ([]data.University, error) {
	return a.profiles.Universities(ctx)
}

func (a *Application) SkillCategories(ctx context.Context) ([]data.SkillCategory, error) {
	return a.profiles.SkillCategories(ctx)
}

func (a *Application) Skills(ctx context.Context, query string, limit int) ([]data.Skill, error) {
	return a.profiles.Skills(ctx, query, limit)
}
