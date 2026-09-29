package datatest

import (
	"context"
	"slices"
	"strings"

	"skillswap/backend/internal/data"
)

func (m *Memory) Universities(context.Context) ([]data.University, error) {
	return []data.University{{
		ID: DemoUniversityID, Name: "Демонстраційний університет", ShortName: "Demo Uni",
		EmailDomain: "students.example.test", City: "Київ",
	}}, nil
}

func (m *Memory) SkillCategories(context.Context) ([]data.SkillCategory, error) {
	return []data.SkillCategory{
		{ID: "20000000-0000-0000-0000-000000000001", Name: "Музика", Slug: "music"},
		{ID: "20000000-0000-0000-0000-000000000002", Name: "Дизайн", Slug: "design"},
	}, nil
}

func (m *Memory) Skills(_ context.Context, query string, limit int) ([]data.Skill, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	items := []data.Skill{}
	for _, item := range m.skills {
		if item.active && strings.Contains(strings.ToLower(item.skill.Name), strings.ToLower(query)) {
			items = append(items, item.skill)
		}
	}
	slices.SortFunc(items, func(a, b data.Skill) int { return strings.Compare(a.Name, b.Name) })
	return items[:min(limit, len(items))], nil
}

func (m *Memory) ProfileByUserID(_ context.Context, userID string) (data.Profile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.users[userID]
	if !ok {
		return data.Profile{}, data.ErrNotFound
	}
	profile := data.Profile{
		UserID:         a.user.ID,
		FirstName:      a.profile.FirstName,
		LastName:       a.profile.LastName,
		UniversityID:   a.user.UniversityID,
		UniversityName: "Демонстраційний університет",
		FacultyID:      a.profile.FacultyID,
		Course:         a.profile.Course,
		City:           a.profile.City,
		Bio:            a.profile.Bio,
		Formats:        append([]string{}, a.profile.Formats...),
		TeachingSkills: m.userSkills(a, data.TeachingList),
		LearningSkills: m.userSkills(a, data.LearningList),
	}
	for _, faculty := range m.faculties[a.user.UniversityID] {
		if faculty.ID == a.profile.FacultyID {
			profile.FacultyName = faculty.Name
		}
	}
	return profile, nil
}

func (m *Memory) userSkills(a *account, list data.SkillList) []data.UserSkill {
	items := []data.UserSkill{}
	for skillID, level := range a.skills[list] {
		skill := m.skills[skillID].skill
		items = append(items, data.UserSkill{SkillID: skillID, CategoryID: skill.CategoryID, Name: skill.Name, Level: level})
	}
	slices.SortFunc(items, func(a, b data.UserSkill) int { return strings.Compare(a.Name, b.Name) })
	return items
}

func (m *Memory) UpdateProfile(_ context.Context, userID string, update data.ProfileUpdate) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.users[userID]
	if !ok {
		return data.ErrNotFound
	}
	update.Formats = append([]string{}, update.Formats...)
	a.profile = update
	a.user.FirstName = update.FirstName
	a.user.LastName = update.LastName
	return nil
}

func (m *Memory) FacultiesByUniversity(_ context.Context, universityID string) ([]data.Faculty, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]data.Faculty{}, m.faculties[universityID]...), nil
}

func (m *Memory) ActiveSkillExists(_ context.Context, skillID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.skills[skillID].active, nil
}

func (m *Memory) AddUserSkill(_ context.Context, list data.SkillList, userID, skillID, level string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.users[userID]
	if !ok {
		return data.ErrNotFound
	}
	if _, exists := a.skills[list][skillID]; exists {
		return data.ErrSkillAlreadyAdded
	}
	a.skills[list][skillID] = level
	return nil
}

func (m *Memory) UpdateUserSkillLevel(_ context.Context, list data.SkillList, userID, skillID, level string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.users[userID]
	if !ok {
		return data.ErrNotFound
	}
	if _, exists := a.skills[list][skillID]; !exists {
		return data.ErrNotFound
	}
	a.skills[list][skillID] = level
	return nil
}

func (m *Memory) RemoveUserSkill(_ context.Context, list data.SkillList, userID, skillID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.users[userID]
	if !ok {
		return data.ErrNotFound
	}
	if _, exists := a.skills[list][skillID]; !exists {
		return data.ErrNotFound
	}
	delete(a.skills[list], skillID)
	return nil
}
