package data

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"skillswap/backend/internal/expect"
)

// withCompletedExchange runs fn in a transaction that is rolled back, with a completed exchange
// between Olha and Andrii.
func withCompletedExchange(t *testing.T, fn func(ctx context.Context, store *Postgres, exchangeID string)) {
	t.Helper()
	store := openTestStore(t)
	rollback := errors.New("roll back test data")
	err := store.WithinTx(context.Background(), func(ctx context.Context) error {
		var exchangeID string
		if err := store.conn(ctx).QueryRow(ctx, `INSERT INTO exchanges (user_a_id, user_b_id, status, total_sessions, format, completed_at)
			VALUES ($1, $2, 'COMPLETED', 2, 'ONLINE', now()) RETURNING id`, demoOlha, demoAndrii).Scan(&exchangeID); err != nil {
			t.Fatal(err)
		}
		fn(ctx, store, exchangeID)
		return rollback
	})
	expect.Error(t, "WithinTx", err, rollback)
}

// Skipped unless TEST_DATABASE_URL points to a migrated, demo-seeded database.
func TestReviewsAgainstPostgres(t *testing.T) {
	withCompletedExchange(t, func(ctx context.Context, store *Postgres, exchangeID string) {
		before, summaryBefore, err := store.UserReviews(ctx, demoAndrii, 20, 0)
		expect.NoError(t, "Andrii's reviews before", err)

		id, err := store.CreateReview(ctx, NewReview{ExchangeID: exchangeID, AuthorID: demoOlha, RecipientID: demoAndrii,
			Rating: 4, Comment: "Дуже корисно"})
		expect.NoError(t, "create Olha's review", err)
		_, err = store.CreateReview(ctx, NewReview{ExchangeID: exchangeID, AuthorID: demoAndrii, RecipientID: demoOlha, Rating: 5})
		expect.NoError(t, "create Andrii's review without text", err)

		review, err := store.ReviewByID(ctx, id)
		expect.NoError(t, "load Olha's review", err)
		expect.Equal(t, "exchange", review.ExchangeID, exchangeID)
		expect.Equal(t, "author's first name", review.AuthorFirstName, "Ольга")
		expect.Equal(t, "recipient", review.RecipientID, demoAndrii)
		expect.Equal(t, "rating", review.Rating, 4)
		expect.Equal(t, "comment", review.Comment, "Дуже корисно")
		_, err = store.ReviewByID(ctx, "90000000-0000-0000-0000-000000000999")
		expect.Error(t, "load an unknown review", err, ErrNotFound)

		reviews, err := store.ExchangeReviews(ctx, exchangeID)
		expect.NoError(t, "load the exchange reviews", err)
		expect.Equal(t, "exchange reviews", len(reviews), 2)
		// Both reviews have the same created_at inside one transaction, so the order is not checked.
		andriisReview := slices.IndexFunc(reviews, func(r Review) bool { return r.AuthorID == demoAndrii })
		expect.Equal(t, "Andrii's review has no text", reviews[andriisReview].Comment, "")

		var authors []string
		err = store.conn(ctx).QueryRow(ctx, `SELECT `+exchangeReviewAuthors+` FROM exchanges e WHERE e.id = $1`, exchangeID).Scan(&authors)
		expect.NoError(t, "load the review authors", err)
		slices.Sort(authors)
		expect.Slice(t, "review authors", authors, []string{demoOlha, demoAndrii})

		after, summary, err := store.UserReviews(ctx, demoAndrii, 20, 0)
		expect.NoError(t, "Andrii's reviews after", err)
		expect.Equal(t, "review count", summary.ReviewCount, summaryBefore.ReviewCount+1)
		expect.Equal(t, "reviews on the page", len(after), min(len(before)+1, 20))
		expect.Equal(t, "newest review", after[0].ID, id)

		visible, err := store.StudentVisible(ctx, demoOlha, demoAndrii)
		expect.NoError(t, "check visibility", err)
		expect.Equal(t, "Andrii is visible to Olha", visible, true)
		visible, err = store.StudentVisible(ctx, demoOlha, demoOleh)
		expect.NoError(t, "check visibility of a blocked student", err)
		expect.Equal(t, "Oleh is visible to Olha", visible, false)

		// A second review of the same exchange by the same author is rejected by the unique index.
		_, err = store.CreateReview(ctx, NewReview{ExchangeID: exchangeID, AuthorID: demoOlha, RecipientID: demoAndrii, Rating: 1})
		expect.Error(t, "create a second review", err, ErrReviewExists)
	})
}

// Skipped unless TEST_DATABASE_URL is set.
func TestReviewConstraintsAgainstPostgres(t *testing.T) {
	for _, test := range []struct {
		name       string
		review     func(exchangeID string) NewReview
		constraint string
	}{
		{"rating above 5", func(id string) NewReview {
			return NewReview{ExchangeID: id, AuthorID: demoOlha, RecipientID: demoAndrii, Rating: 6}
		}, "ck_exchange_reviews_1"},
		{"review of oneself", func(id string) NewReview {
			return NewReview{ExchangeID: id, AuthorID: demoOlha, RecipientID: demoOlha, Rating: 5}
		}, "ck_exchange_reviews_2"},
		{"blank comment", func(id string) NewReview {
			return NewReview{ExchangeID: id, AuthorID: demoOlha, RecipientID: demoAndrii, Rating: 5, Comment: "   "}
		}, "ck_exchange_reviews_comment"},
		{"comment over 1000 characters", func(id string) NewReview {
			return NewReview{ExchangeID: id, AuthorID: demoOlha, RecipientID: demoAndrii, Rating: 5, Comment: strings.Repeat("ї", 1001)}
		}, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			withCompletedExchange(t, func(ctx context.Context, store *Postgres, exchangeID string) {
				_, err := store.CreateReview(ctx, test.review(exchangeID))
				var pgErr *pgconn.PgError
				expect.Equal(t, "database error", errors.As(err, &pgErr), true)
				if test.constraint != "" {
					expect.Equal(t, "constraint", pgErr.ConstraintName, test.constraint)
				}
				expect.Equal(t, "check or length violation", slices.Contains([]string{"23514", "22001"}, pgErr.Code), true)
			})
		})
	}
}
