package data

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrReviewExists = errors.New("the author has already reviewed this exchange")

type Review struct {
	ID              string
	ExchangeID      string
	AuthorID        string
	AuthorFirstName string
	AuthorLastName  string
	RecipientID     string
	Rating          int
	Comment         string // empty when there is no text
	CreatedAt       time.Time
}

type NewReview struct {
	ExchangeID  string
	AuthorID    string
	RecipientID string
	Rating      int
	Comment     string
}

// RatingSummary is the average of all ratings a user received; AverageRating is 0 without reviews.
type RatingSummary struct {
	AverageRating float64
	ReviewCount   int
}

type IReviewData interface {
	StudentVisible(ctx context.Context, viewerID, studentID string) (bool, error)
	CreateReview(ctx context.Context, review NewReview) (string, error)
	ReviewByID(ctx context.Context, reviewID string) (Review, error)
	ExchangeReviews(ctx context.Context, exchangeID string) ([]Review, error)
	UserReviews(ctx context.Context, userID string, limit, offset int) ([]Review, RatingSummary, error)
}

var _ IReviewData = (*Postgres)(nil)

// StudentVisible reports whether the viewer may open the student's profile.
func (p *Postgres) StudentVisible(ctx context.Context, viewerID, studentID string) (bool, error) {
	var visible bool
	err := p.conn(ctx).QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users c WHERE c.id = $2 AND `+visibleStudent+`)`,
		viewerID, studentID).Scan(&visible)
	return visible, err
}

// CreateReview returns ErrReviewExists when the author has already reviewed the exchange.
func (p *Postgres) CreateReview(ctx context.Context, r NewReview) (string, error) {
	var id string
	err := p.conn(ctx).QueryRow(ctx, `INSERT INTO exchange_reviews (exchange_id, author_id, recipient_id, rating, comment)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''))
		RETURNING id`, r.ExchangeID, r.AuthorID, r.RecipientID, r.Rating, r.Comment).Scan(&id)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "idx_exchange_reviews_exchange_id_author_id" {
		return "", ErrReviewExists
	}
	return id, err
}

const reviewSelect = `SELECT r.id, r.exchange_id, r.author_id, a.first_name, a.last_name, r.recipient_id,
		r.rating, COALESCE(r.comment, ''), r.created_at
	FROM exchange_reviews r
	JOIN users a ON a.id = r.author_id`

func scanReview(row pgx.Row) (Review, error) {
	var r Review
	err := row.Scan(&r.ID, &r.ExchangeID, &r.AuthorID, &r.AuthorFirstName, &r.AuthorLastName, &r.RecipientID,
		&r.Rating, &r.Comment, &r.CreatedAt)
	return r, err
}

func (p *Postgres) ReviewByID(ctx context.Context, reviewID string) (Review, error) {
	r, err := scanReview(p.conn(ctx).QueryRow(ctx, reviewSelect+` WHERE r.id = $1`, reviewID))
	if err != nil {
		return Review{}, notFound(err)
	}
	return r, nil
}

// ExchangeReviews returns at most one review from each participant, oldest first.
func (p *Postgres) ExchangeReviews(ctx context.Context, exchangeID string) ([]Review, error) {
	rows, err := p.conn(ctx).Query(ctx, reviewSelect+` WHERE r.exchange_id = $1 ORDER BY r.created_at, r.id`, exchangeID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Review, error) { return scanReview(row) })
}

// UserReviews returns one page of the reviews the user received, newest first, and the rating
// over all of them.
func (p *Postgres) UserReviews(ctx context.Context, userID string, limit, offset int) ([]Review, RatingSummary, error) {
	var summary RatingSummary
	if err := p.conn(ctx).QueryRow(ctx, `SELECT COALESCE(avg(rating), 0)::float8, count(*)
		FROM exchange_reviews WHERE recipient_id = $1`, userID).Scan(&summary.AverageRating, &summary.ReviewCount); err != nil {
		return nil, RatingSummary{}, err
	}
	rows, err := p.conn(ctx).Query(ctx, reviewSelect+` WHERE r.recipient_id = $1
		ORDER BY r.created_at DESC, r.id DESC
		LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, RatingSummary{}, err
	}
	reviews, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Review, error) { return scanReview(row) })
	if err != nil {
		return nil, RatingSummary{}, err
	}
	return reviews, summary, nil
}
