package trust

import (
	"context"
	"errors"
	"strings"
	"testing"

	"skillswap/backend/internal/communication"
	"skillswap/backend/internal/data"
	"skillswap/backend/internal/data/datatest"
	"skillswap/backend/internal/exchange"
	"skillswap/backend/internal/expect"
	"skillswap/backend/internal/profile"
)

type fixture struct {
	service   *Service
	exchanges *exchange.Service
	comm      *communication.Service
	store     *datatest.Memory
	olha      string
	andrii    string
}

func (f *fixture) student(t *testing.T, email, teach, learn string) string {
	t.Helper()
	ctx := context.Background()
	id, err := f.store.CreateUser(ctx, data.NewUser{UniversityID: datatest.DemoUniversityID, Email: email, FirstName: email, LastName: "Test"})
	expect.NoError(t, "create user", err)
	expect.NoError(t, "verify email", f.store.MarkEmailVerified(ctx, id))
	expect.NoError(t, "update profile", f.store.UpdateProfile(ctx, id, data.ProfileUpdate{
		FirstName: email, LastName: "Test", City: "Київ", Formats: []string{data.FormatOnline},
	}))
	expect.NoError(t, "add teaching skill", f.store.AddUserSkill(ctx, data.TeachingList, id, teach, "ADVANCED"))
	expect.NoError(t, "add learning skill", f.store.AddUserSkill(ctx, data.LearningList, id, learn, "BEGINNER"))
	return id
}

// newFixture has Olha (teaches guitar, learns Photoshop) and Andrii (the reverse).
func newFixture(t *testing.T) *fixture {
	t.Helper()
	store := datatest.NewMemory()
	comm := communication.NewService(store, store, store)
	f := &fixture{
		service:   NewService(store, store, store, comm),
		exchanges: exchange.NewService(store, store, store, comm),
		comm:      comm,
		store:     store,
	}
	f.olha = f.student(t, "olha@students.example.test", datatest.GuitarSkillID, datatest.PhotoshopSkillID)
	f.andrii = f.student(t, "andrii@students.example.test", datatest.PhotoshopSkillID, datatest.GuitarSkillID)
	return f
}

// exchangeWithStatus creates an exchange between Olha and Andrii and moves it to the status.
func (f *fixture) exchangeWithStatus(t *testing.T, status string) string {
	t.Helper()
	ctx := context.Background()
	request, err := f.exchanges.CreateRequest(ctx, f.olha, exchange.NewRequest{
		RecipientID: f.andrii, TeachSkillID: datatest.GuitarSkillID, LearnSkillID: datatest.PhotoshopSkillID,
		Format: data.FormatOnline, TeachSessions: 1, TeachDurationMinutes: 60, LearnSessions: 1, LearnDurationMinutes: 60,
	})
	expect.NoError(t, "create request", err)
	accepted, err := f.exchanges.AcceptRequest(ctx, f.andrii, request.ID)
	expect.NoError(t, "accept request", err)
	f.store.SetExchangeStatus(accepted.Exchange.ID, status)
	return accepted.Exchange.ID
}

func fieldErrors(t *testing.T, err error) map[string]string {
	t.Helper()
	var invalid *profile.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
	return invalid.Fields
}

