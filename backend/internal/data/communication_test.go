package data

import (
	"context"
	"errors"
	"slices"
	"testing"

	"skillswap/backend/internal/expect"
)

const (
	demoTarasToOlhaConversation = "72000000-0000-0000-0000-000000000001"
	demoTarasToOlhaMessage      = "73000000-0000-0000-0000-000000000001"
	demoOlhaRequestNotification = "74000000-0000-0000-0000-000000000001"
)

// Skipped unless TEST_DATABASE_URL points to a migrated, demo-seeded database.
func TestCommunicationOnDemoData(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	conv, err := store.ConversationByID(ctx, demoOlha, demoTarasToOlhaConversation)
	if err != nil {
		t.Fatal(err)
	}
	expect.Equal(t, "conversation", conv.ID, demoTarasToOlhaConversation)
	expect.Equal(t, "conversation request", conv.RequestID, demoTarasToOlha)
	expect.Equal(t, "requester", conv.Requester.UserID, demoTaras)
	expect.Equal(t, "recipient", conv.Recipient.UserID, demoOlha)
	expect.Equal(t, "at least 1 unread message", conv.UnreadCount >= 1, true)
	expect.Equal(t, "last message is set", conv.LastMessage != nil, true)
	expect.Equal(t, "last message", conv.LastMessage.ID, demoTarasToOlhaMessage)
	_, err = store.ConversationByID(ctx, demoAndrii, demoTarasToOlhaConversation)
	expect.Error(t, "stranger conversation access", err, ErrNotFound)

	msgs, total, err := store.ConversationMessages(ctx, demoOlha, demoTarasToOlhaConversation, "", 20, 0)
	expect.NoError(t, "load demo messages", err)
	expect.Equal(t, "at least 1 message in total", total >= 1, true)
	expect.Equal(t, "at least 1 message on the page", len(msgs) >= 1, true)
	expect.Equal(t, "first message", msgs[0].ID, demoTarasToOlhaMessage)

	notifs, total, unread, err := store.UserNotifications(ctx, demoOlha, true, 20, 0)
	expect.NoError(t, "load demo notifications", err)
	expect.Equal(t, "at least 2 notifications in total", total >= 2, true)
	expect.Equal(t, "at least 2 unread notifications", unread >= 2, true)
	expect.Equal(t, "at least 2 notifications on the page", len(notifs) >= 2, true)
}

