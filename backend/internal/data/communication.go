package data

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	ConversationRequest  = "REQUEST"
	ConversationExchange = "EXCHANGE"

	MessageText = "TEXT"

	EventExchangeRequest = "EXCHANGE_REQUEST"
	EventRequestResponse = "REQUEST_RESPONSE"
	EventChatMessage     = "CHAT_MESSAGE"
	EventSessionUpdated  = "SESSION_UPDATED"
	EventReviewReceived  = "REVIEW_RECEIVED"
)

var ErrInvalidCursor = errors.New("message cursor does not belong to this conversation")

type MessagePreview struct {
	ID              string
	AuthorID        string
	AuthorFirstName string
	AuthorLastName  string
	Body            string
	CreatedAt       time.Time
}

type Conversation struct {
	ID               string
	Type             string
	RequestID        string
	ExchangeID       string // empty until the request is accepted
	RequestStatus    string
	ExchangeStatus   string // empty when there is no exchange
	Format           string
	Requester        RequestParty
	Recipient        RequestParty
	RequesterTeaches RequestTerms
	RecipientTeaches RequestTerms
	LastMessage      *MessagePreview
	LastMessageAt    *time.Time
	UnreadCount      int
	CanSend          bool
	CreatedAt        time.Time
}

type Message struct {
	ID              string
	ConversationID  string
	AuthorID        string
	AuthorFirstName string
	AuthorLastName  string
	Type            string
	Body            string
	CreatedAt       time.Time
	EditedAt        *time.Time
	ReadAt          *time.Time
}

type NewNotification struct {
	RecipientID    string
	ActorID        string
	EventType      string
	EventKey       string
	Title          string
	Body           string
	RequestID      string
	ExchangeID     string
	ConversationID string
	MessageID      string
	SessionID      string
	ReviewID       string
}

type Notification struct {
	ID             string
	RecipientID    string
	ActorID        string
	ActorFirstName string
	ActorLastName  string
	EventType      string
	EventKey       string
	Title          string
	Body           string
	RequestID      string
	ExchangeID     string
	ConversationID string
	MessageID      string
	SessionID      string
	ReviewID       string
	CreatedAt      time.Time
	ReadAt         *time.Time
}

type ICommunicationData interface {
	EnsureRequestConversation(ctx context.Context, requestID string) (string, error)
	EnsureExchangeConversation(ctx context.Context, exchangeID string) (string, error)
	ConversationByID(ctx context.Context, userID, conversationID string) (Conversation, error)
	UserConversations(ctx context.Context, userID string, limit, offset int) ([]Conversation, int, error)
	LockConversation(ctx context.Context, userID, conversationID string) (Conversation, error)
	CreateMessage(ctx context.Context, conversationID, authorID, body string) (Message, error)
	ConversationMessages(ctx context.Context, userID, conversationID, afterID string, limit, offset int) ([]Message, int, error)
	MarkConversationRead(ctx context.Context, userID, conversationID string) (int, error)
	CreateNotification(ctx context.Context, notification NewNotification) (bool, error)
	UserNotifications(ctx context.Context, userID string, unreadOnly bool, limit, offset int) ([]Notification, int, int, error)
	MarkNotificationRead(ctx context.Context, userID, notificationID string) (Notification, int, error)
	MarkAllNotificationsRead(ctx context.Context, userID string) (int, error)
}

var _ ICommunicationData = (*Postgres)(nil)

// LockUsers locks two user rows in canonical UUID order (LR2 5.2, ARCH-06).
func (p *Postgres) LockUsers(ctx context.Context, userA, userB string) error {
	rows, err := p.conn(ctx).Query(ctx, `SELECT id FROM users WHERE id IN ($1, $2) ORDER BY id FOR UPDATE`, userA, userB)
	if err != nil {
		return err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return ErrNotFound
	}
	return nil
}

