package communication

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"skillswap/backend/internal/data"
	"skillswap/backend/internal/data/datatest"
	"skillswap/backend/internal/profile"
)

type fixture struct {
	service  *Service
	store    *datatest.Memory
	olha     string
	andrii   string
	stranger string
}

func (f *fixture) student(t *testing.T, email, firstName, lastName, teach, learn string) string {
	t.Helper()
	ctx := context.Background()
	id, err := f.store.CreateUser(ctx, data.NewUser{
		UniversityID: datatest.DemoUniversityID,
		Email:        email,
		FirstName:    firstName,
		LastName:     lastName,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.store.MarkEmailVerified(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := f.store.UpdateProfile(ctx, id, data.ProfileUpdate{
		FirstName: firstName,
		LastName:  lastName,
		City:      "Київ",
		Formats:   []string{data.FormatOnline, data.FormatOffline},
	}); err != nil {
		t.Fatal(err)
	}
	if err := f.store.AddUserSkill(ctx, data.TeachingList, id, teach, "ADVANCED"); err != nil {
		t.Fatal(err)
	}
	if err := f.store.AddUserSkill(ctx, data.LearningList, id, learn, "BEGINNER"); err != nil {
		t.Fatal(err)
	}
	return id
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	store := datatest.NewMemory()
	f := &fixture{
		service: NewService(store, store, store),
		store:   store,
	}
	f.olha = f.student(t, "olha@students.example.test", "Ольга", "Гнатюк", datatest.GuitarSkillID, datatest.PhotoshopSkillID)
	f.andrii = f.student(t, "andrii@students.example.test", "Андрій", "Коваль", datatest.PhotoshopSkillID, datatest.GuitarSkillID)
	f.stranger = f.student(t, "marko@students.example.test", "Марко", "Савчук", datatest.PhotoshopSkillID, datatest.GuitarSkillID)
	return f
}

func (f *fixture) createRequest(t *testing.T) data.ExchangeRequest {
	t.Helper()
	ctx := context.Background()
	id, err := f.store.CreateRequest(ctx, data.NewRequest{
		RequesterID:          f.olha,
		RecipientID:          f.andrii,
		TeachSkillID:         datatest.GuitarSkillID,
		LearnSkillID:         datatest.PhotoshopSkillID,
		Format:               data.FormatOnline,
		TeachSessions:        2,
		TeachDurationMinutes: 60,
		LearnSessions:        2,
		LearnDurationMinutes: 60,
		Message:              "Привіт!",
	})
	if err != nil {
		t.Fatal(err)
	}
	r, err := f.store.RequestByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.service.RecordRequestCreated(ctx, r); err != nil {
		t.Fatal(err)
	}
	return r
}

func fieldErrors(t *testing.T, err error) map[string]string {
	t.Helper()
	var invalid *profile.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
	return invalid.Fields
}

func TestConversationsAndExchangeLinking(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	req := f.createRequest(t)

	byRequest, err := f.service.RequestConversation(ctx, f.olha, strings.ToUpper(req.ID))
	if err != nil {
		t.Fatal(err)
	}
	if byRequest.ID == "" || byRequest.Type != data.ConversationRequest || byRequest.RequestID != req.ID ||
		byRequest.ExchangeID != "" || byRequest.RequestStatus != data.RequestPending || !byRequest.CanSend ||
		byRequest.UnreadCount != 0 || byRequest.LastMessage != nil {
		t.Fatalf("conversation = %+v", byRequest)
	}

	msg, err := f.service.SendMessage(ctx, f.olha, byRequest.ID, "Обговоримо деталі перед прийняттям.")
	if err != nil {
		t.Fatal(err)
	}

	// Accept the request and link the conversation to the created exchange.
	if err := f.store.UpdateRequestStatus(ctx, req.ID, data.RequestAccepted, f.andrii); err != nil {
		t.Fatal(err)
	}
	exchangeID, err := f.store.CreateExchange(ctx, req.ID, f.andrii)
	if err != nil {
		t.Fatal(err)
	}
	acceptedReq, err := f.store.RequestByID(ctx, req.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.service.RecordRequestAccepted(ctx, acceptedReq, exchangeID); err != nil {
		t.Fatal(err)
	}

	byExchange, err := f.service.ExchangeConversation(ctx, f.andrii, strings.ToUpper(exchangeID))
	if err != nil {
		t.Fatal(err)
	}
	if byExchange.ID != byRequest.ID || byExchange.Type != data.ConversationExchange ||
		byExchange.RequestID != req.ID || byExchange.ExchangeID != exchangeID ||
		byExchange.RequestStatus != data.RequestAccepted || byExchange.ExchangeStatus != data.ExchangeActive ||
		byExchange.UnreadCount != 1 || byExchange.LastMessage == nil || byExchange.LastMessage.ID != msg.ID {
		t.Fatalf("linked conversation = %+v", byExchange)
	}

	for _, user := range []string{f.olha, f.andrii} {
		conv, err := f.service.Conversation(ctx, user, strings.ToUpper(byRequest.ID))
		if err != nil || conv.ID != byRequest.ID {
			t.Fatalf("Conversation(%s) = %+v, err %v", user, conv, err)
		}
		list, err := f.service.Conversations(ctx, user, 1)
		if err != nil || list.Total != 1 || len(list.Items) != 1 || list.Items[0].ID != byRequest.ID {
			t.Fatalf("Conversations(%s) = %+v, err %v", user, list, err)
		}
	}

	// Access checks for a third party and invalid IDs.
	for _, id := range []string{byRequest.ID, "72000000-0000-0000-0000-000000000999", "not-a-uuid"} {
		if _, err := f.service.Conversation(ctx, f.stranger, id); !errors.Is(err, ErrConversationNotFound) {
			t.Fatalf("Conversation(%s): err = %v", id, err)
		}
	}
	for _, id := range []string{req.ID, "70000000-0000-0000-0000-000000000999", "not-a-uuid"} {
		if _, err := f.service.RequestConversation(ctx, f.stranger, id); !errors.Is(err, ErrRequestNotFound) {
			t.Fatalf("RequestConversation(%s): err = %v", id, err)
		}
	}
	for _, id := range []string{exchangeID, "80000000-0000-0000-0000-000000000999", "not-a-uuid"} {
		if _, err := f.service.ExchangeConversation(ctx, f.stranger, id); !errors.Is(err, ErrExchangeNotFound) {
			t.Fatalf("ExchangeConversation(%s): err = %v", id, err)
		}
	}
}

func TestSendMessageValidationAndBlocking(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	req := f.createRequest(t)
	conv, err := f.service.RequestConversation(ctx, f.olha, req.ID)
	if err != nil {
		t.Fatal(err)
	}

	sent, err := f.service.SendMessage(ctx, f.olha, strings.ToUpper(conv.ID),
		"  Привіт! Ось посилання на зустріч:\nhttps://meet.example.test/skill-swap  ")
	if err != nil {
		t.Fatal(err)
	}
	if sent.ID == "" || sent.ConversationID != conv.ID || sent.AuthorID != f.olha ||
		sent.AuthorFirstName != "Ольга" || sent.Type != data.MessageText ||
		sent.Body != "Привіт! Ось посилання на зустріч:\nhttps://meet.example.test/skill-swap" {
		t.Fatalf("message = %+v", sent)
	}

	// Exactly 2000 Cyrillic characters is allowed.
	maxBody := strings.Repeat("ї", MaxMessageLength)
	if _, err := f.service.SendMessage(ctx, f.andrii, conv.ID, maxBody); err != nil {
		t.Fatalf("2000 characters: err = %v", err)
	}

	for _, test := range []struct {
		name string
		body string
	}{
		{"empty", ""},
		{"spaces only", "   \n\t  "},
		{"too long", strings.Repeat("ї", MaxMessageLength+1)},
		{"control character", "Привіт\x00"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := f.service.SendMessage(ctx, f.olha, conv.ID, test.body)
			if fields := fieldErrors(t, err); fields["body"] == "" || len(fields) != 1 {
				t.Fatalf("fields = %v, want only body", fields)
			}
		})
	}

	for _, test := range []struct{ name, user, id string }{
		{"not a participant", f.stranger, conv.ID},
		{"unknown conversation", f.olha, "72000000-0000-0000-0000-000000000999"},
		{"not a UUID", f.olha, "chat"},
	} {
		if _, err := f.service.SendMessage(ctx, test.user, test.id, "Привіт"); !errors.Is(err, ErrConversationNotFound) {
			t.Fatalf("%s: err = %v", test.name, err)
		}
	}

	// Blocking stops new messages in both directions while keeping history readable.
	f.store.SetBlocked(f.andrii, f.olha)
	for _, user := range []string{f.olha, f.andrii} {
		if _, err := f.service.SendMessage(ctx, user, conv.ID, "Нове повідомлення"); !errors.Is(err, ErrConversationBlocked) {
			t.Fatalf("blocked SendMessage(%s): err = %v", user, err)
		}
		updated, err := f.service.Conversation(ctx, user, conv.ID)
		if err != nil || updated.CanSend {
			t.Fatalf("Conversation(%s) after block = %+v, err %v", user, updated, err)
		}
		msgs, err := f.service.Messages(ctx, user, conv.ID, MessageQuery{Page: 1})
		if err != nil || len(msgs.Items) != 2 {
			t.Fatalf("Messages(%s) after block = %+v, err %v", user, msgs, err)
		}
	}
}

func TestMessagesPollingAndReadReceipts(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	req := f.createRequest(t)
	conv, err := f.service.RequestConversation(ctx, f.olha, req.ID)
	if err != nil {
		t.Fatal(err)
	}

	m1, err := f.service.SendMessage(ctx, f.olha, conv.ID, "Перше повідомлення")
	if err != nil {
		t.Fatal(err)
	}
	m2, err := f.service.SendMessage(ctx, f.andrii, conv.ID, "Друге повідомлення")
	if err != nil {
		t.Fatal(err)
	}
	m3, err := f.service.SendMessage(ctx, f.olha, conv.ID, "Третє повідомлення")
	if err != nil {
		t.Fatal(err)
	}

	page, err := f.service.Messages(ctx, f.andrii, conv.ID, MessageQuery{Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 3 || page.Page != 1 || page.PollIntervalSeconds != PollIntervalSeconds || len(page.Items) != 3 {
		t.Fatalf("page = %+v", page)
	}
	if !slices.Equal([]string{page.Items[0].ID, page.Items[1].ID, page.Items[2].ID}, []string{m1.ID, m2.ID, m3.ID}) {
		t.Fatalf("message order = %+v", page.Items)
	}

	//Incremental polling with after returns only newer messages.
	polled, err := f.service.Messages(ctx, f.andrii, conv.ID, MessageQuery{AfterID: strings.ToUpper(m1.ID)})
	if err != nil {
		t.Fatal(err)
	}
	if polled.Total != 2 || len(polled.Items) != 2 || polled.Items[0].ID != m2.ID || polled.Items[1].ID != m3.ID {
		t.Fatalf("polled after m1 = %+v", polled)
	}
	none, err := f.service.Messages(ctx, f.andrii, conv.ID, MessageQuery{AfterID: m3.ID})
	if err != nil || none.Total != 0 || len(none.Items) != 0 {
		t.Fatalf("polled after m3 = %+v, err %v", none, err)
	}

	// Andrii has 2 unread messages from Olha (m1 and m3); Olha has 1 from Andrii (m2).
	andriiConv, _ := f.service.Conversation(ctx, f.andrii, conv.ID)
	olhaConv, _ := f.service.Conversation(ctx, f.olha, conv.ID)
	if andriiConv.UnreadCount != 2 || olhaConv.UnreadCount != 1 {
		t.Fatalf("unread counts: Andrii %d, Olha %d", andriiConv.UnreadCount, olhaConv.UnreadCount)
	}

	readResult, err := f.service.MarkConversationRead(ctx, f.andrii, conv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if readResult.ReadCount != 2 || readResult.Conversation.UnreadCount != 0 {
		t.Fatalf("read result = %+v", readResult)
	}
	// Repeating MarkConversationRead is idempotent.
	again, err := f.service.MarkConversationRead(ctx, f.andrii, conv.ID)
	if err != nil || again.ReadCount != 0 {
		t.Fatalf("repeated read = %+v, err %v", again, err)
	}
	// Olha now sees ReadAt on her messages (m1 and m3).
	afterRead, err := f.service.Messages(ctx, f.olha, conv.ID, MessageQuery{Page: 1})
	if err != nil || afterRead.Items[0].ReadAt == nil || afterRead.Items[1].ReadAt != nil || afterRead.Items[2].ReadAt == nil {
		t.Fatalf("messages after read = %+v, err %v", afterRead, err)
	}

	// Validation and access errors on Messages and MarkConversationRead.
	for _, badAfter := range []string{"not-a-uuid", "73000000-0000-0000-0000-000000000999"} {
		_, err := f.service.Messages(ctx, f.olha, conv.ID, MessageQuery{AfterID: badAfter})
		if fields := fieldErrors(t, err); fields["after"] == "" {
			t.Fatalf("after %q: fields = %v", badAfter, fields)
		}
	}
	_, err = f.service.Messages(ctx, f.olha, conv.ID, MessageQuery{Page: 2, AfterID: m1.ID})
	if fields := fieldErrors(t, err); fields["after"] == "" {
		t.Fatalf("after with page 2: fields = %v", fields)
	}
	for _, p := range []int{-1, MaxPage + 1} {
		if _, err := f.service.Messages(ctx, f.olha, conv.ID, MessageQuery{Page: p}); !errors.Is(err, ErrInvalidPage) {
			t.Fatalf("page %d: err = %v", p, err)
		}
	}
	if _, err := f.service.Messages(ctx, f.stranger, conv.ID, MessageQuery{Page: 1}); !errors.Is(err, ErrConversationNotFound) {
		t.Fatalf("stranger messages: err = %v", err)
	}
	if _, err := f.service.MarkConversationRead(ctx, f.stranger, conv.ID); !errors.Is(err, ErrConversationNotFound) {
		t.Fatalf("stranger mark read: err = %v", err)
	}
}

func TestNotificationsLifecycleAndIdempotency(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	req := f.createRequest(t)

	// Repeating RecordRequestCreated does not duplicate the notification.
	if err := f.service.RecordRequestCreated(ctx, req); err != nil {
		t.Fatal(err)
	}

	// Only Andrii (the recipient) received EXCHANGE_REQUEST; Olha and stranger have none.
	if olhaList, err := f.service.Notifications(ctx, f.olha, false, 1); err != nil || olhaList.Total != 0 || olhaList.UnreadCount != 0 {
		t.Fatalf("Olha notifications = %+v, err %v", olhaList, err)
	}
	andriiList, err := f.service.Notifications(ctx, f.andrii, false, 1)
	if err != nil || andriiList.Total != 1 || andriiList.UnreadCount != 1 || len(andriiList.Items) != 1 {
		t.Fatalf("Andrii notifications = %+v, err %v", andriiList, err)
	}
	first := andriiList.Items[0]
	if first.EventType != data.EventExchangeRequest || first.RequestID != req.ID || first.ConversationID == "" ||
		first.ActorID != f.olha || first.ReadAt != nil {
		t.Fatalf("notification = %+v", first)
	}

	// Olha sends a chat message -> Andrii gets CHAT_MESSAGE (now 2 unread).
	if _, err := f.service.SendMessage(ctx, f.olha, first.ConversationID, "Коли зручно почати?"); err != nil {
		t.Fatal(err)
	}
	andriiList, err = f.service.Notifications(ctx, f.andrii, true, 1)
	if err != nil || andriiList.Total != 2 || andriiList.UnreadCount != 2 {
		t.Fatalf("Andrii unread notifications = %+v, err %v", andriiList, err)
	}

	// Andrii marks the first notification as read.
	marked, err := f.service.MarkNotificationRead(ctx, f.andrii, strings.ToUpper(first.ID))
	if err != nil {
		t.Fatal(err)
	}
	if marked.Notification.ID != first.ID || marked.Notification.ReadAt == nil || marked.UnreadCount != 1 {
		t.Fatalf("marked = %+v", marked)
	}
	// Repeating MarkNotificationRead keeps the same ReadAt and unreadCount.
	again, err := f.service.MarkNotificationRead(ctx, f.andrii, first.ID)
	if err != nil || !again.Notification.ReadAt.Equal(*marked.Notification.ReadAt) || again.UnreadCount != 1 {
		t.Fatalf("marked again = %+v, err %v", again, err)
	}

	// Unread-only filter returns only the 1 remaining unread notification, while all returns 2.
	unreadPage, err := f.service.Notifications(ctx, f.andrii, true, 1)
	if err != nil || unreadPage.Total != 1 || unreadPage.UnreadCount != 1 || len(unreadPage.Items) != 1 {
		t.Fatalf("unread only = %+v, err %v", unreadPage, err)
	}
	allPage, err := f.service.Notifications(ctx, f.andrii, false, 1)
	if err != nil || allPage.Total != 2 || allPage.UnreadCount != 1 || len(allPage.Items) != 2 {
		t.Fatalf("all notifications = %+v, err %v", allPage, err)
	}

	// Andrii marks all notifications read.
	readAll, err := f.service.MarkAllNotificationsRead(ctx, f.andrii)
	if err != nil || readAll.MarkedCount != 1 || readAll.UnreadCount != 0 {
		t.Fatalf("read all = %+v, err %v", readAll, err)
	}

	// Andrii accepts the request -> Olha gets REQUEST_RESPONSE; repeating does not duplicate.
	if err := f.store.UpdateRequestStatus(ctx, req.ID, data.RequestAccepted, f.andrii); err != nil {
		t.Fatal(err)
	}
	exchangeID, err := f.store.CreateExchange(ctx, req.ID, f.andrii)
	if err != nil {
		t.Fatal(err)
	}
	acceptedReq, _ := f.store.RequestByID(ctx, req.ID)
	if err := f.service.RecordRequestAccepted(ctx, acceptedReq, exchangeID); err != nil {
		t.Fatal(err)
	}
	if err := f.service.RecordRequestAccepted(ctx, acceptedReq, exchangeID); err != nil {
		t.Fatal(err)
	}
	olhaList, err := f.service.Notifications(ctx, f.olha, false, 1)
	if err != nil || olhaList.Total != 1 || olhaList.UnreadCount != 1 {
		t.Fatalf("Olha notifications after acceptance = %+v, err %v", olhaList, err)
	}
	if n := olhaList.Items[0]; n.EventType != data.EventRequestResponse || n.RequestID != req.ID || n.ExchangeID != exchangeID {
		t.Fatalf("acceptance notification = %+v", n)
	}

	// Access and validation checks.
	for _, test := range []struct{ name, user, id string }{
		{"another user's notification", f.olha, first.ID},
		{"unknown notification", f.andrii, "74000000-0000-0000-0000-000000000999"},
		{"not a UUID", f.andrii, "notif"},
	} {
		if _, err := f.service.MarkNotificationRead(ctx, test.user, test.id); !errors.Is(err, ErrNotificationNotFound) {
			t.Fatalf("%s: err = %v", test.name, err)
		}
	}
	for _, p := range []int{0, MaxPage + 1} {
		if _, err := f.service.Notifications(ctx, f.andrii, false, p); !errors.Is(err, ErrInvalidPage) {
			t.Fatalf("Notifications page %d: err = %v", p, err)
		}
		if _, err := f.service.Conversations(ctx, f.andrii, p); !errors.Is(err, ErrInvalidPage) {
			t.Fatalf("Conversations page %d: err = %v", p, err)
		}
	}
}
