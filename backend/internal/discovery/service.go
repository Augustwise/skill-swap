package discovery

import (
	"context"
	"errors"
	"slices"
	"strings"

	"skillswap/backend/internal/data"
	"skillswap/backend/internal/profile"
	"skillswap/backend/internal/validate"
)

const (
	// PageSize is the number of results on one page.
	PageSize = 20
	MaxPage  = 500

	maxQueryLength = 100
)

var (
	levels  = []string{"BEGINNER", "INTERMEDIATE", "ADVANCED"}
	formats = []string{data.FormatOnline, data.FormatOffline}
)

var ErrInvalidPage = errors.New("page is out of range")

type Service struct {
	profiles  data.IProfileData
	discovery data.IDiscoveryData
}

func NewService(profiles data.IProfileData, discovery data.IDiscoveryData) *Service {
	return &Service{profiles: profiles, discovery: discovery}
}

type MatchPage struct {
	Items    []data.Match
	Page     int
	Total    int
	Eligible bool
}

func (s *Service) MutualMatches(ctx context.Context, userID string, page int) (MatchPage, error) {
	if page < 1 || page > MaxPage {
		return MatchPage{}, ErrInvalidPage
	}
	result := MatchPage{Items: []data.Match{}, Page: page}
	viewer, err := s.profiles.ProfileByUserID(ctx, userID)
	if err != nil {
		return MatchPage{}, err
	}
	if result.Eligible = profile.EligibleForMatching(viewer); !result.Eligible {
		return result, nil
	}
	result.Items, result.Total, err = s.discovery.MutualMatches(ctx, userID, PageSize, (page-1)*PageSize)
	if err != nil {
		return MatchPage{}, err
	}
	return result, nil
}

type StudentPage struct {
	Items []data.StudentCard
	Page  int
	Total int
}

// SearchStudents applies all filters together; empty filters are ignored.
func (s *Service) SearchStudents(ctx context.Context, userID string, filter data.StudentFilter, page int) (StudentPage, error) {
	if page < 1 || page > MaxPage {
		return StudentPage{}, ErrInvalidPage
	}
	fields := map[string]string{}
	var ok bool
	if filter.Query, ok = validate.Line(filter.Query, maxQueryLength); !ok {
		fields["q"] = "Search text must be at most 100 characters"
	}
	filter.CategoryID = strings.ToLower(filter.CategoryID)
	if filter.CategoryID != "" && !validate.UUID(filter.CategoryID) {
		fields["categoryId"] = "Choose a category from the catalog"
	}
	if filter.Level != "" && !slices.Contains(levels, filter.Level) {
		fields["level"] = "Level must be BEGINNER, INTERMEDIATE or ADVANCED"
	}
	if filter.Format != "" && !slices.Contains(formats, filter.Format) {
		fields["format"] = "Format must be ONLINE or OFFLINE"
	}
	if len(fields) > 0 {
		return StudentPage{}, &profile.ValidationError{Fields: fields}
	}
	items, total, err := s.discovery.SearchStudents(ctx, userID, filter, PageSize, (page-1)*PageSize)
	if err != nil {
		return StudentPage{}, err
	}
	return StudentPage{Items: items, Page: page, Total: total}, nil
}