// Runs inside a transaction that is rolled back.
// Skipped unless TEST_DATABASE_URL points to a migrated, demo-seeded database.
func TestConversationsAndMessagesAgainstPostgres(t *testing.T) {
	store := openTestStore(t)
	rollback := errors.New("roll back test data")
	err := store.WithinTx(context.Background(), func(ctx context.Context) error {
		reqID, err := store.CreateRequest(ctx, NewRequest{
			RequesterID:          demoOlha,
			RecipientID:          demoAndrii,
			TeachSkillID:         guitarSkill,
			LearnSkillID:         photoshop,
			Format:               FormatOnline,
			TeachSessions:        2,
			TeachDurationMinutes: 60,
			LearnSessions:        2,
			LearnDurationMinutes: 60,
			Message:              "Привіт!",
		})
		if err != nil {
			t.Fatal(err)
		}
		convID, err := store.EnsureRequestConversation(ctx, reqID)
		expect.NoError(t, "EnsureRequestConversation", err)
		expect.Equal(t, "conversation ID is set", convID != "", true)
		// Calling EnsureRequestConversation again returns the same conversation ID.
		again, err := store.EnsureRequestConversation(ctx, reqID)
		expect.NoError(t, "EnsureRequestConversation again", err)
		expect.Equal(t, "conversation ID on the second call", again, convID)

		conv, err := store.ConversationByID(ctx, demoOlha, convID)
		if err != nil {
			t.Fatal(err)
		}
		expect.Equal(t, "conversation type", conv.Type, ConversationRequest)
		expect.Equal(t, "conversation request", conv.RequestID, reqID)
		expect.Equal(t, "conversation exchange", conv.ExchangeID, "")
		expect.Equal(t, "request status", conv.RequestStatus, RequestPending)
		expect.Equal(t, "Olha can send", conv.CanSend, true)
		expect.Equal(t, "Olha's unread messages", conv.UnreadCount, 0)

		m1, err := store.CreateMessage(ctx, convID, demoOlha, "Перше повідомлення в чаті запиту")
		if err != nil {
			t.Fatal(err)
		}
		m2, err := store.CreateMessage(ctx, convID, demoAndrii, "Друге повідомлення")
		if err != nil {
			t.Fatal(err)
		}

		// Andrii has 1 unread message (m1); Olha has 1 unread message (m2).
		andriiConv, err := store.ConversationByID(ctx, demoAndrii, convID)
		expect.NoError(t, "load Andrii's conversation", err)
		expect.Equal(t, "Andrii's unread messages", andriiConv.UnreadCount, 1)
		expect.Equal(t, "Andrii's last message is set", andriiConv.LastMessage != nil, true)
		expect.Equal(t, "Andrii's last message", andriiConv.LastMessage.ID, m2.ID)

		// Polling after m1 returns only m2.
		after, total, err := store.ConversationMessages(ctx, demoAndrii, convID, m1.ID, 20, 0)
		expect.NoError(t, "load messages after m1", err)
		expect.Equal(t, "total after m1", total, 1)
		expect.Equal(t, "messages after m1", len(after), 1)
		expect.Equal(t, "message after m1", after[0].ID, m2.ID)
		_, _, err = store.ConversationMessages(ctx, demoAndrii, convID, "73000000-0000-0000-0000-000000000999", 20, 0)
		expect.Error(t, "unknown cursor", err, ErrInvalidCursor)
		_, _, err = store.ConversationMessages(ctx, demoMarko, convID, "", 20, 0)
		expect.Error(t, "stranger messages", err, ErrNotFound)

		// Andrii marks the conversation read.
		readCount, err := store.MarkConversationRead(ctx, demoAndrii, convID)
		expect.NoError(t, "MarkConversationRead", err)
		expect.Equal(t, "messages marked read", readCount, 1)
		all, total, err := store.ConversationMessages(ctx, demoOlha, convID, "", 20, 0)
		expect.NoError(t, "load messages after read", err)
		expect.Equal(t, "total after read", total, 2)
		expect.Equal(t, "first message is read", all[0].ReadAt != nil, true)
		expect.Equal(t, "second message is read", all[1].ReadAt != nil, false)

		// Accept the request and link the existing conversation to the exchange.
		if err := store.UpdateRequestStatus(ctx, reqID, RequestAccepted, demoAndrii); err != nil {
			t.Fatal(err)
		}
		exchangeID, err := store.CreateExchange(ctx, reqID, demoAndrii)
		if err != nil {
			t.Fatal(err)
		}
		linkedID, err := store.EnsureExchangeConversation(ctx, exchangeID)
		expect.NoError(t, "EnsureExchangeConversation", err)
		expect.Equal(t, "exchange conversation ID", linkedID, convID)
		linked, err := store.ConversationByID(ctx, demoOlha, convID)
		expect.NoError(t, "load the linked conversation", err)
		expect.Equal(t, "linked conversation type", linked.Type, ConversationExchange)
		expect.Equal(t, "linked conversation exchange", linked.ExchangeID, exchangeID)
		expect.Equal(t, "exchange status", linked.ExchangeStatus, ExchangeActive)
		expect.Equal(t, "linked last message is set", linked.LastMessage != nil, true)
		expect.Equal(t, "linked last message", linked.LastMessage.ID, m2.ID)

		// UserConversations lists the most active conversation first.
		list, listTotal, err := store.UserConversations(ctx, demoOlha, 20, 0)
		expect.NoError(t, "UserConversations", err)
		expect.Equal(t, "at least 2 conversations in total", listTotal >= 2, true)
		expect.Equal(t, "first conversation", list[0].ID, convID)

		// Blocking disables CanSend in LockConversation while keeping the conversation readable.
		if _, err := store.conn(ctx).Exec(ctx, `INSERT INTO user_blocks (blocker_id, blocked_id) VALUES ($1, $2)`,
			demoAndrii, demoOlha); err != nil {
			t.Fatal(err)
		}
		locked, err := store.LockConversation(ctx, demoOlha, convID)
		expect.NoError(t, "LockConversation after block", err)
		expect.Equal(t, "Olha can send after block", locked.CanSend, false)
		return rollback
	})
	expect.Error(t, "WithinTx", err, rollback)
}

