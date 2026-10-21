package trust

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"skillswap/backend/internal/communication"
	"skillswap/backend/internal/data"
	"skillswap/backend/internal/profile"
	"skillswap/backend/internal/validate"
)

// Reasons stored with exchanges that end because the pair can no longer interact.
const (
	blockReason   = "Один з учасників заблокував іншого"
	suspendReason = "Обліковий запис учасника заблоковано адміністратором"
	deleteReason  = "Учасник видалив обліковий запис"
)

type BlockResult struct {
	Block              data.Block
	CancelledExchanges int
	ClosedRequests     int
}

// Block stops new requests and messages between the user and the student (FR-14). Their
// active exchanges are cancelled with the history kept, and pending requests are closed.
// Blocking again changes nothing.
func (s *Service) Block(ctx context.Context, userID, studentID string) (BlockResult, error) {
	studentID, err := s.target(userID, studentID, "You cannot block yourself")
	if err != nil {
		return BlockResult{}, err
	}
	var result BlockResult
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.moderation.LockUsers(ctx, userID, studentID); err != nil {
			return err
		}
		known, err := s.moderation.KnownStudent(ctx, userID, studentID)
		switch {
		case err != nil:
			return err
		case !known:
			return ErrStudentNotFound
		}
		created, err := s.moderation.CreateBlock(ctx, userID, studentID)
		if err != nil {
			return err
		}
		if created {
			end, err := s.endInteractions(ctx, ending{userID: userID, partnerID: studentID, actorID: userID, reason: blockReason})
			if err != nil {
				return err
			}
			result.CancelledExchanges, result.ClosedRequests = end.exchanges, end.requests
		}
		result.Block, err = s.moderation.BlockByUsers(ctx, userID, studentID)
		return err
	})
	return result, err
}

// Unblock allows new requests and messages again; cancelled exchanges stay cancelled.
func (s *Service) Unblock(ctx context.Context, userID, studentID string) error {
	studentID, err := s.target(userID, studentID, "You cannot unblock yourself")
	if err != nil {
		return err
	}
	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.moderation.LockUsers(ctx, userID, studentID); err != nil {
			return err
		}
		return s.moderation.DeleteBlock(ctx, userID, studentID)
	})
}

type BlockPage struct {
	Items []data.Block
	Page  int
	Total int
}

func (s *Service) Blocks(ctx context.Context, userID string, page int) (BlockPage, error) {
	if page < 1 || page > MaxPage {
		return BlockPage{}, ErrInvalidPage
	}
	items, total, err := s.moderation.UserBlocks(ctx, userID, PageSize, (page-1)*PageSize)
	if err != nil {
		return BlockPage{}, err
	}
	return BlockPage{Items: items, Page: page, Total: total}, nil
}

// target checks the ID of another student the user acts on.
func (s *Service) target(userID, studentID, selfMessage string) (string, error) {
	studentID = strings.ToLower(studentID)
	switch {
	case !validate.UUID(studentID):
		return "", ErrStudentNotFound
	case studentID == userID:
		return "", &profile.ValidationError{Fields: map[string]string{"userId": selfMessage}}
	}
	return studentID, nil
}

// ending describes whose interactions end: all of the user's, or only those with partnerID.
// An empty actorID records the changes as made by the system.
type ending struct {
	userID    string
	partnerID string
	actorID   string
	reason    string
}

type ended struct {
	exchanges int
	requests  int
}

// endInteractions cancels the active exchanges and closes the pending requests: the user's own
// requests are withdrawn and the incoming ones declined. It must run inside a transaction that
// holds the user's lock, so no request can be accepted in the meantime.
func (s *Service) endInteractions(ctx context.Context, e ending) (ended, error) {
	exchangeIDs, err := s.moderation.ActiveExchangeIDs(ctx, e.userID, e.partnerID)
	if err != nil {
		return ended{}, err
	}
	for _, id := range exchangeIDs {
		if err := s.exchanges.CancelExchange(ctx, id, e.actorID, e.reason); err != nil {
			return ended{}, err
		}
		cancelled, err := s.exchanges.ExchangeByID(ctx, id)
		if err != nil {
			return ended{}, err
		}
		if err := s.notifyCancelled(ctx, cancelled, e); err != nil {
			return ended{}, err
		}
	}
	requests, err := s.moderation.PendingRequests(ctx, e.userID, e.partnerID)
	if err != nil {
		return ended{}, err
	}
	for _, r := range requests {
		status := data.RequestDeclined
		if r.RequesterID == e.userID {
			status = data.RequestWithdrawn
		}
		if err := s.exchanges.UpdateRequestStatus(ctx, r.ID, status, e.actorID); err != nil {
			return ended{}, err
		}
	}
	return ended{exchanges: len(exchangeIDs), requests: len(requests)}, nil
}

func (s *Service) notifyCancelled(ctx context.Context, exchange data.Exchange, e ending) error {
	if s.comm == nil {
		return nil
	}
	return s.comm.RecordSessionUpdated(ctx, communication.SessionUpdateEvent{
		RecipientID: partnerID(exchange, e.userID),
		ActorID:     e.actorID,
		EventKey:    "exchange-cancelled:" + exchange.ID,
		Title:       "Обмін скасовано",
		Body: fmt.Sprintf("Обмін «%s ↔ %s» скасовано. Причина: %s.",
			exchange.RequesterTeaches.Name, exchange.RecipientTeaches.Name, e.reason),
		ExchangeID: exchange.ID,
	})
}

// DeleteAccount anonymizes the account in the local MVP and ends its exchanges and requests
// (FR-15). The caller confirms the password first.
func (s *Service) DeleteAccount(ctx context.Context, userID string) error {
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.moderation.LockUsers(ctx, userID, userID); err != nil {
			return err
		}
		// Anonymize first: the notifications created next must keep their text.
		if err := s.moderation.AnonymizeUser(ctx, userID); err != nil {
			return err
		}
		_, err := s.endInteractions(ctx, ending{userID: userID, actorID: userID, reason: deleteReason})
		return err
	})
	if errors.Is(err, data.ErrNotFound) {
		return ErrUserNotFound
	}
	return err
}