func TestCreateReview(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	exchangeID := f.exchangeWithStatus(t, data.ExchangeCompleted)

	created, err := f.service.CreateReview(ctx, f.olha, strings.ToUpper(exchangeID),
		ReviewInput{Rating: 5, Comment: "  Чудові заняття з Photoshop!\n"})
	expect.NoError(t, "create review", err)
	review := created.Review
	expect.Equal(t, "review ID is set", review.ID != "", true)
	expect.Equal(t, "exchange", review.ExchangeID, exchangeID)
	expect.Equal(t, "author", review.AuthorID, f.olha)
	expect.Equal(t, "author's first name", review.AuthorFirstName, "olha@students.example.test")
	expect.Equal(t, "recipient is the partner", review.RecipientID, f.andrii)
	expect.Equal(t, "rating", review.Rating, 5)
	expect.Equal(t, "comment is trimmed", review.Comment, "Чудові заняття з Photoshop!")
	expect.Slice(t, "review authors of the exchange", created.Exchange.ReviewAuthorIDs, []string{f.olha})

	// Andrii is notified, and the notification opens the exchange.
	page, err := f.comm.Notifications(ctx, f.andrii, true, 1)
	expect.NoError(t, "load Andrii's notifications", err)
	newest := page.Items[0]
	expect.Equal(t, "event type", newest.EventType, data.EventReviewReceived)
	expect.Equal(t, "actor", newest.ActorID, f.olha)
	expect.Equal(t, "notification exchange", newest.ExchangeID, exchangeID)
	expect.Equal(t, "notification review", newest.ReviewID, review.ID)

	// A review without text is allowed, and Andrii reviews Olha independently.
	byAndrii, err := f.service.CreateReview(ctx, f.andrii, exchangeID, ReviewInput{Rating: 4, Comment: "   "})
	expect.NoError(t, "Andrii's review", err)
	expect.Equal(t, "Andrii's review recipient", byAndrii.Review.RecipientID, f.olha)
	expect.Equal(t, "empty comment", byAndrii.Review.Comment, "")
	expect.Slice(t, "review authors after both reviews", byAndrii.Exchange.ReviewAuthorIDs, []string{f.olha, f.andrii})
}

func TestCreateReviewOnlyOnce(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	exchangeID := f.exchangeWithStatus(t, data.ExchangeCompleted)

	_, err := f.service.CreateReview(ctx, f.olha, exchangeID, ReviewInput{Rating: 5})
	expect.NoError(t, "first review", err)
	_, err = f.service.CreateReview(ctx, f.olha, exchangeID, ReviewInput{Rating: 1, Comment: "Передумала"})
	expect.Error(t, "second review", err, ErrReviewExists)

	page, err := f.service.UserReviews(ctx, f.andrii, f.andrii, 1)
	expect.NoError(t, "load Andrii's reviews", err)
	expect.Equal(t, "Andrii's review count", page.ReviewCount, 1)
	expect.Equal(t, "the first rating is kept", page.Items[0].Rating, 5)
}

func TestCreateReviewRejectsUnfinishedAndForeignExchanges(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	marko := f.student(t, "marko@students.example.test", datatest.PhotoshopSkillID, datatest.GuitarSkillID)
	active := f.exchangeWithStatus(t, data.ExchangeActive)
	cancelled := f.exchangeWithStatus(t, data.ExchangeCancelled)
	completed := f.exchangeWithStatus(t, data.ExchangeCompleted)

	for _, test := range []struct {
		name       string
		userID     string
		exchangeID string
		want       error
	}{
		{"active exchange", f.olha, active, ErrExchangeNotCompleted},
		{"cancelled exchange", f.andrii, cancelled, ErrExchangeNotCompleted},
		{"another pair's exchange", marko, completed, ErrExchangeNotFound},
		{"unknown exchange", f.olha, "80000000-0000-0000-0000-000000000999", ErrExchangeNotFound},
		{"malformed ID", f.olha, "not-a-uuid", ErrExchangeNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := f.service.CreateReview(ctx, test.userID, test.exchangeID, ReviewInput{Rating: 3})
			expect.Error(t, "create review", err, test.want)
		})
	}
}

func TestCreateReviewValidation(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	exchangeID := f.exchangeWithStatus(t, data.ExchangeCompleted)

	for _, test := range []struct {
		name  string
		in    ReviewInput
		field string
	}{
		{"rating below 1", ReviewInput{Rating: 0}, "rating"},
		{"rating above 5", ReviewInput{Rating: 6}, "rating"},
		{"comment over 1000 characters", ReviewInput{Rating: 5, Comment: strings.Repeat("ї", MaxCommentLength+1)}, "comment"},
		{"control character in the comment", ReviewInput{Rating: 5, Comment: "Добре\x00"}, "comment"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := f.service.CreateReview(ctx, f.olha, exchangeID, test.in)
			expect.Equal(t, "field has an error", fieldErrors(t, err)[test.field] != "", true)
		})
	}

	// Exactly 1000 characters are allowed.
	_, err := f.service.CreateReview(ctx, f.olha, exchangeID, ReviewInput{Rating: 1, Comment: strings.Repeat("ї", MaxCommentLength)})
	expect.NoError(t, "comment of 1000 characters", err)
}

