package datatest

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	"skillswap/backend/internal/data"
)

var _ data.ICommunicationData = (*Memory)(nil)

type storedConversation struct {
	id            string
	kind          string
	requestID     string
	exchangeID    string
	participants  map[string]bool
	createdAt     time.Time
	lastMessageAt *time.Time
}

// SetBlocked records a personal block between two users.
func (m *Memory) SetBlocked(blockerID, blockedID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.blocks[[2]string{blockerID, blockedID}] = true
}

// SetAccountStatus changes the status of a user account, e.g. to SUSPENDED.
func (m *Memory) SetAccountStatus(userID, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if a, ok := m.users[userID]; ok {
		a.user.Status = status
	}
}

func (m *Memory) blocked(userA, userB string) bool {
	return m.blocks[[2]string{userA, userB}] || m.blocks[[2]string{userB, userA}]
}

func (m *Memory) EnsureRequestConversation(_ context.Context, requestID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.requests[requestID]
	if !ok {
		return "", data.ErrNotFound
	}
	for _, c := range m.conversations {
		if c.requestID == requestID {
			if r.ExchangeID != "" {
				c.exchangeID = r.ExchangeID
				c.kind = data.ConversationExchange
			}
			c.participants[r.Requester.UserID] = true
			c.participants[r.Recipient.UserID] = true
			return c.id, nil
		}
	}
	id := fmt.Sprintf("72000000-0000-0000-0000-%012d", len(m.conversations)+1)
	kind := data.ConversationRequest
	if r.ExchangeID != "" {
		kind = data.ConversationExchange
	}
	m.conversations[id] = &storedConversation{
		id:         id,
		kind:       kind,
		requestID:  requestID,
		exchangeID: r.ExchangeID,
		participants: map[string]bool{
			r.Requester.UserID: true,
			r.Recipient.UserID: true,
		},
		createdAt: r.CreatedAt,
	}
	return id, nil
}