// EnsureRequestConversation creates the private chat of a request or links an existing request
// chat to its accepted exchange.
func (p *Postgres) EnsureRequestConversation(ctx context.Context, requestID string) (string, error) {
	var id string
	err := p.conn(ctx).QueryRow(ctx, `WITH r AS (
			SELECT r.id AS request_id, r.requester_id, r.recipient_id, r.created_at, e.id AS exchange_id
			FROM exchange_requests r
			LEFT JOIN exchanges e ON e.source_request_id = r.id
			WHERE r.id = $1
		), conv AS (
			INSERT INTO conversations (type, exchange_request_id, exchange_id, created_by_user_id, created_at)
			SELECT CASE WHEN exchange_id IS NOT NULL THEN 'EXCHANGE'::conversation_type ELSE 'REQUEST'::conversation_type END,
				request_id, exchange_id, requester_id, created_at
			FROM r
			ON CONFLICT (exchange_request_id) DO UPDATE
				SET exchange_id = COALESCE(conversations.exchange_id, EXCLUDED.exchange_id),
					type = CASE WHEN COALESCE(conversations.exchange_id, EXCLUDED.exchange_id) IS NOT NULL
						THEN 'EXCHANGE'::conversation_type ELSE conversations.type END
			RETURNING id
		), participants AS (
			INSERT INTO conversation_participants (conversation_id, user_id, joined_at)
			SELECT conv.id, participant.user_id, r.created_at
			FROM conv CROSS JOIN r
			CROSS JOIN LATERAL (VALUES (r.requester_id), (r.recipient_id)) AS participant(user_id)
			ON CONFLICT (conversation_id, user_id) DO NOTHING
		)
		SELECT id FROM conv`, requestID).Scan(&id)
	if err != nil {
		return "", notFound(err)
	}
	return id, nil
}

// EnsureExchangeConversation returns the conversation for an exchange, reusing the source
// request conversation when the exchange started from a request.
func (p *Postgres) EnsureExchangeConversation(ctx context.Context, exchangeID string) (string, error) {
	var sourceRequestID string
	err := p.conn(ctx).QueryRow(ctx, `SELECT COALESCE(source_request_id::text, '')
		FROM exchanges WHERE id = $1`, exchangeID).Scan(&sourceRequestID)
	if err != nil {
		return "", notFound(err)
	}
	if sourceRequestID != "" {
		return p.EnsureRequestConversation(ctx, sourceRequestID)
	}
	var id string
	err = p.conn(ctx).QueryRow(ctx, `WITH e AS (
			SELECT id AS exchange_id, user_a_id, user_b_id, created_at
			FROM exchanges WHERE id = $1
		), conv AS (
			INSERT INTO conversations (type, exchange_id, created_by_user_id, created_at)
			SELECT 'EXCHANGE', exchange_id, user_a_id, created_at FROM e
			ON CONFLICT (exchange_id) DO UPDATE SET type = 'EXCHANGE'
			RETURNING id
		), participants AS (
			INSERT INTO conversation_participants (conversation_id, user_id, joined_at)
			SELECT conv.id, participant.user_id, e.created_at
			FROM conv CROSS JOIN e
			CROSS JOIN LATERAL (VALUES (e.user_a_id), (e.user_b_id)) AS participant(user_id)
			ON CONFLICT (conversation_id, user_id) DO NOTHING
		)
		SELECT id FROM conv`, exchangeID).Scan(&id)
	if err != nil {
		return "", notFound(err)
	}
	return id, nil
}

var conversationRequesterTerms = `CASE WHEN e.id IS NOT NULL
	THEN ` + commitmentTerms("e.user_a_id") + `
	ELSE ` + requestTerms("requester_teaching_skill_id", "recipient_learning_skill_id", "requester_sessions", "requester_duration_minutes") + `
END`

var conversationRecipientTerms = `CASE WHEN e.id IS NOT NULL
	THEN ` + commitmentTerms("e.user_b_id") + `
	ELSE ` + requestTerms("recipient_teaching_skill_id", "requester_learning_skill_id", "recipient_sessions", "recipient_duration_minutes") + `
END`

const conversationLastMessage = `(SELECT json_build_object(
		'ID', m.id,
		'AuthorID', COALESCE(m.author_id::text, ''),
		'AuthorFirstName', COALESCE(u.first_name, ''),
		'AuthorLastName', COALESCE(u.last_name, ''),
		'Body', COALESCE(m.body, ''),
		'CreatedAt', m.created_at
	)
	FROM messages m
	LEFT JOIN users u ON u.id = m.author_id
	WHERE m.conversation_id = c.id AND m.deleted_at IS NULL
	ORDER BY m.created_at DESC, m.id DESC
	LIMIT 1)`

