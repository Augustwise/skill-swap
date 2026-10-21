package api

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"skillswap/backend/internal/data"
	"skillswap/backend/internal/data/datatest"
	"skillswap/backend/internal/expect"
)

type createdReviewResponse struct {
	Review   reviewResponse   `json:"review"`
	Exchange exchangeResponse `json:"exchange"`
}

type exchangeReviewsResponse struct {
	MyReview      *reviewResponse `json:"myReview"`
	PartnerReview *reviewResponse `json:"partnerReview"`
	CanReview     bool            `json:"canReview"`
}

type reviewPageResponse struct {
	Items         []reviewResponse `json:"items"`
	Page          int              `json:"page"`
	PageSize      int              `json:"pageSize"`
	Total         int              `json:"total"`
	AverageRating *float64         `json:"averageRating"`
	ReviewCount   int              `json:"reviewCount"`
}

// completeExchange marks every lesson as held and lets both participants confirm the end.
func completeExchange(t *testing.T, e *discoveryEnv, exchangeID string, olha, andrii *session) {
	t.Helper()
	for _, s := range getExchangeDetails(t, e, exchangeID, olha).Sessions {
		e.store.SetSessionStatus(s.ID, data.SessionCompleted)
	}
	completePath := "/api/v1/exchanges/" + exchangeID + "/complete"
	expectStatus(t, send(t, e.handler, http.MethodPost, completePath, "", olha), http.StatusOK)
	expectStatus(t, send(t, e.handler, http.MethodPost, completePath, "", andrii), http.StatusOK)
}

