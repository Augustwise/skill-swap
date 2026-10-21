// Package trust holds reviews and ratings (FR-12).
package trust

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
	MinRating        = 1
	MaxRating        = 5
	MaxCommentLength = 1000

	// PageSize is the number of reviews on one page.
	PageSize = 20
	MaxPage  = 500
)

var (
	ErrInvalidPage          = errors.New("page is out of range")
	ErrStudentNotFound      = errors.New("student was not found or is not visible")
	ErrExchangeNotFound     = errors.New("exchange was not found or the user does not take part in it")
	ErrExchangeNotCompleted = errors.New("only a completed exchange can be reviewed")
	ErrReviewExists         = errors.New("the user has already reviewed this exchange")
)

type ICommunication interface {
	RecordReviewReceived(ctx context.Context, review data.Review, exchange data.Exchange) error
}

type Service struct {
	exchanges data.IExchangeData
	reviews   data.IReviewData
	tx        data.ITransaction
	comm      ICommunication
}

func NewService(exchanges data.IExchangeData, reviews data.IReviewData, tx data.ITransaction, comm ICommunication) *Service {
	return &Service{exchanges: exchanges, reviews: reviews, tx: tx, comm: comm}
}

type ReviewInput struct {
	Rating  int
	Comment string
}

type ReviewResult struct {
	Review   data.Review
	Exchange data.Exchange
}

// CreateReview lets a participant of a completed exchange rate the partner once.
func (s *Service) CreateReview(ctx context.Context, userID, exchangeID string, in ReviewInput) (ReviewResult, error) {
	if !validate.UUID(exchangeID) {
		return ReviewResult{}, ErrExchangeNotFound
	}
	fields := map[string]string{}
	if in.Rating < MinRating || in.Rating > MaxRating {
		fields["rating"] = "Rating must be a whole number from 1 to 5"
	}
	comment, ok := validate.Text(in.Comment, MaxCommentLength)
	if !ok {
		fields["comment"] = "Comment must be at most 1000 characters"
	}
	if len(fields) > 0 {
		return ReviewResult{}, &profile.ValidationError{Fields: fields}
	}

	var result ReviewResult
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		exchange, err := s.participantExchange(ctx, userID, exchangeID)
		switch {
		case err != nil:
			return err
		case exchange.Status != data.ExchangeCompleted:
			return ErrExchangeNotCompleted
		case slices.Contains(exchange.ReviewAuthorIDs, userID):
			return ErrReviewExists
		}
		reviewID, err := s.reviews.CreateReview(ctx, data.NewReview{
			ExchangeID:  exchange.ID,
			AuthorID:    userID,
			RecipientID: partnerID(exchange, userID),
			Rating:      in.Rating,
			Comment:     comment,
		})
		switch {
		case errors.Is(err, data.ErrReviewExists):
			// The same author reviewed the exchange in a parallel request.
			return ErrReviewExists
		case err != nil:
			return err
		}
		if result.Review, err = s.reviews.ReviewByID(ctx, reviewID); err != nil {
			return err
		}
		if s.comm != nil {
			if err := s.comm.RecordReviewReceived(ctx, result.Review, exchange); err != nil {
				return err
			}
		}
		result.Exchange, err = s.exchanges.ExchangeByID(ctx, exchange.ID)
		return err
	})
	return result, err
}

type ExchangeReviews struct {
	Exchange data.Exchange
	// Mine is the user's review of the partner and Partner the partner's review of the user;
	// each is nil until it is written.
	Mine    *data.Review
	Partner *data.Review
}

func (s *Service) ExchangeReviews(ctx context.Context, userID, exchangeID string) (ExchangeReviews, error) {
	if !validate.UUID(exchangeID) {
		return ExchangeReviews{}, ErrExchangeNotFound
	}
	exchange, err := s.participantExchange(ctx, userID, exchangeID)
	if err != nil {
		return ExchangeReviews{}, err
	}
	reviews, err := s.reviews.ExchangeReviews(ctx, exchange.ID)
	if err != nil {
		return ExchangeReviews{}, err
	}
	result := ExchangeReviews{Exchange: exchange}
	for _, r := range reviews {
		if r.AuthorID == userID {
			result.Mine = &r
		} else {
			result.Partner = &r
		}
	}
	return result, nil
}

type ReviewPage struct {
	Items []data.Review
	data.RatingSummary
	Page int
}

// UserReviews lists the reviews a student received. Users always see their own reviews;
// another student's reviews follow the same visibility rules as the profile.
func (s *Service) UserReviews(ctx context.Context, viewerID, studentID string, page int) (ReviewPage, error) {
	if page < 1 || page > MaxPage {
		return ReviewPage{}, ErrInvalidPage
	}
	studentID = strings.ToLower(studentID)
	if studentID != viewerID {
		if !validate.UUID(studentID) {
			return ReviewPage{}, ErrStudentNotFound
		}
		visible, err := s.reviews.StudentVisible(ctx, viewerID, studentID)
		if err != nil {
			return ReviewPage{}, err
		}
		if !visible {
			return ReviewPage{}, ErrStudentNotFound
		}
	}
	items, summary, err := s.reviews.UserReviews(ctx, studentID, PageSize, (page-1)*PageSize)
	if err != nil {
		return ReviewPage{}, err
	}
	return ReviewPage{Items: items, RatingSummary: summary, Page: page}, nil
}

func (s *Service) participantExchange(ctx context.Context, userID, exchangeID string) (data.Exchange, error) {
	exchange, err := s.exchanges.ExchangeByID(ctx, strings.ToLower(exchangeID))
	switch {
	case errors.Is(err, data.ErrNotFound):
		return data.Exchange{}, ErrExchangeNotFound
	case err != nil:
		return data.Exchange{}, err
	case userID != exchange.Requester.UserID && userID != exchange.Recipient.UserID:
		return data.Exchange{}, ErrExchangeNotFound
	}
	return exchange, nil
}

func partnerID(e data.Exchange, userID string) string {
	if userID == e.Requester.UserID {
		return e.Recipient.UserID
	}
	return e.Requester.UserID
}