const conversationUnreadCount = `(SELECT count(*)
	FROM messages m
	LEFT JOIN message_receipts mr ON mr.message_id = m.id AND mr.user_id = $1
	WHERE m.conversation_id = c.id AND m.deleted_at IS NULL
		AND m.author_id IS NOT NULL AND m.author_id <> $1
		AND mr.read_at IS NULL)`

const conversationCanSend = `(rq.deleted_at IS NULL AND rq.account_status = 'ACTIVE'
	AND rc.deleted_at IS NULL AND rc.account_status = 'ACTIVE'
	AND NOT EXISTS (SELECT 1 FROM user_blocks b
		WHERE (b.blocker_id = rq.id AND b.blocked_id = rc.id)
			OR (b.blocker_id = rc.id AND b.blocked_id = rq.id)))`

var conversationColumns = `c.id, c.type::text, COALESCE(c.exchange_request_id::text, ''),
	COALESCE(c.exchange_id::text, ''), COALESCE(r.status::text, ''), COALESCE(e.status::text, ''),
	COALESCE(e.format::text, r.format::text, 'ONLINE'),
	` + requestParty("rq") + `, ` + requestParty("rc") + `,
	` + conversationRequesterTerms + `, ` + conversationRecipientTerms + `,
	` + conversationLastMessage + `, c.last_message_at,
	` + conversationUnreadCount + `, ` + conversationCanSend + `, c.created_at`

const conversationFrom = `
	FROM conversations c
	JOIN conversation_participants cp ON cp.conversation_id = c.id AND cp.user_id = $1
	LEFT JOIN exchange_requests r ON r.id = c.exchange_request_id
	LEFT JOIN exchanges e ON e.id = c.exchange_id
	JOIN users rq ON rq.id = COALESCE(r.requester_id, e.user_a_id)
	JOIN users rc ON rc.id = COALESCE(r.recipient_id, e.user_b_id)`

var conversationSelect = `SELECT ` + conversationColumns + conversationFrom

func (c *Conversation) scanTargets(extra ...any) []any {
	return append([]any{&c.ID, &c.Type, &c.RequestID, &c.ExchangeID, &c.RequestStatus, &c.ExchangeStatus,
		&c.Format, &c.Requester, &c.Recipient, &c.RequesterTeaches, &c.RecipientTeaches,
		&c.LastMessage, &c.LastMessageAt, &c.UnreadCount, &c.CanSend, &c.CreatedAt}, extra...)
}

func (p *Postgres) ConversationByID(ctx context.Context, userID, conversationID string) (Conversation, error) {
	var c Conversation
	err := p.conn(ctx).QueryRow(ctx, conversationSelect+` WHERE c.id = $2`, userID, conversationID).Scan(c.scanTargets()...)
	if err != nil {
		return Conversation{}, notFound(err)
	}
	return c, nil
}

