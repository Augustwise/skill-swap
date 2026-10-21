package communication

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"skillswap/backend/internal/data"
	"skillswap/backend/internal/profile"
	"skillswap/backend/internal/validate"
)

const (
	// PageSize is the number of conversations, messages or notifications on one page.
	PageSize = 20
	MaxPage  = 500

	// MaxMessageLength is the FR-08 character limit for one chat message.
	MaxMessageLength = 2000

	// PollIntervalSeconds is the FR-08 / LR2 3.3 polling interval for an open chat.
	PollIntervalSeconds = 5

	previewLength = 120
)

var (
	ErrInvalidPage          = errors.New("page is out of range")
	ErrConversationNotFound = errors.New("conversation was not found or the user does not take part in it")
	ErrConversationBlocked  = errors.New("new messages cannot be sent in this conversation")
	ErrRequestNotFound      = errors.New("request was not found or the user does not take part in it")
	ErrExchangeNotFound     = errors.New("exchange was not found or the user does not take part in it")
	ErrNotificationNotFound = errors.New("notification was not found or does not belong to the user")
)

type Service struct {
	exchanges data.IExchangeData
	comm      data.ICommunicationData
	tx        data.ITransaction
}

func NewService(exchanges data.IExchangeData, comm data.ICommunicationData, tx data.ITransaction) *Service {
	return &Service{exchanges: exchanges, comm: comm, tx: tx}
}

// RecordRequestCreated creates the request's private chat and notifies the recipient in the
// caller's transaction.
func (s *Service) RecordRequestCreated(ctx context.Context, request data.ExchangeRequest) error {
	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		convID, err := s.comm.EnsureRequestConversation(ctx, request.ID)
		if err != nil {
			return err
		}
		_, err = s.comm.CreateNotification(ctx, data.NewNotification{
			RecipientID:    request.Recipient.UserID,
			ActorID:        request.Requester.UserID,
			EventType:      data.EventExchangeRequest,
			EventKey:       "exchange-request:" + request.ID,
			Title:          "Новий запит на обмін",
			Body:           fmt.Sprintf("%s: %s ↔ %s.", fullName(request.Requester.FirstName, request.Requester.LastName), request.RequesterTeaches.Name, request.RecipientTeaches.Name),
			RequestID:      request.ID,
			ConversationID: convID,
		})
		return err
	})
}

// RecordRequestAccepted links the request chat to the new exchange and stores the notification
// for the requester in the caller's transaction (LR2 5.1, Figure 3).
func (s *Service) RecordRequestAccepted(ctx context.Context, request data.ExchangeRequest, exchangeID string) error {
	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		convID, err := s.comm.EnsureRequestConversation(ctx, request.ID)
		if err != nil {
			return err
		}
		_, err = s.comm.CreateNotification(ctx, data.NewNotification{
			RecipientID:    request.Requester.UserID,
			ActorID:        request.Recipient.UserID,
			EventType:      data.EventRequestResponse,
			EventKey:       "request-response:" + request.ID,
			Title:          "Запит на обмін прийнято",
			Body:           fmt.Sprintf("%s прийняв запит на обмін: %s ↔ %s.", fullName(request.Recipient.FirstName, request.Recipient.LastName), request.RequesterTeaches.Name, request.RecipientTeaches.Name),
			RequestID:      request.ID,
			ExchangeID:     exchangeID,
			ConversationID: convID,
		})
		return err
	})
}

// RecordRequestDeclined ensures the request chat exists and notifies the requester that the
// request was declined.
func (s *Service) RecordRequestDeclined(ctx context.Context, request data.ExchangeRequest) error {
	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		convID, err := s.comm.EnsureRequestConversation(ctx, request.ID)
		if err != nil {
			return err
		}
		_, err = s.comm.CreateNotification(ctx, data.NewNotification{
			RecipientID:    request.Requester.UserID,
			ActorID:        request.Recipient.UserID,
			EventType:      data.EventRequestResponse,
			EventKey:       "request-response:" + request.ID,
			Title:          "Запит на обмін відхилено",
			Body:           fmt.Sprintf("%s відхилив запит на обмін: %s ↔ %s.", fullName(request.Recipient.FirstName, request.Recipient.LastName), request.RequesterTeaches.Name, request.RecipientTeaches.Name),
			RequestID:      request.ID,
			ConversationID: convID,
		})
		return err
	})
}

