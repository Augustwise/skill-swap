package core

import (
	"context"

	"skillswap/backend/internal/data"
	"skillswap/backend/internal/profile"
)

type IApplication interface {
	Universities(ctx context.Context) ([]data.University, error)
	Faculties(ctx context.Context, universityID string) ([]data.Faculty, error)
	SkillCategories(ctx context.Context) ([]data.SkillCategory, error)
	Skills(ctx context.Context, query string, limit int) ([]data.Skill, error)

	Profile(ctx context.Context, userID string) (data.Profile, error)
	UpdateProfile(ctx context.Context, userID string, patch profile.ProfilePatch) (data.Profile, error)
	AddSkill(ctx context.Context, userID string, list data.SkillList, skillID, level string) (data.Profile, error)
	UpdateSkillLevel(ctx context.Context, userID string, list data.SkillList, skillID, level string) (data.Profile, error)
	RemoveSkill(ctx context.Context, userID string, list data.SkillList, skillID string) (data.Profile, error)
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

func (a *Application) Faculties(ctx context.Context, universityID string) ([]data.Faculty, error) {
	return a.profiles.Faculties(ctx, universityID)
}

func (a *Application) SkillCategories(ctx context.Context) ([]data.SkillCategory, error) {
	return a.profiles.SkillCategories(ctx)
}

func (a *Application) Skills(ctx context.Context, query string, limit int) ([]data.Skill, error) {
	return a.profiles.Skills(ctx, query, limit)
}

func (a *Application) Profile(ctx context.Context, userID string) (data.Profile, error) {
	return a.profiles.Profile(ctx, userID)
}

func (a *Application) UpdateProfile(ctx context.Context, userID string, patch profile.ProfilePatch) (data.Profile, error) {
	return a.profiles.UpdateProfile(ctx, userID, patch)
}

func (a *Application) AddSkill(ctx context.Context, userID string, list data.SkillList, skillID, level string) (data.Profile, error) {
	return a.profiles.AddSkill(ctx, userID, list, skillID, level)
}

func (a *Application) UpdateSkillLevel(ctx context.Context, userID string, list data.SkillList, skillID, level string) (data.Profile, error) {
	return a.profiles.UpdateSkillLevel(ctx, userID, list, skillID, level)
}

func (a *Application) RemoveSkill(ctx context.Context, userID string, list data.SkillList, skillID string) (data.Profile, error) {
	return a.profiles.RemoveSkill(ctx, userID, list, skillID)
}
