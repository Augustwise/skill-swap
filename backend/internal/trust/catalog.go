package trust

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"skillswap/backend/internal/data"
	"skillswap/backend/internal/profile"
	"skillswap/backend/internal/validate"
)

const (
	// CatalogPageSize is the number of skills on one page of the administrator catalog.
	CatalogPageSize = 50

	maxSkillName        = 150
	maxSkillDescription = 500
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

type CatalogPage struct {
	Items []data.CatalogSkill
	Page  int
	Total int
}

// CatalogSkills lists every skill for administrators, including inactive ones.
func (s *Service) CatalogSkills(ctx context.Context, query string, page int) (CatalogPage, error) {
	if page < 1 || page > MaxPage {
		return CatalogPage{}, ErrInvalidPage
	}
	query, ok := validate.Line(query, maxSkillName)
	if !ok {
		return CatalogPage{}, &profile.ValidationError{Fields: map[string]string{"q": "Search text must be at most 150 characters"}}
	}
	items, total, err := s.moderation.CatalogSkills(ctx, query, CatalogPageSize, (page-1)*CatalogPageSize)
	if err != nil {
		return CatalogPage{}, err
	}
	return CatalogPage{Items: items, Page: page, Total: total}, nil
}

// SkillPatch holds the fields to change; nil keeps the current value.
type SkillPatch struct {
	CategoryID  *string
	Name        *string
	Slug        *string
	Description *string
	IsActive    *bool
}

// CreateSkill adds a skill to the shared catalog; a new skill is active unless isActive is false.
func (s *Service) CreateSkill(ctx context.Context, moderatorID string, patch SkillPatch) (data.CatalogSkill, error) {
	in, err := validSkill(applySkillPatch(data.SkillInput{IsActive: true}, patch))
	if err != nil {
		return data.CatalogSkill{}, err
	}
	var created data.CatalogSkill
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		id, err := s.moderation.CreateSkill(ctx, in)
		if err != nil {
			return catalogError(err)
		}
		if err := s.logSkill(ctx, moderatorID, id, data.ActionCreateSkill, fmt.Sprintf("Додано навичку «%s»", in.Name)); err != nil {
			return err
		}
		created, err = s.moderation.CatalogSkillByID(ctx, id)
		return err
	})
	return created, err
}

// UpdateSkill renames, moves, describes, hides or restores a catalog skill. A hidden skill
// disappears from the catalog, search and matches; agreed exchanges keep their snapshots.
func (s *Service) UpdateSkill(ctx context.Context, moderatorID, skillID string, patch SkillPatch) (data.CatalogSkill, error) {
	if !validate.UUID(skillID) {
		return data.CatalogSkill{}, ErrSkillNotFound
	}
	var updated data.CatalogSkill
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		current, err := s.moderation.CatalogSkillByID(ctx, strings.ToLower(skillID))
		switch {
		case errors.Is(err, data.ErrNotFound):
			return ErrSkillNotFound
		case err != nil:
			return err
		}
		in, err := validSkill(applySkillPatch(data.SkillInput{
			CategoryID: current.CategoryID, Name: current.Name, Slug: current.Slug,
			Description: current.Description, IsActive: current.IsActive,
		}, patch))
		if err != nil {
			return err
		}
		if err := s.moderation.UpdateSkill(ctx, current.ID, in); err != nil {
			return catalogError(err)
		}
		reason := fmt.Sprintf("Змінено навичку «%s»", in.Name)
		switch {
		case current.IsActive && !in.IsActive:
			reason = fmt.Sprintf("Навичку «%s» приховано з каталогу", in.Name)
		case !current.IsActive && in.IsActive:
			reason = fmt.Sprintf("Навичку «%s» повернено до каталогу", in.Name)
		}
		if err := s.logSkill(ctx, moderatorID, current.ID, data.ActionUpdateSkill, reason); err != nil {
			return err
		}
		updated, err = s.moderation.CatalogSkillByID(ctx, current.ID)
		return err
	})
	return updated, err
}

func (s *Service) logSkill(ctx context.Context, moderatorID, skillID, action, reason string) error {
	_, err := s.moderation.CreateModerationAction(ctx, data.NewModerationAction{
		ModeratorID: moderatorID, SkillID: skillID, Action: action, Reason: reason,
	})
	return err
}

func applySkillPatch(in data.SkillInput, patch SkillPatch) data.SkillInput {
	if patch.CategoryID != nil {
		in.CategoryID = *patch.CategoryID
	}
	if patch.Name != nil {
		in.Name = *patch.Name
	}
	if patch.Slug != nil {
		in.Slug = *patch.Slug
	}
	if patch.Description != nil {
		in.Description = *patch.Description
	}
	if patch.IsActive != nil {
		in.IsActive = *patch.IsActive
	}
	return in
}

func validSkill(in data.SkillInput) (data.SkillInput, error) {
	fields := map[string]string{}
	in.CategoryID = strings.ToLower(in.CategoryID)
	if !validate.UUID(in.CategoryID) {
		fields["categoryId"] = "Choose a category from the list"
	}
	var ok bool
	if in.Name, ok = validate.Line(in.Name, maxSkillName); !ok || in.Name == "" {
		fields["name"] = "Name is required and must be at most 150 characters"
	}
	in.Slug = strings.TrimSpace(in.Slug)
	if len(in.Slug) > maxSkillName || !slugPattern.MatchString(in.Slug) {
		fields["slug"] = "Slug must use lowercase Latin letters, digits and single hyphens, e.g. web-design"
	}
	if in.Description, ok = validate.Text(in.Description, maxSkillDescription); !ok {
		fields["description"] = "Description must be at most 500 characters"
	}
	if len(fields) > 0 {
		return data.SkillInput{}, &profile.ValidationError{Fields: fields}
	}
	return in, nil
}

func catalogError(err error) error {
	switch {
	case errors.Is(err, data.ErrSkillExists):
		return ErrSkillExists
	case errors.Is(err, data.ErrCategoryNotFound):
		return &profile.ValidationError{Fields: map[string]string{"categoryId": "Choose a category from the list"}}
	}
	return err
}
