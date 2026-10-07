package core

import (
	"context"

	"skillswap/backend/internal/data"
	"skillswap/backend/internal/discovery"
	"skillswap/backend/internal/exchange"
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

	MutualMatches(ctx context.Context, userID string, page int) (discovery.MatchPage, error)
	SearchStudents(ctx context.Context, userID string, filter data.StudentFilter, page int) (discovery.StudentPage, error)
	StudentProfile(ctx context.Context, userID, studentID string) (data.StudentProfile, error)

	CreateRequest(ctx context.Context, userID string, request exchange.NewRequest) (data.ExchangeRequest, error)
	Requests(ctx context.Context, userID string, filter data.RequestFilter, page int) (exchange.RequestPage, error)
	RequestDetails(ctx context.Context, userID, requestID string) (exchange.RequestDetails, error)
}

type Application struct {
	profiles  *profile.Service
	discovery *discovery.Service
	exchanges *exchange.Service
}

var _ IApplication = (*Application)(nil)

func NewApplication(profiles *profile.Service, discovery *discovery.Service, exchanges *exchange.Service) *Application {
	return &Application{profiles: profiles, discovery: discovery, exchanges: exchanges}
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

func (a *Application) MutualMatches(ctx context.Context, userID string, page int) (discovery.MatchPage, error) {
	return a.discovery.MutualMatches(ctx, userID, page)
}

func (a *Application) SearchStudents(ctx context.Context, userID string, filter data.StudentFilter, page int) (discovery.StudentPage, error) {
	return a.discovery.SearchStudents(ctx, userID, filter, page)
}

func (a *Application) StudentProfile(ctx context.Context, userID, studentID string) (data.StudentProfile, error) {
	return a.discovery.StudentProfile(ctx, userID, studentID)
}

func (a *Application) CreateRequest(ctx context.Context, userID string, request exchange.NewRequest) (data.ExchangeRequest, error) {
	return a.exchanges.CreateRequest(ctx, userID, request)
}

func (a *Application) Requests(ctx context.Context, userID string, filter data.RequestFilter, page int) (exchange.RequestPage, error) {
	return a.exchanges.Requests(ctx, userID, filter, page)
}

func (a *Application) RequestDetails(ctx context.Context, userID, requestID string) (exchange.RequestDetails, error) {
	return a.exchanges.RequestDetails(ctx, userID, requestID)
}