func TestReviewRoutes(t *testing.T) {
	e, olha, andrii, olhaID, andriiID, exchangeID := newExchangeEnv(t)
	marko := e.student(t, "marko@students.example.test", `["ONLINE"]`, "Київ", datatest.PhotoshopSkillID, datatest.GuitarSkillID)
	reviewsPath := "/api/v1/exchanges/" + exchangeID + "/reviews"

	// An active exchange cannot be reviewed yet.
	active := getExchangeDetails(t, e, exchangeID, olha).Exchange
	expect.Equal(t, "an active exchange can be reviewed", active.CanReview, false)
	assertProblem(t, send(t, e.handler, http.MethodPost, reviewsPath, `{"rating":5}`, olha), http.StatusConflict, "exchange_not_completed")

	completeExchange(t, e, exchangeID, olha, andrii)
	completed := getExchangeDetails(t, e, exchangeID, olha).Exchange
	expect.Equal(t, "a completed exchange can be reviewed", completed.CanReview, true)
	expect.Equal(t, "reviewed by Olha", completed.ReviewedByMe, false)

	assertProblem(t, send(t, e.handler, http.MethodPost, reviewsPath, `{"rating":5}`, marko), http.StatusNotFound, "exchange_not_found")
	assertProblem(t, send(t, e.handler, http.MethodPost, reviewsPath, `{"rating":0}`, olha), http.StatusUnprocessableEntity, "validation_failed")
	longComment := fmt.Sprintf(`{"rating":5,"comment":%q}`, strings.Repeat("a", 1001))
	assertProblem(t, send(t, e.handler, http.MethodPost, reviewsPath, longComment, olha), http.StatusUnprocessableEntity, "validation_failed")
	assertProblem(t, send(t, e.handler, http.MethodPost, reviewsPath, `{"rating":4.5}`, olha), http.StatusBadRequest, "invalid_json")
	assertProblem(t, send(t, e.handler, http.MethodPost, reviewsPath, `{"rating":5}`, nil), http.StatusUnauthorized, "unauthenticated")

	// Olha reviews Andrii.
	created := sendAndDecode[createdReviewResponse](t, e, http.MethodPost, reviewsPath,
		`{"rating":5,"comment":"Дуже зрозуміло пояснює Photoshop"}`, olha, http.StatusCreated)
	expect.Equal(t, "review author", created.Review.Author.ID, olhaID)
	expect.Equal(t, "review exchange", created.Review.ExchangeID, exchangeID)
	expect.Equal(t, "review rating", created.Review.Rating, 5)
	expect.Equal(t, "review comment", created.Review.Comment, "Дуже зрозуміло пояснює Photoshop")
	expect.Equal(t, "reviewed by Olha", created.Exchange.ReviewedByMe, true)
	expect.Equal(t, "Olha can review again", created.Exchange.CanReview, false)
	assertProblem(t, send(t, e.handler, http.MethodPost, reviewsPath, `{"rating":1}`, olha), http.StatusConflict, "review_already_exists")

	// Andrii sees Olha's review and can still write his own.
	asAndrii := sendAndDecode[exchangeReviewsResponse](t, e, http.MethodGet, reviewsPath, "", andrii, http.StatusOK)
	expect.Equal(t, "Andrii's review", asAndrii.MyReview == nil, true)
	expect.Equal(t, "Olha's review as Andrii sees it", valueOf(asAndrii.PartnerReview).ID, created.Review.ID)
	expect.Equal(t, "Andrii can review", asAndrii.CanReview, true)
	expect.Equal(t, "reviewed by Olha, as Andrii sees it",
		getExchangeDetails(t, e, exchangeID, andrii).Exchange.ReviewedByPartner, true)
	assertProblem(t, send(t, e.handler, http.MethodGet, reviewsPath, "", marko), http.StatusNotFound, "exchange_not_found")

	sendAndDecode[createdReviewResponse](t, e, http.MethodPost, reviewsPath, `{"rating":4}`, andrii, http.StatusCreated)

	// Andrii's profile shows the rating and the review.
	profile := sendAndDecode[struct {
		Student studentProfileResponse `json:"student"`
	}](t, e, http.MethodGet, "/api/v1/students/"+andriiID, "", olha, http.StatusOK).Student
	expect.Equal(t, "Andrii's average rating", valueOf(profile.AverageRating), 5.0)
	expect.Equal(t, "Andrii's review count", profile.ReviewCount, 1)
	expect.Equal(t, "Andrii's newest review", profile.Reviews[0].ID, created.Review.ID)

	reviews := sendAndDecode[reviewPageResponse](t, e, http.MethodGet, "/api/v1/students/"+andriiID+"/reviews", "", marko, http.StatusOK)
	expect.Equal(t, "total", reviews.Total, 1)
	expect.Equal(t, "page size", reviews.PageSize, 20)
	expect.Equal(t, "average rating", valueOf(reviews.AverageRating), 5.0)
	expect.Equal(t, "review author", reviews.Items[0].Author.ID, olhaID)

	mine := sendAndDecode[reviewPageResponse](t, e, http.MethodGet, "/api/v1/me/reviews", "", olha, http.StatusOK)
	expect.Equal(t, "Olha's review count", mine.ReviewCount, 1)
	expect.Equal(t, "Olha's average rating", valueOf(mine.AverageRating), 4.0)
	empty := sendAndDecode[reviewPageResponse](t, e, http.MethodGet, "/api/v1/me/reviews", "", marko, http.StatusOK)
	expect.Equal(t, "Marko's average rating", empty.AverageRating == nil, true)
	expect.Equal(t, "Marko's reviews", len(empty.Items), 0)

	assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/students/not-a-uuid/reviews", "", olha), http.StatusNotFound, "student_not_found")
	assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/me/reviews?page=0", "", olha), http.StatusBadRequest, "invalid_page")
	assertProblem(t, send(t, e.handler, http.MethodDelete, reviewsPath, "", olha), http.StatusMethodNotAllowed, "method_not_allowed")

	// Andrii's notification about the review opens the exchange.
	notifications := sendAndDecode[notificationPageResponse](t, e, http.MethodGet, "/api/v1/me/notifications", "", andrii, http.StatusOK)
	newest := notifications.Items[0]
	expect.Equal(t, "event type", newest.EventType, data.EventReviewReceived)
	expect.Equal(t, "target kind", newest.Target.Kind, "EXCHANGE")
	expect.Equal(t, "target exchange", newest.Target.ID, exchangeID)
	expect.Equal(t, "review ID", valueOf(newest.ReviewID), created.Review.ID)
}

func TestAverageRatingHasOneDecimal(t *testing.T) {
	for _, test := range []struct {
		average float64
		count   int
		want    float64
	}{
		{4.666666, 3, 4.7},
		{4.25, 4, 4.3},
		{3, 1, 3},
	} {
		got := averageRating(data.RatingSummary{AverageRating: test.average, ReviewCount: test.count})
		expect.Equal(t, fmt.Sprintf("rounded %v", test.average), valueOf(got), test.want)
	}
	expect.Equal(t, "average without reviews", averageRating(data.RatingSummary{}) == nil, true)
}