func (p *Postgres) UserConversations(ctx context.Context, userID string, limit, offset int) ([]Conversation, int, error) {
	rows, err := p.conn(ctx).Query(ctx, `SELECT `+conversationColumns+`, count(*) OVER ()`+conversationFrom+`
		ORDER BY COALESCE(c.last_message_at, c.created_at) DESC, c.id DESC
		LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	total := 0
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Conversation, error) {
		var c Conversation
		err := row.Scan(c.scanTargets(&total)...)
		return c, err
	})
	if err != nil {
		return nil, 0, err
	}
	if len(items) == 0 && offset > 0 {
		err = p.conn(ctx).QueryRow(ctx, `SELECT count(*) FROM conversation_participants WHERE user_id = $1`,
			userID).Scan(&total)
	}
	return items, total, err
}

// LockConversation verifies participant membership and locks both participants in UUID order
// before checking whether new messages are allowed.
func (p *Postgres) LockConversation(ctx context.Context, userID, conversationID string) (Conversation, error) {
	var userA, userB string
	err := p.conn(ctx).QueryRow(ctx, `SELECT COALESCE(r.requester_id, e.user_a_id), COALESCE(r.recipient_id, e.user_b_id)
		FROM conversations c
		JOIN conversation_participants cp ON cp.conversation_id = c.id AND cp.user_id = $1
		LEFT JOIN exchange_requests r ON r.id = c.exchange_request_id
		LEFT JOIN exchanges e ON e.id = c.exchange_id
		WHERE c.id = $2`, userID, conversationID).Scan(&userA, &userB)
	if err != nil {
		return Conversation{}, notFound(err)
	}
	if err := p.LockUsers(ctx, userA, userB); err != nil {
		return Conversation{}, err
	}
	return p.ConversationByID(ctx, userID, conversationID)
}

func (p *Postgres) CreateMessage(ctx context.Context, conversationID, authorID, body string) (Message, error) {
	var m Message
	err := p.conn(ctx).QueryRow(ctx, `WITH created AS (
			INSERT INTO messages (conversation_id, author_id, type, body, created_at)
			VALUES (
				$1, $2, 'TEXT', $3,
				GREATEST(
					clock_timestamp(),
					COALESCE((SELECT MAX(created_at) + interval '1 microsecond' FROM messages WHERE conversation_id = $1), clock_timestamp())
				)
			)
			RETURNING id, conversation_id, author_id, type::text AS type, body, created_at, edited_at
		), touched AS (
			UPDATE conversations
			SET last_message_at = (SELECT created_at FROM created)
			WHERE id = $1
		)
		SELECT created.id, created.conversation_id, COALESCE(created.author_id::text, ''),
			COALESCE(u.first_name, ''), COALESCE(u.last_name, ''), created.type,
			COALESCE(created.body, ''), created.created_at, created.edited_at, NULL::timestamptz
		FROM created
		LEFT JOIN users u ON u.id = created.author_id`, conversationID, authorID, body).Scan(
		&m.ID, &m.ConversationID, &m.AuthorID, &m.AuthorFirstName, &m.AuthorLastName,
		&m.Type, &m.Body, &m.CreatedAt, &m.EditedAt, &m.ReadAt)
	if err != nil {
		return Message{}, notFound(err)
	}
	return m, nil
}

const messageColumns = `m.id, m.conversation_id, COALESCE(m.author_id::text, ''),
	COALESCE(u.first_name, ''), COALESCE(u.last_name, ''), m.type::text, COALESCE(m.body, ''),
	m.created_at, m.edited_at,
	(SELECT mr.read_at FROM message_receipts mr
		WHERE mr.message_id = m.id AND mr.user_id IS DISTINCT FROM m.author_id
		ORDER BY mr.read_at DESC NULLS LAST LIMIT 1)`

func (m *Message) scanTargets(extra ...any) []any {
	return append([]any{&m.ID, &m.ConversationID, &m.AuthorID, &m.AuthorFirstName, &m.AuthorLastName,
		&m.Type, &m.Body, &m.CreatedAt, &m.EditedAt, &m.ReadAt}, extra...)
}

func (p *Postgres) ConversationMessages(ctx context.Context, userID, conversationID, afterID string, limit, offset int) ([]Message, int, error) {
	var member bool
	if err := p.conn(ctx).QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM conversation_participants
		WHERE conversation_id = $1 AND user_id = $2)`, conversationID, userID).Scan(&member); err != nil {
		return nil, 0, err
	}
	if !member {
		return nil, 0, ErrNotFound
	}
	if afterID != "" {
		var anchorAt time.Time
		err := p.conn(ctx).QueryRow(ctx, `SELECT created_at FROM messages
			WHERE id = $2 AND conversation_id = $1 AND deleted_at IS NULL`, conversationID, afterID).Scan(&anchorAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, 0, ErrInvalidCursor
		}
		if err != nil {
			return nil, 0, err
		}
		rows, err := p.conn(ctx).Query(ctx, `SELECT `+messageColumns+`, count(*) OVER ()
			FROM messages m
			LEFT JOIN users u ON u.id = m.author_id
			WHERE m.conversation_id = $1 AND m.deleted_at IS NULL
				AND (m.created_at, m.id) > ($2, $3::uuid)
			ORDER BY m.created_at ASC, m.id ASC
			LIMIT $4`, conversationID, anchorAt, afterID, limit)
		if err != nil {
			return nil, 0, err
		}
		total := 0
		items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Message, error) {
			var m Message
			err := row.Scan(m.scanTargets(&total)...)
			return m, err
		})
		return items, total, err
	}

	rows, err := p.conn(ctx).Query(ctx, `SELECT `+messageColumns+`, count(*) OVER ()
		FROM messages m
		LEFT JOIN users u ON u.id = m.author_id
		WHERE m.conversation_id = $1 AND m.deleted_at IS NULL
		ORDER BY m.created_at ASC, m.id ASC
		LIMIT $2 OFFSET $3`, conversationID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	total := 0
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Message, error) {
		var m Message
		err := row.Scan(m.scanTargets(&total)...)
		return m, err
	})
	if err != nil {
		return nil, 0, err
	}
	if len(items) == 0 && offset > 0 {
		err = p.conn(ctx).QueryRow(ctx, `SELECT count(*) FROM messages
			WHERE conversation_id = $1 AND deleted_at IS NULL`, conversationID).Scan(&total)
	}
	return items, total, err
}

