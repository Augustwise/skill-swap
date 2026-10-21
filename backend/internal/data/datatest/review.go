package datatest

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	"skillswap/backend/internal/data"
)

var _ data.IReviewData = (*Memory)(nil)

func (m *Memory) StudentVisible(_ context.Context, viewerID, studentID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.visible(viewerID, studentID, m.users[studentID]), nil
}

func (m *Memory) CreateReview(_ context.Context, in data.NewReview) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.reviews {
		if r.ExchangeID == in.ExchangeID && r.AuthorID == in.AuthorID {
			return "", data.ErrReviewExists
		}
	}
	now := time.Now().UTC()
	for _, r := range m.reviews {
		if !now.After(r.CreatedAt) {
			now = r.CreatedAt.Add(time.Microsecond)
		}
	}
	id := fmt.Sprintf("90000000-0000-0000-0000-%012d", len(m.reviews)+1)
	m.reviews[id] = &data.Review{
		ID: id, ExchangeID: in.ExchangeID, AuthorID: in.AuthorID, RecipientID: in.RecipientID,
		Rating: in.Rating, Comment: in.Comment, CreatedAt: now,
	}
	return id, nil
}

// hydrateReview must be called with m.mu held.
func (m *Memory) hydrateReview(r *data.Review) data.Review {
	out := *r
	if a, ok := m.users[r.AuthorID]; ok {
		out.AuthorFirstName, out.AuthorLastName = a.profile.FirstName, a.profile.LastName
	}
	return out
}

func (m *Memory) ReviewByID(_ context.Context, reviewID string) (data.Review, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.reviews[reviewID]
	if !ok {
		return data.Review{}, data.ErrNotFound
	}
	return m.hydrateReview(r), nil
}

// exchangeReviews must be called with m.mu held.
func (m *Memory) exchangeReviews(exchangeID string) []data.Review {
	found := []data.Review{}
	for _, r := range m.reviews {
		if r.ExchangeID == exchangeID {
			found = append(found, m.hydrateReview(r))
		}
	}
	slices.SortFunc(found, func(a, b data.Review) int {
		return cmp.Or(a.CreatedAt.Compare(b.CreatedAt), cmp.Compare(a.ID, b.ID))
	})
	return found
}

func (m *Memory) ExchangeReviews(_ context.Context, exchangeID string) ([]data.Review, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.exchangeReviews(exchangeID), nil
}

// receivedReviews must be called with m.mu held.
func (m *Memory) receivedReviews(userID string, limit, offset int) ([]data.Review, float64, int) {
	found := []data.Review{}
	sum := 0
	for _, r := range m.reviews {
		if r.RecipientID == userID {
			found = append(found, m.hydrateReview(r))
			sum += r.Rating
		}
	}
	slices.SortFunc(found, func(a, b data.Review) int {
		return cmp.Or(b.CreatedAt.Compare(a.CreatedAt), cmp.Compare(b.ID, a.ID))
	})
	total := len(found)
	average := 0.0
	if total > 0 {
		average = float64(sum) / float64(total)
	}
	return append([]data.Review{}, found[min(offset, total):min(offset+limit, total)]...), average, total
}

func (m *Memory) UserReviews(_ context.Context, userID string, limit, offset int) ([]data.Review, data.RatingSummary, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	reviews, average, total := m.receivedReviews(userID, limit, offset)
	return reviews, data.RatingSummary{AverageRating: average, ReviewCount: total}, nil
}