type SessionUpdateEvent struct {
	RecipientID    string
	ActorID        string
	EventKey       string
	Title          string
	Body           string
	ExchangeID     string
	SessionID      string
	ConversationID string
}

// RecordSessionUpdated stores an idempotent notification when a lesson proposal or schedule
// changes (FR-13).
func (s *Service) RecordSessionUpdated(ctx context.Context, event SessionUpdateEvent) error {
	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		convID := event.ConversationID
		if convID == "" && event.ExchangeID != "" {
			var err error
			convID, err = s.comm.EnsureExchangeConversation(ctx, event.ExchangeID)
			if err != nil {
				return err
			}
		}
		_, err := s.comm.CreateNotification(ctx, data.NewNotification{
			RecipientID:    event.RecipientID,
			ActorID:        event.ActorID,
			EventType:      data.EventSessionUpdated,
			EventKey:       event.EventKey,
			Title:          event.Title,
			Body:           event.Body,
			ExchangeID:     event.ExchangeID,
			SessionID:      event.SessionID,
			ConversationID: convID,
		})
		return err
	})
}

// RecordReviewReceived notifies the reviewed partner in the caller's transaction (FR-12, FR-13).
func (s *Service) RecordReviewReceived(ctx context.Context, review data.Review, exchange data.Exchange) error {
	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		_, err := s.comm.CreateNotification(ctx, data.NewNotification{
			RecipientID: review.RecipientID,
			ActorID:     review.AuthorID,
			EventType:   data.EventReviewReceived,
			EventKey:    "review-received:" + review.ID,
			Title:       "Новий відгук",
			Body: fmt.Sprintf("%s: оцінка %d з 5 за обмін «%s ↔ %s».", fullName(review.AuthorFirstName, review.AuthorLastName),
				review.Rating, exchange.RequesterTeaches.Name, exchange.RecipientTeaches.Name),
			ExchangeID: exchange.ID,
			ReviewID:   review.ID,
		})
		return err
	})
}

type ConversationPage struct {
	Items []data.Conversation
	Page  int
	Total int
}

func (s *Service) Conversations(ctx context.Context, userID string, page int) (ConversationPage, error) {
	if page < 1 || page > MaxPage {
		return ConversationPage{}, ErrInvalidPage
	}
	items, total, err := s.comm.UserConversations(ctx, userID, PageSize, (page-1)*PageSize)
	if err != nil {
		return ConversationPage{}, err
	}
	return ConversationPage{Items: items, Page: page, Total: total}, nil
}

func (s *Service) Conversation(ctx context.Context, userID, conversationID string) (data.Conversation, error) {
	if !validate.UUID(conversationID) {
		return data.Conversation{}, ErrConversationNotFound
	}
	conv, err := s.comm.ConversationByID(ctx, userID, strings.ToLower(conversationID))
	if errors.Is(err, data.ErrNotFound) {
		return data.Conversation{}, ErrConversationNotFound
	}
	return conv, err
}

func (s *Service) RequestConversation(ctx context.Context, userID, requestID string) (data.Conversation, error) {
	if !validate.UUID(requestID) {
		return data.Conversation{}, ErrRequestNotFound
	}
	id := strings.ToLower(requestID)
	request, err := s.exchanges.RequestByID(ctx, id)
	switch {
	case errors.Is(err, data.ErrNotFound):
		return data.Conversation{}, ErrRequestNotFound
	case err != nil:
		return data.Conversation{}, err
	case userID != request.Requester.UserID && userID != request.Recipient.UserID:
		return data.Conversation{}, ErrRequestNotFound
	}
	var conv data.Conversation
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		convID, err := s.comm.EnsureRequestConversation(ctx, request.ID)
		if err != nil {
			return err
		}
		conv, err = s.comm.ConversationByID(ctx, userID, convID)
		return err
	})
	if errors.Is(err, data.ErrNotFound) {
		return data.Conversation{}, ErrRequestNotFound
	}
	return conv, err
}