func (m *Memory) EnsureExchangeConversation(ctx context.Context, exchangeID string) (string, error) {
	m.mu.Lock()
	e, ok := m.exchanges[exchangeID]
	var requestID string
	if ok {
		requestID = e.RequestID
	}
	m.mu.Unlock()
	if !ok {
		return "", data.ErrNotFound
	}
	if requestID != "" {
		return m.EnsureRequestConversation(ctx, requestID)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.conversations {
		if c.exchangeID == exchangeID {
			c.kind = data.ConversationExchange
			return c.id, nil
		}
	}
	id := fmt.Sprintf("72000000-0000-0000-0000-%012d", len(m.conversations)+1)
	m.conversations[id] = &storedConversation{
		id:         id,
		kind:       data.ConversationExchange,
		exchangeID: exchangeID,
		participants: map[string]bool{
			e.Requester.UserID: true,
			e.Recipient.UserID: true,
		},
		createdAt: e.CreatedAt,
	}
	return id, nil
}

func (m *Memory) buildConversation(userID string, c *storedConversation) (data.Conversation, bool) {
	if c == nil || !c.participants[userID] {
		return data.Conversation{}, false
	}
	out := data.Conversation{
		ID:            c.id,
		Type:          c.kind,
		RequestID:     c.requestID,
		ExchangeID:    c.exchangeID,
		LastMessageAt: c.lastMessageAt,
		CreatedAt:     c.createdAt,
	}
	if r, ok := m.requests[c.requestID]; ok {
		out.RequestStatus = r.Status
		out.Format = r.Format
		out.Requester = r.Requester
		out.Recipient = r.Recipient
		out.RequesterTeaches = r.RequesterTeaches
		out.RecipientTeaches = r.RecipientTeaches
		if r.ExchangeID != "" && out.ExchangeID == "" {
			out.ExchangeID = r.ExchangeID
		}
	}
	if e, ok := m.exchanges[out.ExchangeID]; ok {
		out.Type = data.ConversationExchange
		out.ExchangeStatus = e.Status
		out.Format = e.Format
		out.Requester = e.Requester
		out.Recipient = e.Recipient
		out.RequesterTeaches = e.RequesterTeaches
		out.RecipientTeaches = e.RecipientTeaches
	}
	if a, ok := m.users[out.Requester.UserID]; ok {
		out.Requester = m.party(a)
	}
	if b, ok := m.users[out.Recipient.UserID]; ok {
		out.Recipient = m.party(b)
	}
	requester, recipient := m.users[out.Requester.UserID], m.users[out.Recipient.UserID]
	out.CanSend = requester != nil && recipient != nil &&
		requester.user.Status == "ACTIVE" && recipient.user.Status == "ACTIVE" &&
		!m.blocked(out.Requester.UserID, out.Recipient.UserID)

	msgs := m.messages[c.id]
	if len(msgs) > 0 {
		last := msgs[len(msgs)-1]
		out.LastMessage = &data.MessagePreview{
			ID:              last.ID,
			AuthorID:        last.AuthorID,
			AuthorFirstName: last.AuthorFirstName,
			AuthorLastName:  last.AuthorLastName,
			Body:            last.Body,
			CreatedAt:       last.CreatedAt,
		}
	}
	for _, msg := range msgs {
		if msg.AuthorID != "" && msg.AuthorID != userID {
			if _, read := m.receipts[[2]string{msg.ID, userID}]; !read {
				out.UnreadCount++
			}
		}
	}
	return out, true
}

func (m *Memory) ConversationByID(_ context.Context, userID, conversationID string) (data.Conversation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out, ok := m.buildConversation(userID, m.conversations[conversationID])
	if !ok {
		return data.Conversation{}, data.ErrNotFound
	}
	return out, nil
}

func (m *Memory) UserConversations(_ context.Context, userID string, limit, offset int) ([]data.Conversation, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	found := []data.Conversation{}
	for _, c := range m.conversations {
		if conv, ok := m.buildConversation(userID, c); ok {
			found = append(found, conv)
		}
	}
	activity := func(c data.Conversation) time.Time {
		if c.LastMessageAt != nil {
			return *c.LastMessageAt
		}
		return c.CreatedAt
	}
	slices.SortFunc(found, func(a, b data.Conversation) int {
		return cmp.Or(activity(b).Compare(activity(a)), cmp.Compare(b.ID, a.ID))
	})
	total := len(found)
	return append([]data.Conversation{}, found[min(offset, total):min(offset+limit, total)]...), total, nil
}

func (m *Memory) LockConversation(ctx context.Context, userID, conversationID string) (data.Conversation, error) {
	return m.ConversationByID(ctx, userID, conversationID)
}

func (m *Memory) CreateMessage(_ context.Context, conversationID, authorID, body string) (data.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.conversations[conversationID]
	if !ok {
		return data.Message{}, data.ErrNotFound
	}
	var count int
	for _, list := range m.messages {
		count += len(list)
	}
	now := time.Now().UTC()
	if c.lastMessageAt != nil && !now.After(*c.lastMessageAt) {
		now = c.lastMessageAt.Add(time.Microsecond)
	}
	id := fmt.Sprintf("73000000-0000-0000-0000-%012d", count+1)
	msg := &data.Message{
		ID:             id,
		ConversationID: conversationID,
		AuthorID:       authorID,
		Type:           data.MessageText,
		Body:           body,
		CreatedAt:      now,
	}
	if a, ok := m.users[authorID]; ok {
		msg.AuthorFirstName = a.profile.FirstName
		msg.AuthorLastName = a.profile.LastName
	}
	m.messages[conversationID] = append(m.messages[conversationID], msg)
	c.lastMessageAt = &now
	return *msg, nil
}

func (m *Memory) messageWithReceipt(msg *data.Message) data.Message {
	out := *msg
	if a, ok := m.users[msg.AuthorID]; ok {
		out.AuthorFirstName = a.profile.FirstName
		out.AuthorLastName = a.profile.LastName
	}
	for key, readAt := range m.receipts {
		if key[0] == msg.ID && key[1] != msg.AuthorID {
			t := readAt
			out.ReadAt = &t
			break
		}
	}
	return out
}

func (m *Memory) ConversationMessages(_ context.Context, userID, conversationID, afterID string, limit, offset int) ([]data.Message, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.conversations[conversationID]
	if !ok || !c.participants[userID] {
		return nil, 0, data.ErrNotFound
	}
	all := make([]data.Message, 0, len(m.messages[conversationID]))
	for _, msg := range m.messages[conversationID] {
		all = append(all, m.messageWithReceipt(msg))
	}
	slices.SortFunc(all, func(a, b data.Message) int {
		return cmp.Or(a.CreatedAt.Compare(b.CreatedAt), cmp.Compare(a.ID, b.ID))
	})
	if afterID != "" {
		idx := slices.IndexFunc(all, func(msg data.Message) bool { return msg.ID == afterID })
		if idx < 0 {
			return nil, 0, data.ErrInvalidCursor
		}
		after := all[idx+1:]
		total := len(after)
		return append([]data.Message{}, after[:min(limit, total)]...), total, nil
	}
	total := len(all)
	return append([]data.Message{}, all[min(offset, total):min(offset+limit, total)]...), total, nil
}

func (m *Memory) MarkConversationRead(_ context.Context, userID, conversationID string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.conversations[conversationID]
	if !ok || !c.participants[userID] {
		return 0, data.ErrNotFound
	}
	now := time.Now().UTC()
	readCount := 0
	for _, msg := range m.messages[conversationID] {
		if msg.AuthorID != "" && msg.AuthorID != userID {
			key := [2]string{msg.ID, userID}
			if _, exists := m.receipts[key]; !exists {
				m.receipts[key] = now
				readCount++
			}
		}
	}
	for _, n := range m.notifications {
		if n.RecipientID == userID && n.ConversationID == conversationID && n.ReadAt == nil {
			t := now
			n.ReadAt = &t
		}
	}
	return readCount, nil
}

func (m *Memory) CreateNotification(_ context.Context, in data.NewNotification) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.notifications {
		if existing.RecipientID == in.RecipientID && existing.EventKey == in.EventKey {
			return false, nil
		}
	}
	now := time.Now().UTC()
	for _, existing := range m.notifications {
		if !now.After(existing.CreatedAt) {
			now = existing.CreatedAt.Add(time.Microsecond)
		}
	}
	id := fmt.Sprintf("74000000-0000-0000-0000-%012d", len(m.notifications)+1)
	n := &data.Notification{
		ID:             id,
		RecipientID:    in.RecipientID,
		ActorID:        in.ActorID,
		EventType:      in.EventType,
		EventKey:       in.EventKey,
		Title:          in.Title,
		Body:           in.Body,
		RequestID:      in.RequestID,
		ExchangeID:     in.ExchangeID,
		ConversationID: in.ConversationID,
		MessageID:      in.MessageID,
		SessionID:      in.SessionID,
		CreatedAt:      now,
	}
	if a, ok := m.users[in.ActorID]; ok {
		n.ActorFirstName = a.profile.FirstName
		n.ActorLastName = a.profile.LastName
	}
	m.notifications[id] = n
	return true, nil
}