// Runs inside a transaction that is rolled back.
// Skipped unless TEST_DATABASE_URL points to a migrated, demo-seeded database.
func TestNotificationsAgainstPostgres(t *testing.T) {
	store := openTestStore(t)
	rollback := errors.New("roll back test data")
	err := store.WithinTx(context.Background(), func(ctx context.Context) error {
		reqID, err := store.CreateRequest(ctx, NewRequest{
			RequesterID:          demoOlha,
			RecipientID:          demoAndrii,
			TeachSkillID:         guitarSkill,
			LearnSkillID:         photoshop,
			Format:               FormatOnline,
			TeachSessions:        1,
			TeachDurationMinutes: 60,
			LearnSessions:        1,
			LearnDurationMinutes: 60,
		})
		if err != nil {
			t.Fatal(err)
		}
		convID, err := store.EnsureRequestConversation(ctx, reqID)
		if err != nil {
			t.Fatal(err)
		}
		in := NewNotification{
			RecipientID:    demoAndrii,
			ActorID:        demoOlha,
			EventType:      EventExchangeRequest,
			EventKey:       "exchange-request:" + reqID,
			Title:          "Новий запит на обмін",
			Body:           "Ольга Гнатюк: Гітара ↔ Photoshop.",
			RequestID:      reqID,
			ConversationID: convID,
		}
		created, err := store.CreateNotification(ctx, in)
		expect.NoError(t, "first CreateNotification", err)
		expect.Equal(t, "first notification created", created, true)
		// Same (recipient_id, event_key) is ignored without error.
		duplicate, err := store.CreateNotification(ctx, in)
		expect.NoError(t, "duplicate CreateNotification", err)
		expect.Equal(t, "duplicate notification created", duplicate, false)

		items, total, unread, err := store.UserNotifications(ctx, demoAndrii, true, 20, 0)
		expect.NoError(t, "load Andrii's notifications", err)
		expect.Equal(t, "Andrii's notifications in total", total, 1)
		expect.Equal(t, "Andrii's unread notifications", unread, 1)
		expect.Equal(t, "Andrii's notifications on the page", len(items), 1)
		notif := items[0]
		expect.Equal(t, "event type", notif.EventType, EventExchangeRequest)
		expect.Equal(t, "actor", notif.ActorID, demoOlha)
		expect.Equal(t, "actor's first name", notif.ActorFirstName, "Ольга")
		expect.Equal(t, "notification request", notif.RequestID, reqID)
		expect.Equal(t, "notification conversation", notif.ConversationID, convID)
		expect.Equal(t, "notification is read", notif.ReadAt != nil, false)

		_, _, err = store.MarkNotificationRead(ctx, demoOlha, notif.ID)
		expect.Error(t, "other user MarkNotificationRead", err, ErrNotFound)
		readNotif, unreadAfter, err := store.MarkNotificationRead(ctx, demoAndrii, notif.ID)
		expect.NoError(t, "MarkNotificationRead", err)
		expect.Equal(t, "notification is read after MarkNotificationRead", readNotif.ReadAt != nil, true)
		expect.Equal(t, "Andrii's unread notifications after MarkNotificationRead", unreadAfter, 0)

		// MarkAllNotificationsRead on Olha's seeded unread notifications.
		marked, err := store.MarkAllNotificationsRead(ctx, demoOlha)
		expect.NoError(t, "MarkAllNotificationsRead", err)
		expect.Equal(t, "at least 2 notifications marked read", marked >= 2, true)
		_, _, olhaUnread, err := store.UserNotifications(ctx, demoOlha, false, 20, 0)
		expect.NoError(t, "load Olha's notifications", err)
		expect.Equal(t, "Olha's unread notifications after MarkAllNotificationsRead", olhaUnread, 0)
		// Seeded notification IDs are still present in the full list.
		allOlha, _, _, err := store.UserNotifications(ctx, demoOlha, false, 20, 0)
		expect.NoError(t, "load all of Olha's notifications", err)
		expect.Equal(t, "seeded notification is in Olha's list",
			slices.ContainsFunc(allOlha, func(n Notification) bool { return n.ID == demoOlhaRequestNotification }), true)
		return rollback
	})
	expect.Error(t, "WithinTx", err, rollback)
}