func (s *Service) ExchangeConversation(ctx context.Context, userID, exchangeID string) (data.Conversation, error) {
	if !validate.UUID(exchangeID) {
		return data.Conversation{}, ErrExchangeNotFound
	}
	id := strings.ToLower(exchangeID)
	exchange, err := s.exchanges.ExchangeByID(ctx, id)
	switch {
	case errors.Is(err, data.ErrNotFound):
		return data.Conversation{}, ErrExchangeNotFound
	case err != nil:
		return data.Conversation{}, err
	case userID != exchange.Requester.UserID && userID != exchange.Recipient.UserID:
		return data.Conversation{}, ErrExchangeNotFound
	}
	var conv data.Conversation
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		convID, err := s.comm.EnsureExchangeConversation(ctx, exchange.ID)
		if err != nil {
			return err
		}
		conv, err = s.comm.ConversationByID(ctx, userID, convID)
		return err
	})
	if errors.Is(err, data.ErrNotFound) {
		return data.Conversation{}, ErrExchangeNotFound
	}
	return conv, err
}

type MessageQuery struct {
	Page    int
	AfterID string
}

type MessagePage struct {
	Items               []data.Message
	Page                int
	Total               int
	PollIntervalSeconds int
}

func (s *Service) Messages(ctx context.Context, userID, conversationID string, query MessageQuery) (MessagePage, error) {
	if !validate.UUID(conversationID) {
		return MessagePage{}, ErrConversationNotFound
	}
	page := query.Page
	if page == 0 {
		page = 1
	}
	if page < 1 || page > MaxPage {
		return MessagePage{}, ErrInvalidPage
	}
	afterID := strings.TrimSpace(query.AfterID)
	if afterID != "" {
		if !validate.UUID(afterID) {
			return MessagePage{}, &profile.ValidationError{Fields: map[string]string{
				"after": "Choose a message from this conversation",
			}}
		}
		if page > 1 {
			return MessagePage{}, &profile.ValidationError{Fields: map[string]string{
				"after": "after cannot be combined with page",
			}}
		}
		afterID = strings.ToLower(afterID)
	}
	items, total, err := s.comm.ConversationMessages(ctx, userID, strings.ToLower(conversationID), afterID, PageSize, (page-1)*PageSize)
	switch {
	case errors.Is(err, data.ErrNotFound):
		return MessagePage{}, ErrConversationNotFound
	case errors.Is(err, data.ErrInvalidCursor):
		return MessagePage{}, &profile.ValidationError{Fields: map[string]string{
			"after": "Choose a message from this conversation",
		}}
	case err != nil:
		return MessagePage{}, err
	}
	return MessagePage{
		Items:               items,
		Page:                page,
		Total:               total,
		PollIntervalSeconds: PollIntervalSeconds,
	}, nil
}

