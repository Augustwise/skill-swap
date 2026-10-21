package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"skillswap/backend/internal/communication"
	"skillswap/backend/internal/data"
	"skillswap/backend/internal/profile"
)

const communicationTimeout = 5 * time.Second

type messagePreviewResponse struct {
	ID        string    `json:"id"`
	Author    *person   `json:"author"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
}

type conversationResponse struct {
	ID               string                  `json:"id"`
	Type             string                  `json:"type"`
	RequestID        *string                 `json:"requestId"`
	ExchangeID       *string                 `json:"exchangeId"`
	RequestStatus    *string                 `json:"requestStatus"`
	ExchangeStatus   *string                 `json:"exchangeStatus"`
	Format           string                  `json:"format"`
	Partner          studentSummary          `json:"partner"`
	Requester        studentSummary          `json:"requester"`
	Recipient        studentSummary          `json:"recipient"`
	RequesterTeaches requestTerms            `json:"requesterTeaches"`
	RecipientTeaches requestTerms            `json:"recipientTeaches"`
	LastMessage      *messagePreviewResponse `json:"lastMessage"`
	LastMessageAt    *time.Time              `json:"lastMessageAt"`
	UnreadCount      int                     `json:"unreadCount"`
	CanSend          bool                    `json:"canSend"`
	CreatedAt        time.Time               `json:"createdAt"`
}

func toConversation(c data.Conversation, viewerID string) conversationResponse {
	requester := toRequestParty(c.Requester)
	recipient := toRequestParty(c.Recipient)
	partner := recipient
	if viewerID == c.Recipient.UserID {
		partner = requester
	}
	response := conversationResponse{
		ID:               c.ID,
		Type:             c.Type,
		Format:           c.Format,
		Partner:          partner,
		Requester:        requester,
		Recipient:        recipient,
		RequesterTeaches: toRequestTerms(c.RequesterTeaches),
		RecipientTeaches: toRequestTerms(c.RecipientTeaches),
		LastMessageAt:    c.LastMessageAt,
		UnreadCount:      c.UnreadCount,
		CanSend:          c.CanSend,
		CreatedAt:        c.CreatedAt,
	}
	if c.RequestID != "" {
		response.RequestID = &c.RequestID
	}
	if c.ExchangeID != "" {
		response.ExchangeID = &c.ExchangeID
	}
	if c.RequestStatus != "" {
		response.RequestStatus = &c.RequestStatus
	}
	if c.ExchangeStatus != "" {
		response.ExchangeStatus = &c.ExchangeStatus
	}
	if c.LastMessage != nil {
		preview := &messagePreviewResponse{
			ID:        c.LastMessage.ID,
			Body:      c.LastMessage.Body,
			CreatedAt: c.LastMessage.CreatedAt,
		}
		if c.LastMessage.AuthorID != "" {
			preview.Author = &person{
				ID:        c.LastMessage.AuthorID,
				FirstName: c.LastMessage.AuthorFirstName,
				LastName:  c.LastMessage.AuthorLastName,
			}
		}
		response.LastMessage = preview
	}
	return response
}

type messageResponse struct {
	ID             string     `json:"id"`
	ConversationID string     `json:"conversationId"`
	Author         *person    `json:"author"`
	Type           string     `json:"type"`
	Body           string     `json:"body"`
	CreatedAt      time.Time  `json:"createdAt"`
	EditedAt       *time.Time `json:"editedAt"`
	ReadAt         *time.Time `json:"readAt"`
}

func toMessage(m data.Message) messageResponse {
	response := messageResponse{
		ID:             m.ID,
		ConversationID: m.ConversationID,
		Type:           m.Type,
		Body:           m.Body,
		CreatedAt:      m.CreatedAt,
		EditedAt:       m.EditedAt,
		ReadAt:         m.ReadAt,
	}
	if m.AuthorID != "" {
		response.Author = &person{ID: m.AuthorID, FirstName: m.AuthorFirstName, LastName: m.AuthorLastName}
	}
	return response
}

type notificationTarget struct {
	Kind           string  `json:"kind"`
	ID             string  `json:"id"`
	RequestID      *string `json:"requestId"`
	ExchangeID     *string `json:"exchangeId"`
	ConversationID *string `json:"conversationId"`
	SessionID      *string `json:"sessionId"`
}

type notificationResponse struct {
	ID             string             `json:"id"`
	EventType      string             `json:"eventType"`
	Title          string             `json:"title"`
	Body           *string            `json:"body"`
	Actor          *person            `json:"actor"`
	Target         notificationTarget `json:"target"`
	RequestID      *string            `json:"requestId"`
	ExchangeID     *string            `json:"exchangeId"`
	ConversationID *string            `json:"conversationId"`
	MessageID      *string            `json:"messageId"`
	SessionID      *string            `json:"sessionId"`
	ReviewID       *string            `json:"reviewId"`
	IsRead         bool               `json:"isRead"`
	ReadAt         *time.Time         `json:"readAt"`
	CreatedAt      time.Time          `json:"createdAt"`
}

func toNotification(n data.Notification) notificationResponse {
	response := notificationResponse{
		ID:        n.ID,
		EventType: n.EventType,
		Title:     n.Title,
		IsRead:    n.ReadAt != nil,
		ReadAt:    n.ReadAt,
		CreatedAt: n.CreatedAt,
	}
	if n.Body != "" {
		response.Body = &n.Body
	}
	if n.ActorID != "" {
		response.Actor = &person{ID: n.ActorID, FirstName: n.ActorFirstName, LastName: n.ActorLastName}
	}
	if n.RequestID != "" {
		response.RequestID = &n.RequestID
	}
	if n.ExchangeID != "" {
		response.ExchangeID = &n.ExchangeID
	}
	if n.ConversationID != "" {
		response.ConversationID = &n.ConversationID
	}
	if n.MessageID != "" {
		response.MessageID = &n.MessageID
	}
	if n.SessionID != "" {
		response.SessionID = &n.SessionID
	}
	if n.ReviewID != "" {
		response.ReviewID = &n.ReviewID
	}
	target := notificationTarget{
		RequestID:      response.RequestID,
		ExchangeID:     response.ExchangeID,
		ConversationID: response.ConversationID,
		SessionID:      response.SessionID,
	}
	switch n.EventType {
	case data.EventExchangeRequest:
		target.Kind, target.ID = "EXCHANGE_REQUEST", n.RequestID
	case data.EventRequestResponse:
		if n.ExchangeID != "" {
			target.Kind, target.ID = "EXCHANGE", n.ExchangeID
		} else {
			target.Kind, target.ID = "EXCHANGE_REQUEST", n.RequestID
		}
	case data.EventChatMessage:
		target.Kind, target.ID = "CONVERSATION", n.ConversationID
	case data.EventSessionUpdated:
		if n.SessionID != "" {
			target.Kind, target.ID = "SESSION", n.SessionID
		} else {
			target.Kind, target.ID = "EXCHANGE", n.ExchangeID
		}
	case data.EventReviewReceived:
		target.Kind, target.ID = "EXCHANGE", n.ExchangeID
	default:
		target.Kind, target.ID = n.EventType, n.ID
	}
	response.Target = target
	return response
}

func (a *API) listConversations(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	page, ok := pageParam(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), communicationTimeout)
	defer cancel()
	result, err := a.app.Conversations(ctx, current.ID, page)
	if err != nil {
		a.communicationError(w, err)
		return
	}
	items := make([]conversationResponse, 0, len(result.Items))
	for _, c := range result.Items {
		items = append(items, toConversation(c, current.ID))
	}
	respond(w, http.StatusOK, map[string]any{
		"items":    items,
		"page":     result.Page,
		"pageSize": communication.PageSize,
		"total":    result.Total,
	})
}

func (a *API) conversationDetails(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	ctx, cancel := context.WithTimeout(r.Context(), communicationTimeout)
	defer cancel()
	conv, err := a.app.Conversation(ctx, current.ID, r.PathValue("conversationId"))
	if err != nil {
		a.communicationError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{"conversation": toConversation(conv, current.ID)})
}

func (a *API) requestConversation(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	ctx, cancel := context.WithTimeout(r.Context(), communicationTimeout)
	defer cancel()
	conv, err := a.app.RequestConversation(ctx, current.ID, r.PathValue("requestId"))
	if err != nil {
		a.communicationError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{"conversation": toConversation(conv, current.ID)})
}

func (a *API) exchangeConversation(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	ctx, cancel := context.WithTimeout(r.Context(), communicationTimeout)
	defer cancel()
	conv, err := a.app.ExchangeConversation(ctx, current.ID, r.PathValue("exchangeId"))
	if err != nil {
		a.communicationError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{"conversation": toConversation(conv, current.ID)})
}

func (a *API) listMessages(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	page, ok := pageParam(w, r)
	if !ok {
		return
	}
	query := communication.MessageQuery{
		Page:    page,
		AfterID: r.URL.Query().Get("after"),
	}
	ctx, cancel := context.WithTimeout(r.Context(), communicationTimeout)
	defer cancel()
	result, err := a.app.Messages(ctx, current.ID, r.PathValue("conversationId"), query)
	if err != nil {
		a.communicationError(w, err)
		return
	}
	items := make([]messageResponse, 0, len(result.Items))
	for _, m := range result.Items {
		items = append(items, toMessage(m))
	}
	respond(w, http.StatusOK, map[string]any{
		"items":               items,
		"page":                result.Page,
		"pageSize":            communication.PageSize,
		"total":               result.Total,
		"pollIntervalSeconds": result.PollIntervalSeconds,
	})
}

type sendMessageBody struct {
	Body string `json:"body"`
}

func (a *API) sendMessage(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	var body sendMessageBody
	if !decodeBody(w, r, &body) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), communicationTimeout)
	defer cancel()
	created, err := a.app.SendMessage(ctx, current.ID, r.PathValue("conversationId"), body.Body)
	if err != nil {
		a.communicationError(w, err)
		return
	}
	respond(w, http.StatusCreated, map[string]any{"message": toMessage(created)})
}

func (a *API) markConversationRead(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	ctx, cancel := context.WithTimeout(r.Context(), communicationTimeout)
	defer cancel()
	result, err := a.app.MarkConversationRead(ctx, current.ID, r.PathValue("conversationId"))
	if err != nil {
		a.communicationError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{
		"conversation": toConversation(result.Conversation, current.ID),
		"readCount":    result.ReadCount,
	})
}

func (a *API) listNotifications(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	page, ok := pageParam(w, r)
	if !ok {
		return
	}
	unreadOnly := false
	switch r.URL.Query().Get("unreadOnly") {
	case "", "false":
	case "true":
		unreadOnly = true
	default:
		validationProblem(w, map[string]string{"unreadOnly": "unreadOnly must be true or false"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), communicationTimeout)
	defer cancel()
	result, err := a.app.Notifications(ctx, current.ID, unreadOnly, page)
	if err != nil {
		a.communicationError(w, err)
		return
	}
	items := make([]notificationResponse, 0, len(result.Items))
	for _, n := range result.Items {
		items = append(items, toNotification(n))
	}
	respond(w, http.StatusOK, map[string]any{
		"items":       items,
		"page":        result.Page,
		"pageSize":    communication.PageSize,
		"total":       result.Total,
		"unreadCount": result.UnreadCount,
	})
}

func (a *API) markNotificationRead(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	ctx, cancel := context.WithTimeout(r.Context(), communicationTimeout)
	defer cancel()
	result, err := a.app.MarkNotificationRead(ctx, current.ID, r.PathValue("notificationId"))
	if err != nil {
		a.communicationError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{
		"notification": toNotification(result.Notification),
		"unreadCount":  result.UnreadCount,
	})
}

func (a *API) markAllNotificationsRead(w http.ResponseWriter, r *http.Request, current data.User, _ string) {
	ctx, cancel := context.WithTimeout(r.Context(), communicationTimeout)
	defer cancel()
	result, err := a.app.MarkAllNotificationsRead(ctx, current.ID)
	if err != nil {
		a.communicationError(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{
		"markedCount": result.MarkedCount,
		"unreadCount": result.UnreadCount,
	})
}

func (a *API) communicationError(w http.ResponseWriter, err error) {
	var invalid *profile.ValidationError
	switch {
	case errors.As(err, &invalid):
		validationProblem(w, invalid.Fields)
	case errors.Is(err, communication.ErrConversationNotFound):
		problem(w, http.StatusNotFound, "conversation_not_found", "Conversation was not found")
	case errors.Is(err, communication.ErrConversationBlocked):
		problem(w, http.StatusConflict, "conversation_blocked", "New messages cannot be sent in this conversation")
	case errors.Is(err, communication.ErrRequestNotFound):
		problem(w, http.StatusNotFound, "request_not_found", "Request was not found")
	case errors.Is(err, communication.ErrExchangeNotFound):
		problem(w, http.StatusNotFound, "exchange_not_found", "Exchange was not found")
	case errors.Is(err, communication.ErrNotificationNotFound):
		problem(w, http.StatusNotFound, "notification_not_found", "Notification was not found")
	case errors.Is(err, communication.ErrInvalidPage):
		problem(w, http.StatusBadRequest, "invalid_page", fmt.Sprintf("page must be a whole number from 1 to %d", communication.MaxPage))
	default:
		a.queryError(w, err)
	}
}
