package discovery

import (
	"context"
	"errors"

	"skillswap/backend/internal/data"
	"skillswap/backend/internal/profile"
)

const (
	// PageSize is the number of results on one page.
	PageSize = 20
	MaxPage = 500
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