func TestExchangeReviews(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	marko := f.student(t, "marko@students.example.test", datatest.PhotoshopSkillID, datatest.GuitarSkillID)
	exchangeID := f.exchangeWithStatus(t, data.ExchangeCompleted)

	before, err := f.service.ExchangeReviews(ctx, f.olha, exchangeID)
	expect.NoError(t, "reviews before anyone wrote one", err)
	expect.Equal(t, "Olha's review", before.Mine == nil, true)
	expect.Equal(t, "Andrii's review", before.Partner == nil, true)

	_, err = f.service.CreateReview(ctx, f.andrii, exchangeID, ReviewInput{Rating: 4, Comment: "Дякую!"})
	expect.NoError(t, "Andrii's review", err)

	asOlha, err := f.service.ExchangeReviews(ctx, f.olha, exchangeID)
	expect.NoError(t, "reviews as Olha sees them", err)
	expect.Equal(t, "Olha's review", asOlha.Mine == nil, true)
	expect.Equal(t, "Andrii's review rating", asOlha.Partner.Rating, 4)
	asAndrii, err := f.service.ExchangeReviews(ctx, f.andrii, exchangeID)
	expect.NoError(t, "reviews as Andrii sees them", err)
	expect.Equal(t, "Andrii's own review", asAndrii.Mine.Comment, "Дякую!")
	expect.Equal(t, "Olha's review as Andrii sees it", asAndrii.Partner == nil, true)

	_, err = f.service.ExchangeReviews(ctx, marko, exchangeID)
	expect.Error(t, "reviews of another pair's exchange", err, ErrExchangeNotFound)
}

func TestUserReviews(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	marko := f.student(t, "marko@students.example.test", datatest.GuitarSkillID, datatest.PhotoshopSkillID)

	empty, err := f.service.UserReviews(ctx, f.olha, f.andrii, 1)
	expect.NoError(t, "reviews of a student without reviews", err)
	expect.Equal(t, "review count", empty.ReviewCount, 0)
	expect.Equal(t, "reviews", len(empty.Items), 0)

	// Andrii gets 5 and 4 from two exchanges: the average is 4.5.
	for _, rating := range []int{5, 4} {
		exchangeID := f.exchangeWithStatus(t, data.ExchangeCompleted)
		_, err := f.service.CreateReview(ctx, f.olha, exchangeID, ReviewInput{Rating: rating})
		expect.NoError(t, "create review", err)
	}
	page, err := f.service.UserReviews(ctx, marko, strings.ToUpper(f.andrii), 1)
	expect.NoError(t, "Andrii's reviews as Marko sees them", err)
	expect.Equal(t, "review count", page.ReviewCount, 2)
	expect.Equal(t, "average rating", page.AverageRating, 4.5)
	expect.Equal(t, "newest review first", page.Items[0].Rating, 4)
	expect.Equal(t, "page", page.Page, 1)

	own, err := f.service.UserReviews(ctx, f.andrii, f.andrii, 1)
	expect.NoError(t, "Andrii's own reviews", err)
	expect.Equal(t, "own review count", own.ReviewCount, 2)

	// A blocked student is not visible, but still sees their own reviews.
	f.store.SetBlocked(f.andrii, marko)
	_, err = f.service.UserReviews(ctx, marko, f.andrii, 1)
	expect.Error(t, "reviews of a student who blocked the viewer", err, ErrStudentNotFound)
	_, err = f.service.UserReviews(ctx, f.olha, "not-a-uuid", 1)
	expect.Error(t, "malformed student ID", err, ErrStudentNotFound)
	_, err = f.service.UserReviews(ctx, f.olha, f.andrii, 0)
	expect.Error(t, "page 0", err, ErrInvalidPage)
	_, err = f.service.UserReviews(ctx, f.olha, f.andrii, MaxPage+1)
	expect.Error(t, "page past the limit", err, ErrInvalidPage)
}
