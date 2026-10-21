package api

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"time"

	"skillswap/backend/internal/data"
	"skillswap/backend/internal/profile"
	"skillswap/backend/internal/trust"
)

const reviewTimeout = 5 * time.Second

type reviewResponse struct {
	ID         string    `json:"id"`
	ExchangeID string    `json:"exchangeId"`
	Author     person    `json:"author"`
	Rating     int       `json:"rating"`
	Comment    string    `json:"comment"`
	CreatedAt  time.Time `json:"createdAt"`
}

func toReview(r data.Review) reviewResponse {
	return reviewResponse{
		ID: r.ID, ExchangeID: r.ExchangeID,
		Author: person{ID: r.AuthorID, FirstName: r.AuthorFirstName, LastName: r.AuthorLastName},
		Rating: r.Rating, Comment: r.Comment, CreatedAt: r.CreatedAt,
	}
}

func toReviews(reviews []data.Review) []reviewResponse {
	items := make([]reviewResponse, 0, len(reviews))
	for _, r := range reviews {
		items = append(items, toReview(r))
	}
	return items
}

// averageRating rounds the average to one decimal place (FR-12) and is nil without reviews.
func averageRating(summary data.RatingSummary) *float64 {
	if summary.ReviewCount == 0 {
		return nil
	}
	average := math.Round(summary.AverageRating*10) / 10
	return &average
}

type createReviewBody struct {
	Rating  int    `json:"rating"`
	Comment string `json:"comment"`
}

func (a *API) createReview(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	var body createReviewBody
	if !decodeBody(w, r, &body) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), reviewTimeout)
	defer cancel()
	created, err := a.app.CreateReview(ctx, current.ID, r.PathValue("exchangeId"), trust.ReviewInput(body))
	if err != nil {
		a.reviewError(w, err)
		return
	}
	respond(w, http.StatusCreated, map[string]any{
		"review":   toReview(created.Review),
		"exchange": toExchange(created.Exchange, current.ID),
	})
}

func (a *API) exchangeReviews(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	ctx, cancel := context.WithTimeout(r.Context(), reviewTimeout)
	defer cancel()
	found, err := a.app.ExchangeReviews(ctx, current.ID, r.PathValue("exchangeId"))
	if err != nil {
		a.reviewError(w, err)
		return
	}
	response := map[string]any{
		"myReview":      nil,
		"partnerReview": nil,
		"canReview":     toExchange(found.Exchange, current.ID).CanReview,
	}
	if found.Mine != nil {
		response["myReview"] = toReview(*found.Mine)
	}
	if found.Partner != nil {
		response["partnerReview"] = toReview(*found.Partner)
	}
	respond(w, http.StatusOK, response)
}

func (a *API) myReviews(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	a.listReviews(w, r, current, current.ID)
}

func (a *API) studentReviews(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	a.listReviews(w, r, current, r.PathValue("userId"))
}

func (a *API) listReviews(w http.ResponseWriter, r *http.Request, current data.User, studentID string) {
	page, ok := pageParam(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), reviewTimeout)
	defer cancel()
	result, err := a.app.UserReviews(ctx, current.ID, studentID, page)
	if err != nil {
		a.reviewError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{
		"items":         toReviews(result.Items),
		"page":          result.Page,
		"pageSize":      trust.PageSize,
		"total":         result.ReviewCount,
		"averageRating": averageRating(result.RatingSummary),
		"reviewCount":   result.ReviewCount,
	})
}

func (a *API) reviewError(w http.ResponseWriter, err error) {
	var invalid *profile.ValidationError
	switch {
	case errors.As(err, &invalid):
		validationProblem(w, invalid.Fields)
	case errors.Is(err, trust.ErrExchangeNotFound):
		problem(w, http.StatusNotFound, "exchange_not_found", "Exchange was not found")
	case errors.Is(err, trust.ErrStudentNotFound):
		problem(w, http.StatusNotFound, "student_not_found", "Student was not found")
	case errors.Is(err, trust.ErrExchangeNotCompleted):
		problem(w, http.StatusConflict, "exchange_not_completed", "Only a completed exchange can be reviewed")
	case errors.Is(err, trust.ErrReviewExists):
		problem(w, http.StatusConflict, "review_already_exists", "You have already reviewed this exchange")
	case errors.Is(err, trust.ErrInvalidPage):
		problem(w, http.StatusBadRequest, "invalid_page", fmt.Sprintf("page must be a whole number from 1 to %d", trust.MaxPage))
	default:
		a.queryError(w, err)
	}
}