func (s *Service) SendMessage(ctx context.Context, userID, conversationID, rawBody string) (data.Message, error) {
	if !validate.UUID(conversationID) {
		return data.Message{}, ErrConversationNotFound
	}
	id := strings.ToLower(conversationID)
	body, ok := validate.Text(rawBody, MaxMessageLength)

	var created data.Message
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		conv, err := s.comm.LockConversation(ctx, userID, id)
		switch {
		case errors.Is(err, data.ErrNotFound):
			return ErrConversationNotFound
		case err != nil:
			return err
		case !ok || body == "":
			return &profile.ValidationError{Fields: map[string]string{
				"body": "Message must be between 1 and 2000 characters",
			}}
		case !conv.CanSend:
			return ErrConversationBlocked
		}
		created, err = s.comm.CreateMessage(ctx, conv.ID, userID, body)
		if err != nil {
			return err
		}
		recipientID := conv.Recipient.UserID
		if userID == recipientID {
			recipientID = conv.Requester.UserID
		}
		_, err = s.comm.CreateNotification(ctx, data.NewNotification{
			RecipientID:    recipientID,
			ActorID:        userID,
			EventType:      data.EventChatMessage,
			EventKey:       "chat-message:" + created.ID,
			Title:          "Нове повідомлення",
			Body:           fmt.Sprintf("%s: %s", fullName(created.AuthorFirstName, created.AuthorLastName), truncateText(created.Body, previewLength)),
			RequestID:      conv.RequestID,
			ExchangeID:     conv.ExchangeID,
			ConversationID: conv.ID,
			MessageID:      created.ID,
		})
		return err
	})
	return created, err
}

type ConversationReadResult struct {
	Conversation data.Conversation
	ReadCount    int
}

func (s *Service) MarkConversationRead(ctx context.Context, userID, conversationID string) (ConversationReadResult, error) {
	if !validate.UUID(conversationID) {
		return ConversationReadResult{}, ErrConversationNotFound
	}
	id := strings.ToLower(conversationID)
	var result ConversationReadResult
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		result.ReadCount, err = s.comm.MarkConversationRead(ctx, userID, id)
		if err != nil {
			return err
		}
		result.Conversation, err = s.comm.ConversationByID(ctx, userID, id)
		return err
	})
	if errors.Is(err, data.ErrNotFound) {
		return ConversationReadResult{}, ErrConversationNotFound
	}
	return result, err
}

type NotificationPage struct {
	Items       []data.Notification
	Page        int
	Total       int
	UnreadCount int
}

func (s *Service) Notifications(ctx context.Context, userID string, unreadOnly bool, page int) (NotificationPage, error) {
	if page < 1 || page > MaxPage {
		return NotificationPage{}, ErrInvalidPage
	}
	items, total, unreadCount, err := s.comm.UserNotifications(ctx, userID, unreadOnly, PageSize, (page-1)*PageSize)
	if err != nil {
		return NotificationPage{}, err
	}
	return NotificationPage{
		Items:       items,
		Page:        page,
		Total:       total,
		UnreadCount: unreadCount,
	}, nil
}

type NotificationReadResult struct {
	Notification data.Notification
	UnreadCount  int
}

func (s *Service) MarkNotificationRead(ctx context.Context, userID, notificationID string) (NotificationReadResult, error) {
	if !validate.UUID(notificationID) {
		return NotificationReadResult{}, ErrNotificationNotFound
	}
	id := strings.ToLower(notificationID)
	var result NotificationReadResult
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		result.Notification, result.UnreadCount, err = s.comm.MarkNotificationRead(ctx, userID, id)
		return err
	})
	if errors.Is(err, data.ErrNotFound) {
		return NotificationReadResult{}, ErrNotificationNotFound
	}
	return result, err
}

type NotificationsReadAllResult struct {
	MarkedCount int
	UnreadCount int
}

func (s *Service) MarkAllNotificationsRead(ctx context.Context, userID string) (NotificationsReadAllResult, error) {
	var marked int
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		marked, err = s.comm.MarkAllNotificationsRead(ctx, userID)
		return err
	})
	if err != nil {
		return NotificationsReadAllResult{}, err
	}
	return NotificationsReadAllResult{MarkedCount: marked, UnreadCount: 0}, nil
}

func fullName(firstName, lastName string) string {
	return strings.TrimSpace(firstName + " " + lastName)
}

func truncateText(text string, maxRunes int) string {
	normalized := strings.Join(strings.Fields(text), " ")
	runes := []rune(normalized)
	if len(runes) <= maxRunes {
		return normalized
	}
	return strings.TrimSpace(string(runes[:maxRunes-1])) + "…"
}