func (m *Memory) hydrateNotification(n *data.Notification) data.Notification {
	out := *n
	if a, ok := m.users[n.ActorID]; ok {
		out.ActorFirstName = a.profile.FirstName
		out.ActorLastName = a.profile.LastName
	}
	return out
}

func (m *Memory) UserNotifications(_ context.Context, userID string, unreadOnly bool, limit, offset int) ([]data.Notification, int, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	found := []data.Notification{}
	unreadCount := 0
	for _, n := range m.notifications {
		if n.RecipientID != userID {
			continue
		}
		if n.ReadAt == nil {
			unreadCount++
		}
		if !unreadOnly || n.ReadAt == nil {
			found = append(found, m.hydrateNotification(n))
		}
	}
	slices.SortFunc(found, func(a, b data.Notification) int {
		return cmp.Or(b.CreatedAt.Compare(a.CreatedAt), cmp.Compare(b.ID, a.ID))
	})
	total := len(found)
	return append([]data.Notification{}, found[min(offset, total):min(offset+limit, total)]...), total, unreadCount, nil
}

func (m *Memory) MarkNotificationRead(_ context.Context, userID, notificationID string) (data.Notification, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n, ok := m.notifications[notificationID]
	if !ok || n.RecipientID != userID {
		return data.Notification{}, 0, data.ErrNotFound
	}
	if n.ReadAt == nil {
		now := time.Now().UTC()
		n.ReadAt = &now
	}
	if n.MessageID != "" {
		key := [2]string{n.MessageID, userID}
		if _, exists := m.receipts[key]; !exists {
			m.receipts[key] = *n.ReadAt
		}
	}
	unreadCount := 0
	for _, item := range m.notifications {
		if item.RecipientID == userID && item.ReadAt == nil {
			unreadCount++
		}
	}
	return m.hydrateNotification(n), unreadCount, nil
}

func (m *Memory) MarkAllNotificationsRead(_ context.Context, userID string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	marked := 0
	for _, n := range m.notifications {
		if n.RecipientID == userID && n.ReadAt == nil {
			t := now
			n.ReadAt = &t
			marked++
			if n.MessageID != "" {
				key := [2]string{n.MessageID, userID}
				if _, exists := m.receipts[key]; !exists {
					m.receipts[key] = t
				}
			}
		}
	}
	return marked, nil
}
