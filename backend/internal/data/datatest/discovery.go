package datatest

import (
	"cmp"
	"context"
	"slices"
	"strings"

	"skillswap/backend/internal/data"
)

var _ data.IDiscoveryData = (*Memory)(nil)

func (m *Memory) MutualMatches(_ context.Context, viewerID string, limit, offset int) ([]data.Match, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	viewer, ok := m.users[viewerID]
	if !ok {
		return []data.Match{}, 0, nil
	}
	type ranked struct {
		match data.Match
		pairs int
	}
	var all []ranked
	for id, c := range m.users {
		if id == viewerID || !c.user.EmailVerified || c.user.Status != "ACTIVE" {
			continue
		}
		match := data.Match{
			UserID: id, FirstName: c.profile.FirstName, LastName: c.profile.LastName,
			UniversityID: c.user.UniversityID, UniversityName: "Демонстраційний університет", City: c.profile.City,
			CommonFormats: usableFormats(viewer.profile, c.profile),
			CanTeach:      m.overlap(c.skills[data.TeachingList], viewer.skills[data.LearningList]),
			WantsToLearn:  m.overlap(c.skills[data.LearningList], viewer.skills[data.TeachingList]),
		}
		if len(match.CommonFormats) > 0 && len(match.CanTeach) > 0 && len(match.WantsToLearn) > 0 {
			all = append(all, ranked{match: match, pairs: len(match.CanTeach) + len(match.WantsToLearn)})
		}
	}
	slices.SortFunc(all, func(a, b ranked) int {
		return cmp.Or(cmp.Compare(b.pairs, a.pairs), cmp.Compare(a.match.FirstName, b.match.FirstName),
			cmp.Compare(a.match.LastName, b.match.LastName), cmp.Compare(a.match.UserID, b.match.UserID))
	})
	page := []data.Match{}
	for _, r := range all[min(offset, len(all)):min(offset+limit, len(all))] {
		page = append(page, r.match)
	}
	return page, len(all), nil
}


func (m *Memory) overlap(student, viewer map[string]string) []data.UserSkill {
	items := []data.UserSkill{}
	for skillID, level := range student {
		if _, ok := viewer[skillID]; ok && m.skills[skillID].active {
			skill := m.skills[skillID].skill
			items = append(items, data.UserSkill{SkillID: skillID, CategoryID: skill.CategoryID, Name: skill.Name, Level: level})
		}
	}
	slices.SortFunc(items, func(a, b data.UserSkill) int { return strings.Compare(a.Name, b.Name) })
	return items
}

func usableFormats(viewer, student data.ProfileUpdate) []string {
	sameCity := cityKey(viewer.City) != "" && cityKey(viewer.City) == cityKey(student.City)
	formats := []string{}
	for _, format := range []string{data.FormatOnline, data.FormatOffline} {
		if slices.Contains(viewer.Formats, format) && slices.Contains(student.Formats, format) &&
			(format != data.FormatOffline || sameCity) {
			formats = append(formats, format)
		}
	}
	return formats
}

func cityKey(city string) string {
	return strings.ToLower(strings.TrimSpace(city))
}