func (p *Postgres) MarkConversationRead(ctx context.Context, userID, conversationID string) (int, error) {
	var member bool
	if err := p.conn(ctx).QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM conversation_participants
		WHERE conversation_id = $1 AND user_id = $2)`, conversationID, userID).Scan(&member); err != nil {
		return 0, err
	}
	if !member {
		return 0, ErrNotFound
	}
	tag, err := p.conn(ctx).Exec(ctx, `INSERT INTO message_receipts (message_id, user_id, delivered_at, read_at)
		SELECT m.id, $2, now(), now()
		FROM messages m
		LEFT JOIN message_receipts mr ON mr.message_id = m.id AND mr.user_id = $2
		WHERE m.conversation_id = $1 AND m.deleted_at IS NULL
			AND m.author_id IS NOT NULL AND m.author_id <> $2
			AND mr.read_at IS NULL
		ON CONFLICT (message_id, user_id) DO UPDATE
			SET delivered_at = COALESCE(message_receipts.delivered_at, EXCLUDED.delivered_at),
				read_at = COALESCE(message_receipts.read_at, EXCLUDED.read_at)`, conversationID, userID)
	if err != nil {
		return 0, err
	}
	if _, err := p.conn(ctx).Exec(ctx, `UPDATE notifications SET read_at = now()
		WHERE recipient_id = $2 AND conversation_id = $1 AND read_at IS NULL`, conversationID, userID); err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// CreateNotification inserts a notification once per (recipient_id, event_key).
func (p *Postgres) CreateNotification(ctx context.Context, n NewNotification) (bool, error) {
	tag, err := p.conn(ctx).Exec(ctx, `INSERT INTO notifications (
			recipient_id, actor_id, event_type, event_key, title, body,
			exchange_request_id, exchange_id, conversation_id, message_id, session_id, review_id
		)
		VALUES (
			$1, NULLIF($2, '')::uuid, $3::notification_event_type, $4, $5, NULLIF($6, ''),
			NULLIF($7, '')::uuid, NULLIF($8, '')::uuid, NULLIF($9, '')::uuid, NULLIF($10, '')::uuid, NULLIF($11, '')::uuid,
			NULLIF($12, '')::uuid
		)
		ON CONFLICT (recipient_id, event_key) DO NOTHING`,
		n.RecipientID, n.ActorID, n.EventType, n.EventKey, n.Title, n.Body,
		n.RequestID, n.ExchangeID, n.ConversationID, n.MessageID, n.SessionID, n.ReviewID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

const notificationColumns = `n.id, n.recipient_id, COALESCE(n.actor_id::text, ''),
	COALESCE(a.first_name, ''), COALESCE(a.last_name, ''), n.event_type::text, n.event_key,
	n.title, COALESCE(n.body, ''), COALESCE(n.exchange_request_id::text, ''),
	COALESCE(n.exchange_id::text, ''), COALESCE(n.conversation_id::text, ''),
	COALESCE(n.message_id::text, ''), COALESCE(n.session_id::text, ''), COALESCE(n.review_id::text, ''),
	n.created_at, n.read_at`

const notificationFrom = `
	FROM notifications n
	LEFT JOIN users a ON a.id = n.actor_id`

func (n *Notification) scanTargets(extra ...any) []any {
	return append([]any{&n.ID, &n.RecipientID, &n.ActorID, &n.ActorFirstName, &n.ActorLastName,
		&n.EventType, &n.EventKey, &n.Title, &n.Body, &n.RequestID, &n.ExchangeID,
		&n.ConversationID, &n.MessageID, &n.SessionID, &n.ReviewID, &n.CreatedAt, &n.ReadAt}, extra...)
}

func (p *Postgres) UserNotifications(ctx context.Context, userID string, unreadOnly bool, limit, offset int) ([]Notification, int, int, error) {
	where := ` WHERE n.recipient_id = $1 AND (NOT $2 OR n.read_at IS NULL)`
	rows, err := p.conn(ctx).Query(ctx, `SELECT `+notificationColumns+`, count(*) OVER ()`+notificationFrom+where+`
		ORDER BY n.created_at DESC, n.id DESC
		LIMIT $3 OFFSET $4`, userID, unreadOnly, limit, offset)
	if err != nil {
		return nil, 0, 0, err
	}
	total := 0
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Notification, error) {
		var n Notification
		err := row.Scan(n.scanTargets(&total)...)
		return n, err
	})
	if err != nil {
		return nil, 0, 0, err
	}
	if len(items) == 0 && offset > 0 {
		if err := p.conn(ctx).QueryRow(ctx, `SELECT count(*) FROM notifications n`+where,
			userID, unreadOnly).Scan(&total); err != nil {
			return nil, 0, 0, err
		}
	}
	var unreadCount int
	if unreadOnly && (len(items) > 0 || offset == 0) {
		unreadCount = total
	} else {
		if err := p.conn(ctx).QueryRow(ctx, `SELECT count(*) FROM notifications
			WHERE recipient_id = $1 AND read_at IS NULL`, userID).Scan(&unreadCount); err != nil {
			return nil, 0, 0, err
		}
	}
	return items, total, unreadCount, nil
}

func (p *Postgres) MarkNotificationRead(ctx context.Context, userID, notificationID string) (Notification, int, error) {
	var n Notification
	err := p.conn(ctx).QueryRow(ctx, `WITH updated AS (
			UPDATE notifications
			SET read_at = COALESCE(read_at, now())
			WHERE id = $2 AND recipient_id = $1
			RETURNING *
		)
		SELECT `+notificationColumns+`
		FROM updated n
		LEFT JOIN users a ON a.id = n.actor_id`, userID, notificationID).Scan(n.scanTargets()...)
	if err != nil {
		return Notification{}, 0, notFound(err)
	}
	if n.MessageID != "" {
		if _, err := p.conn(ctx).Exec(ctx, `INSERT INTO message_receipts (message_id, user_id, delivered_at, read_at)
			VALUES ($1, $2, $3, $3)
			ON CONFLICT (message_id, user_id) DO UPDATE
				SET delivered_at = COALESCE(message_receipts.delivered_at, EXCLUDED.delivered_at),
					read_at = COALESCE(message_receipts.read_at, EXCLUDED.read_at)`,
			n.MessageID, userID, *n.ReadAt); err != nil {
			return Notification{}, 0, err
		}
	}
	var unreadCount int
	if err := p.conn(ctx).QueryRow(ctx, `SELECT count(*) FROM notifications
		WHERE recipient_id = $1 AND read_at IS NULL`, userID).Scan(&unreadCount); err != nil {
		return Notification{}, 0, err
	}
	return n, unreadCount, nil
}

func (p *Postgres) MarkAllNotificationsRead(ctx context.Context, userID string) (int, error) {
	tag, err := p.conn(ctx).Exec(ctx, `WITH updated AS (
			UPDATE notifications
			SET read_at = now()
			WHERE recipient_id = $1 AND read_at IS NULL
			RETURNING message_id, read_at
		), receipts AS (
			INSERT INTO message_receipts (message_id, user_id, delivered_at, read_at)
			SELECT message_id, $1, read_at, read_at
			FROM updated
			WHERE message_id IS NOT NULL
			ON CONFLICT (message_id, user_id) DO UPDATE
				SET delivered_at = COALESCE(message_receipts.delivered_at, EXCLUDED.delivered_at),
					read_at = COALESCE(message_receipts.read_at, EXCLUDED.read_at)
		)
		SELECT 1 FROM updated`, userID)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}
