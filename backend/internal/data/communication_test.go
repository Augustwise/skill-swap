package data

import (
	"context"
	"errors"
	"slices"
	"testing"
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
	if conv.ID != demoTarasToOlhaConversation || conv.RequestID != demoTarasToOlha ||
		conv.Requester.UserID != demoTaras || conv.Recipient.UserID != demoOlha ||
		conv.UnreadCount < 1 || conv.LastMessage == nil || conv.LastMessage.ID != demoTarasToOlhaMessage {
		t.Fatalf("demo conversation = %+v", conv)
	}
	if _, err := store.ConversationByID(ctx, demoAndrii, demoTarasToOlhaConversation); !errors.Is(err, ErrNotFound) {
		t.Fatalf("stranger conversation access: err = %v", err)
	}

	msgs, total, err := store.ConversationMessages(ctx, demoOlha, demoTarasToOlhaConversation, "", 20, 0)
	if err != nil || total < 1 || len(msgs) < 1 || msgs[0].ID != demoTarasToOlhaMessage {
		t.Fatalf("demo messages = %+v, total %d, err %v", msgs, total, err)
	}

	notifs, total, unread, err := store.UserNotifications(ctx, demoOlha, true, 20, 0)
	if err != nil || total < 2 || unread < 2 || len(notifs) < 2 {
		t.Fatalf("demo notifications = %+v, total %d, unread %d, err %v", notifs, total, unread, err)
	}
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
		if err != nil || convID == "" {
			t.Fatalf("EnsureRequestConversation = %q, err %v", convID, err)
		}
		// Calling EnsureRequestConversation again returns the same conversation ID.
		if again, err := store.EnsureRequestConversation(ctx, reqID); err != nil || again != convID {
			t.Fatalf("EnsureRequestConversation again = %q, err %v", again, err)
		}

		conv, err := store.ConversationByID(ctx, demoOlha, convID)
		if err != nil {
			t.Fatal(err)
		}
		if conv.Type != ConversationRequest || conv.RequestID != reqID || conv.ExchangeID != "" ||
			conv.RequestStatus != RequestPending || !conv.CanSend || conv.UnreadCount != 0 {
			t.Fatalf("conversation = %+v", conv)
		}

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
		if err != nil || andriiConv.UnreadCount != 1 || andriiConv.LastMessage == nil || andriiConv.LastMessage.ID != m2.ID {
			t.Fatalf("Andrii conversation = %+v, err %v", andriiConv, err)
		}

		// Polling after m1 returns only m2.
		after, total, err := store.ConversationMessages(ctx, demoAndrii, convID, m1.ID, 20, 0)
		if err != nil || total != 1 || len(after) != 1 || after[0].ID != m2.ID {
			t.Fatalf("messages after m1 = %+v, total %d, err %v", after, total, err)
		}
		if _, _, err := store.ConversationMessages(ctx, demoAndrii, convID, "73000000-0000-0000-0000-000000000999", 20, 0); !errors.Is(err, ErrInvalidCursor) {
			t.Fatalf("unknown cursor: err = %v", err)
		}
		if _, _, err := store.ConversationMessages(ctx, demoMarko, convID, "", 20, 0); !errors.Is(err, ErrNotFound) {
			t.Fatalf("stranger messages: err = %v", err)
		}

		// Andrii marks the conversation read.
		readCount, err := store.MarkConversationRead(ctx, demoAndrii, convID)
		if err != nil || readCount != 1 {
			t.Fatalf("MarkConversationRead = %d, err %v", readCount, err)
		}
		all, total, err := store.ConversationMessages(ctx, demoOlha, convID, "", 20, 0)
		if err != nil || total != 2 || all[0].ReadAt == nil || all[1].ReadAt != nil {
			t.Fatalf("messages after read = %+v, total %d, err %v", all, total, err)
		}

		// Accept the request and link the existing conversation to the exchange.
		if err := store.UpdateRequestStatus(ctx, reqID, RequestAccepted, demoAndrii); err != nil {
			t.Fatal(err)
		}
		exchangeID, err := store.CreateExchange(ctx, reqID, demoAndrii)
		if err != nil {
			t.Fatal(err)
		}
		linkedID, err := store.EnsureExchangeConversation(ctx, exchangeID)
		if err != nil || linkedID != convID {
			t.Fatalf("EnsureExchangeConversation = %q, want %q, err %v", linkedID, convID, err)
		}
		linked, err := store.ConversationByID(ctx, demoOlha, convID)
		if err != nil || linked.Type != ConversationExchange || linked.ExchangeID != exchangeID ||
			linked.ExchangeStatus != ExchangeActive || linked.LastMessage == nil || linked.LastMessage.ID != m2.ID {
			t.Fatalf("linked conversation = %+v, err %v", linked, err)
		}

		// UserConversations lists the most active conversation first.
		list, listTotal, err := store.UserConversations(ctx, demoOlha, 20, 0)
		if err != nil || listTotal < 2 || list[0].ID != convID {
			t.Fatalf("UserConversations = %+v, total %d, err %v", list, listTotal, err)
		}

		// Blocking disables CanSend in LockConversation while keeping the conversation readable.
		if _, err := store.conn(ctx).Exec(ctx, `INSERT INTO user_blocks (blocker_id, blocked_id) VALUES ($1, $2)`,
			demoAndrii, demoOlha); err != nil {
			t.Fatal(err)
		}
		locked, err := store.LockConversation(ctx, demoOlha, convID)
		if err != nil || locked.CanSend {
			t.Fatalf("LockConversation after block = %+v, err %v", locked, err)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("WithinTx err = %v", err)
	}
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
		if err != nil || !created {
			t.Fatalf("first CreateNotification = %v, err %v", created, err)
		}
		// Same (recipient_id, event_key) is ignored without error.
		duplicate, err := store.CreateNotification(ctx, in)
		if err != nil || duplicate {
			t.Fatalf("duplicate CreateNotification = %v, err %v", duplicate, err)
		}

		items, total, unread, err := store.UserNotifications(ctx, demoAndrii, true, 20, 0)
		if err != nil || total != 1 || unread != 1 || len(items) != 1 {
			t.Fatalf("Andrii notifications = %+v, total %d, unread %d, err %v", items, total, unread, err)
		}
		notif := items[0]
		if notif.EventType != EventExchangeRequest || notif.ActorID != demoOlha ||
			notif.ActorFirstName != "Ольга" || notif.RequestID != reqID || notif.ConversationID != convID || notif.ReadAt != nil {
			t.Fatalf("notification = %+v", notif)
		}

		if _, _, err := store.MarkNotificationRead(ctx, demoOlha, notif.ID); !errors.Is(err, ErrNotFound) {
			t.Fatalf("other user MarkNotificationRead: err = %v", err)
		}
		readNotif, unreadAfter, err := store.MarkNotificationRead(ctx, demoAndrii, notif.ID)
		if err != nil || readNotif.ReadAt == nil || unreadAfter != 0 {
			t.Fatalf("MarkNotificationRead = %+v, unread %d, err %v", readNotif, unreadAfter, err)
		}

		// MarkAllNotificationsRead on Olha's seeded unread notifications.
		marked, err := store.MarkAllNotificationsRead(ctx, demoOlha)
		if err != nil || marked < 2 {
			t.Fatalf("MarkAllNotificationsRead = %d, err %v", marked, err)
		}
		_, _, olhaUnread, err := store.UserNotifications(ctx, demoOlha, false, 20, 0)
		if err != nil || olhaUnread != 0 {
			t.Fatalf("Olha unread after MarkAllNotificationsRead = %d, err %v", olhaUnread, err)
		}
		// Seeded notification IDs are still present in the full list.
		allOlha, _, _, err := store.UserNotifications(ctx, demoOlha, false, 20, 0)
		if err != nil || !slices.ContainsFunc(allOlha, func(n Notification) bool { return n.ID == demoOlhaRequestNotification }) {
			t.Fatalf("Olha all notifications = %+v, err %v", allOlha, err)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("WithinTx err = %v", err)
	}
}
