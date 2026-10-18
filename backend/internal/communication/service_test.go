package communication

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"skillswap/backend/internal/data"
	"skillswap/backend/internal/data/datatest"
	"skillswap/backend/internal/expect"
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
	expect.Equal(t, "conversation ID is set", byRequest.ID != "", true)
	expect.Equal(t, "conversation type", byRequest.Type, data.ConversationRequest)
	expect.Equal(t, "request ID", byRequest.RequestID, req.ID)
	expect.Equal(t, "exchange ID", byRequest.ExchangeID, "")
	expect.Equal(t, "request status", byRequest.RequestStatus, data.RequestPending)
	expect.Equal(t, "can send", byRequest.CanSend, true)
	expect.Equal(t, "unread count", byRequest.UnreadCount, 0)
	expect.Equal(t, "last message is set", byRequest.LastMessage != nil, false)

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
	expect.Equal(t, "linked conversation ID", byExchange.ID, byRequest.ID)
	expect.Equal(t, "linked conversation type", byExchange.Type, data.ConversationExchange)
	expect.Equal(t, "linked request ID", byExchange.RequestID, req.ID)
	expect.Equal(t, "linked exchange ID", byExchange.ExchangeID, exchangeID)
	expect.Equal(t, "linked request status", byExchange.RequestStatus, data.RequestAccepted)
	expect.Equal(t, "linked exchange status", byExchange.ExchangeStatus, data.ExchangeActive)
	expect.Equal(t, "linked unread count", byExchange.UnreadCount, 1)
	expect.Equal(t, "linked last message is set", byExchange.LastMessage != nil, true)
	expect.Equal(t, "linked last message ID", byExchange.LastMessage.ID, msg.ID)

	participants := []struct{ name, id string }{{"Olha", f.olha}, {"Andrii", f.andrii}}
	for _, user := range participants {
		conv, err := f.service.Conversation(ctx, user.id, strings.ToUpper(byRequest.ID))
		expect.NoError(t, user.name+": open conversation", err)
		expect.Equal(t, user.name+": conversation ID", conv.ID, byRequest.ID)

		list, err := f.service.Conversations(ctx, user.id, 1)
		expect.NoError(t, user.name+": list conversations", err)
		expect.Equal(t, user.name+": conversations total", list.Total, 1)
		expect.Equal(t, user.name+": conversations on the page", len(list.Items), 1)
		expect.Equal(t, user.name+": listed conversation ID", list.Items[0].ID, byRequest.ID)
	}

	// Access checks for a third party and invalid IDs.
	for _, id := range []string{byRequest.ID, "72000000-0000-0000-0000-000000000999", "not-a-uuid"} {
		_, err := f.service.Conversation(ctx, f.stranger, id)
		expect.Error(t, "stranger opens conversation "+id, err, ErrConversationNotFound)
	}
	for _, id := range []string{req.ID, "70000000-0000-0000-0000-000000000999", "not-a-uuid"} {
		_, err := f.service.RequestConversation(ctx, f.stranger, id)
		expect.Error(t, "stranger opens request conversation "+id, err, ErrRequestNotFound)
	}
	for _, id := range []string{exchangeID, "80000000-0000-0000-0000-000000000999", "not-a-uuid"} {
		_, err := f.service.ExchangeConversation(ctx, f.stranger, id)
		expect.Error(t, "stranger opens exchange conversation "+id, err, ErrExchangeNotFound)
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
	expect.Equal(t, "message ID is set", sent.ID != "", true)
	expect.Equal(t, "conversation ID", sent.ConversationID, conv.ID)
	expect.Equal(t, "author ID", sent.AuthorID, f.olha)
	expect.Equal(t, "author first name", sent.AuthorFirstName, "Ольга")
	expect.Equal(t, "message type", sent.Type, data.MessageText)
	expect.Equal(t, "trimmed body", sent.Body, "Привіт! Ось посилання на зустріч:\nhttps://meet.example.test/skill-swap")

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
			fields := fieldErrors(t, err)
			expect.Equal(t, "body error is set", fields["body"] != "", true)
			expect.Equal(t, "number of field errors", len(fields), 1)
		})
	}

	for _, test := range []struct{ name, user, id string }{
		{"not a participant", f.stranger, conv.ID},
		{"unknown conversation", f.olha, "72000000-0000-0000-0000-000000000999"},
		{"not a UUID", f.olha, "chat"},
	} {
		_, err := f.service.SendMessage(ctx, test.user, test.id, "Привіт")
		expect.Error(t, test.name+": send message", err, ErrConversationNotFound)
	}

	// Blocking stops new messages in both directions while keeping history readable.
	f.store.SetBlocked(f.andrii, f.olha)
	participants := []struct{ name, id string }{{"Olha", f.olha}, {"Andrii", f.andrii}}
	for _, user := range participants {
		_, err := f.service.SendMessage(ctx, user.id, conv.ID, "Нове повідомлення")
		expect.Error(t, user.name+": send message after a block", err, ErrConversationBlocked)

		updated, err := f.service.Conversation(ctx, user.id, conv.ID)
		expect.NoError(t, user.name+": open conversation after a block", err)
		expect.Equal(t, user.name+": can send after a block", updated.CanSend, false)

		msgs, err := f.service.Messages(ctx, user.id, conv.ID, MessageQuery{Page: 1})
		expect.NoError(t, user.name+": read messages after a block", err)
		expect.Equal(t, user.name+": messages after a block", len(msgs.Items), 2)
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
	expect.Equal(t, "messages total", page.Total, 3)
	expect.Equal(t, "page", page.Page, 1)
	expect.Equal(t, "poll interval", page.PollIntervalSeconds, PollIntervalSeconds)
	expect.Equal(t, "messages on the page", len(page.Items), 3)
	expect.Slice(t, "message order",
		[]string{page.Items[0].ID, page.Items[1].ID, page.Items[2].ID}, []string{m1.ID, m2.ID, m3.ID})

	// Incremental polling with after returns only newer messages.
	polled, err := f.service.Messages(ctx, f.andrii, conv.ID, MessageQuery{AfterID: strings.ToUpper(m1.ID)})
	if err != nil {
		t.Fatal(err)
	}
	expect.Equal(t, "total after m1", polled.Total, 2)
	expect.Equal(t, "messages after m1", len(polled.Items), 2)
	expect.Equal(t, "first message after m1", polled.Items[0].ID, m2.ID)
	expect.Equal(t, "second message after m1", polled.Items[1].ID, m3.ID)

	none, err := f.service.Messages(ctx, f.andrii, conv.ID, MessageQuery{AfterID: m3.ID})
	expect.NoError(t, "poll after m3", err)
	expect.Equal(t, "total after m3", none.Total, 0)
	expect.Equal(t, "messages after m3", len(none.Items), 0)

	// Andrii has 2 unread messages from Olha (m1 and m3); Olha has 1 from Andrii (m2).
	andriiConv, _ := f.service.Conversation(ctx, f.andrii, conv.ID)
	olhaConv, _ := f.service.Conversation(ctx, f.olha, conv.ID)
	expect.Equal(t, "Andrii's unread count", andriiConv.UnreadCount, 2)
	expect.Equal(t, "Olha's unread count", olhaConv.UnreadCount, 1)

	readResult, err := f.service.MarkConversationRead(ctx, f.andrii, conv.ID)
	if err != nil {
		t.Fatal(err)
	}
	expect.Equal(t, "messages marked read", readResult.ReadCount, 2)
	expect.Equal(t, "unread count after reading", readResult.Conversation.UnreadCount, 0)

	// Repeating MarkConversationRead is idempotent.
	again, err := f.service.MarkConversationRead(ctx, f.andrii, conv.ID)
	expect.NoError(t, "mark the conversation read again", err)
	expect.Equal(t, "messages marked read again", again.ReadCount, 0)

	// Olha now sees ReadAt on her messages (m1 and m3).
	afterRead, err := f.service.Messages(ctx, f.olha, conv.ID, MessageQuery{Page: 1})
	expect.NoError(t, "Olha reads messages after Andrii read them", err)
	expect.Equal(t, "m1 readAt is set", afterRead.Items[0].ReadAt != nil, true)
	expect.Equal(t, "m2 readAt is set", afterRead.Items[1].ReadAt != nil, false)
	expect.Equal(t, "m3 readAt is set", afterRead.Items[2].ReadAt != nil, true)

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
		_, err := f.service.Messages(ctx, f.olha, conv.ID, MessageQuery{Page: p})
		expect.Error(t, fmt.Sprintf("messages page %d", p), err, ErrInvalidPage)
	}
	_, err = f.service.Messages(ctx, f.stranger, conv.ID, MessageQuery{Page: 1})
	expect.Error(t, "stranger reads messages", err, ErrConversationNotFound)
	_, err = f.service.MarkConversationRead(ctx, f.stranger, conv.ID)
	expect.Error(t, "stranger marks the conversation read", err, ErrConversationNotFound)
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
	olhaList, err := f.service.Notifications(ctx, f.olha, false, 1)
	expect.NoError(t, "Olha's notifications", err)
	expect.Equal(t, "Olha's notifications total", olhaList.Total, 0)
	expect.Equal(t, "Olha's unread count", olhaList.UnreadCount, 0)

	andriiList, err := f.service.Notifications(ctx, f.andrii, false, 1)
	expect.NoError(t, "Andrii's notifications", err)
	expect.Equal(t, "Andrii's notifications total", andriiList.Total, 1)
	expect.Equal(t, "Andrii's unread count", andriiList.UnreadCount, 1)
	expect.Equal(t, "Andrii's notifications on the page", len(andriiList.Items), 1)

	first := andriiList.Items[0]
	expect.Equal(t, "event type", first.EventType, data.EventExchangeRequest)
	expect.Equal(t, "request ID", first.RequestID, req.ID)
	expect.Equal(t, "conversation ID is set", first.ConversationID != "", true)
	expect.Equal(t, "actor ID", first.ActorID, f.olha)
	expect.Equal(t, "readAt is set", first.ReadAt != nil, false)

	// Olha sends a chat message -> Andrii gets CHAT_MESSAGE (now 2 unread).
	if _, err := f.service.SendMessage(ctx, f.olha, first.ConversationID, "Коли зручно почати?"); err != nil {
		t.Fatal(err)
	}
	andriiList, err = f.service.Notifications(ctx, f.andrii, true, 1)
	expect.NoError(t, "Andrii's unread notifications", err)
	expect.Equal(t, "Andrii's unread notifications total", andriiList.Total, 2)
	expect.Equal(t, "Andrii's unread count after the message", andriiList.UnreadCount, 2)

	// Andrii marks the first notification as read.
	marked, err := f.service.MarkNotificationRead(ctx, f.andrii, strings.ToUpper(first.ID))
	if err != nil {
		t.Fatal(err)
	}
	expect.Equal(t, "marked notification ID", marked.Notification.ID, first.ID)
	expect.Equal(t, "marked readAt is set", marked.Notification.ReadAt != nil, true)
	expect.Equal(t, "unread count after marking", marked.UnreadCount, 1)

	// Repeating MarkNotificationRead keeps the same ReadAt and unreadCount.
	again, err := f.service.MarkNotificationRead(ctx, f.andrii, first.ID)
	expect.NoError(t, "mark the notification read again", err)
	expect.Equal(t, "readAt is set after marking again", again.Notification.ReadAt != nil, true)
	expect.Equal(t, "readAt kept after marking again", again.Notification.ReadAt.Equal(*marked.Notification.ReadAt), true)
	expect.Equal(t, "unread count after marking again", again.UnreadCount, 1)

	// Unread-only filter returns only the 1 remaining unread notification, while all returns 2.
	unreadPage, err := f.service.Notifications(ctx, f.andrii, true, 1)
	expect.NoError(t, "unread notifications", err)
	expect.Equal(t, "unread-only total", unreadPage.Total, 1)
	expect.Equal(t, "unread-only unread count", unreadPage.UnreadCount, 1)
	expect.Equal(t, "unread-only notifications on the page", len(unreadPage.Items), 1)

	allPage, err := f.service.Notifications(ctx, f.andrii, false, 1)
	expect.NoError(t, "all notifications", err)
	expect.Equal(t, "all notifications total", allPage.Total, 2)
	expect.Equal(t, "all notifications unread count", allPage.UnreadCount, 1)
	expect.Equal(t, "all notifications on the page", len(allPage.Items), 2)

	// Andrii marks all notifications read.
	readAll, err := f.service.MarkAllNotificationsRead(ctx, f.andrii)
	expect.NoError(t, "mark all notifications read", err)
	expect.Equal(t, "notifications marked read", readAll.MarkedCount, 1)
	expect.Equal(t, "unread count after marking all", readAll.UnreadCount, 0)

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
	olhaList, err = f.service.Notifications(ctx, f.olha, false, 1)
	expect.NoError(t, "Olha's notifications after acceptance", err)
	expect.Equal(t, "Olha's notifications total after acceptance", olhaList.Total, 1)
	expect.Equal(t, "Olha's unread count after acceptance", olhaList.UnreadCount, 1)

	n := olhaList.Items[0]
	expect.Equal(t, "acceptance event type", n.EventType, data.EventRequestResponse)
	expect.Equal(t, "acceptance request ID", n.RequestID, req.ID)
	expect.Equal(t, "acceptance exchange ID", n.ExchangeID, exchangeID)

	// Access and validation checks.
	for _, test := range []struct{ name, user, id string }{
		{"another user's notification", f.olha, first.ID},
		{"unknown notification", f.andrii, "74000000-0000-0000-0000-000000000999"},
		{"not a UUID", f.andrii, "notif"},
	} {
		_, err := f.service.MarkNotificationRead(ctx, test.user, test.id)
		expect.Error(t, test.name+": mark read", err, ErrNotificationNotFound)
	}
	for _, p := range []int{0, MaxPage + 1} {
		_, err := f.service.Notifications(ctx, f.andrii, false, p)
		expect.Error(t, fmt.Sprintf("notifications page %d", p), err, ErrInvalidPage)
		_, err = f.service.Conversations(ctx, f.andrii, p)
		expect.Error(t, fmt.Sprintf("conversations page %d", p), err, ErrInvalidPage)
	}
}
