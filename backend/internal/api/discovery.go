package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"skillswap/backend/internal/data"
	"skillswap/backend/internal/discovery"
)

const discoveryTimeout = 5 * time.Second

type studentSummary struct {
	ID         string    `json:"id"`
	FirstName  string    `json:"firstName"`
	LastName   string    `json:"lastName"`
	University reference `json:"university"`
	City       string    `json:"city"`
}

type mutualMatch struct {
	Student       studentSummary `json:"student"`
	CanTeachYou   []userSkill    `json:"canTeachYou"`
	WantsToLearn  []userSkill    `json:"wantsToLearn"`
	CommonFormats []string       `json:"commonFormats"`
}

func toMutualMatch(m data.Match) mutualMatch {
	return mutualMatch{
		Student: studentSummary{
			ID: m.UserID, FirstName: m.FirstName, LastName: m.LastName,
			University: reference{ID: m.UniversityID, Name: m.UniversityName}, City: m.City,
		},
		CanTeachYou:   toUserSkills(m.CanTeach),
		WantsToLearn:  toUserSkills(m.WantsToLearn),
		CommonFormats: append([]string{}, m.CommonFormats...),
	}
}

func (a *API) mutualMatches(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	page, ok := pageParam(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), discoveryTimeout)
	defer cancel()
	result, err := a.app.MutualMatches(ctx, current.ID, page)
	if err != nil {
		a.discoveryError(w, err)
		return
	}
	items := make([]mutualMatch, 0, len(result.Items))
	for _, m := range result.Items {
		items = append(items, toMutualMatch(m))
	}
	respond(w, http.StatusOK, map[string]any{
		"items":               items,
		"page":                result.Page,
		"pageSize":            discovery.PageSize,
		"total":               result.Total,
		"eligibleForMatching": result.Eligible,
	})
}

// pageParam reads the optional 1-based "page" query parameter.
func pageParam(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw := r.URL.Query().Get("page")
	if raw == "" {
		return 1, true
	}
	page, err := strconv.Atoi(raw)
	if err != nil || page < 1 || page > discovery.MaxPage {
		problem(w, http.StatusBadRequest, "invalid_page", fmt.Sprintf("page must be a whole number from 1 to %d", discovery.MaxPage))
		return 0, false
	}
	return page, true
}

func (a *API) discoveryError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, discovery.ErrInvalidPage):
		problem(w, http.StatusBadRequest, "invalid_page", fmt.Sprintf("page must be a whole number from 1 to %d", discovery.MaxPage))
	case errors.Is(err, data.ErrNotFound):
		problem(w, http.StatusNotFound, "not_found", "Resource was not found")
	default:
		a.queryError(w, err)
	}
}
