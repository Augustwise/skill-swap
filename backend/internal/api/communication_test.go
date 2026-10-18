package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"skillswap/backend/internal/data/datatest"
	"skillswap/backend/internal/expect"
)

type conversationPageResponse struct {
	Items    []conversationResponse `json:"items"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"pageSize"`
	Total    int                    `json:"total"`
}

type messagePageResponse struct {
	Items               []messageResponse `json:"items"`
	Page                int               `json:"page"`
	PageSize            int               `json:"pageSize"`
	Total               int               `json:"total"`
	PollIntervalSeconds int               `json:"pollIntervalSeconds"`
}

type notificationPageResponse struct {
	Items       []notificationResponse `json:"items"`
	Page        int                    `json:"page"`
	PageSize    int                    `json:"pageSize"`
	Total       int                    `json:"total"`
	UnreadCount int                    `json:"unreadCount"`
}

func TestConversationsAndMessagesFlow(t *testing.T) {
	e, olha, andrii, andriiID := newRequestEnv(t)
	olhaID := e.meID(t, olha)

	// Olha creates an exchange request to Andrii.
	response := send(t, e.handler, http.MethodPost, requestsPath,
		requestBody(andriiID, datatest.GuitarSkillID, datatest.PhotoshopSkillID), olha)
	expectStatus(t, response, http.StatusCreated)
	var createdReq struct {
		Request exchangeRequest `json:"request"`
	}
	decodeJSON(t, response, &createdReq)
	reqID := createdReq.Request.ID

	// Both participants open the request conversation.
	response = send(t, e.handler, http.MethodGet, requestsPath+"/"+reqID+"/conversation", "", olha)
	expectStatus(t, response, http.StatusOK)
	var openBody struct {
		Conversation conversationResponse `json:"conversation"`
	}
	decodeJSON(t, response, &openBody)
	conv := openBody.Conversation
	expect.Equal(t, "conversation ID is set", conv.ID != "", true)
	expect.Equal(t, "conversation type", conv.Type, "REQUEST")
	expect.Equal(t, "request ID", valueOf(conv.RequestID), reqID)
	expect.Equal(t, "exchange ID is set", conv.ExchangeID != nil, false)
	expect.Equal(t, "request status", valueOf(conv.RequestStatus), "PENDING")
	expect.Equal(t, "exchange status is set", conv.ExchangeStatus != nil, false)
	expect.Equal(t, "Olha's partner", conv.Partner.ID, andriiID)
	expect.Equal(t, "Olha can send", conv.CanSend, true)
	expect.Equal(t, "unread count", conv.UnreadCount, 0)
	expect.Equal(t, "last message is set", conv.LastMessage != nil, false)
	expect.Equal(t, "lastMessageAt is set", conv.LastMessageAt != nil, false)
	for _, fragment := range []string{`"exchangeId":null`, `"exchangeStatus":null`, `"lastMessage":null`, `"lastMessageAt":null`} {
		if !strings.Contains(response.Body.String(), fragment) {
			t.Fatalf("body %s has no %s", response.Body.String(), fragment)
		}
	}

	// Andrii sees Olha as the partner on the same conversation.
	response = send(t, e.handler, http.MethodGet, "/api/v1/conversations/"+conv.ID, "", andrii)
	expectStatus(t, response, http.StatusOK)
	decodeJSON(t, response, &openBody)
	expect.Equal(t, "conversation Andrii opens", openBody.Conversation.ID, conv.ID)
	expect.Equal(t, "Andrii's partner", openBody.Conversation.Partner.ID, olhaID)

	// Olha sends the first message, Andrii replies.
	msgPath := "/api/v1/conversations/" + conv.ID + "/messages"
	response = send(t, e.handler, http.MethodPost, msgPath,
		`{"body":"  Привіт! Зустрінемося в Google Meet: https://meet.example.test/abc  "}`, olha)
	expectStatus(t, response, http.StatusCreated)
	var sent1 struct {
		Message messageResponse `json:"message"`
	}
	decodeJSON(t, response, &sent1)
	expect.Equal(t, "first message ID is set", sent1.Message.ID != "", true)
	expect.Equal(t, "first message conversation", sent1.Message.ConversationID, conv.ID)
	expect.Equal(t, "first message author", valueOf(sent1.Message.Author).ID, olhaID)
	expect.Equal(t, "first message type", sent1.Message.Type, "TEXT")
	expect.Equal(t, "first message body", sent1.Message.Body, "Привіт! Зустрінемося в Google Meet: https://meet.example.test/abc")
	expect.Equal(t, "first message readAt is set", sent1.Message.ReadAt != nil, false)

	response = send(t, e.handler, http.MethodPost, msgPath, `{"body":"Домовилися, підходить!"}`, andrii)
	expectStatus(t, response, http.StatusCreated)
	var sent2 struct {
		Message messageResponse `json:"message"`
	}
	decodeJSON(t, response, &sent2)

	// Andrii polls only messages after sent1.
	polled := sendAndDecode[messagePageResponse](t, e, http.MethodGet, msgPath+"?after="+sent1.Message.ID, "", andrii, http.StatusOK)
	expect.Equal(t, "polled total", polled.Total, 1)
	expect.Equal(t, "polled messages", len(polled.Items), 1)
	expect.Equal(t, "polled message", polled.Items[0].ID, sent2.Message.ID)
	expect.Equal(t, "poll interval in seconds", polled.PollIntervalSeconds, 5)
	expect.Equal(t, "page size", polled.PageSize, 20)

	// Andrii marks the conversation read.
	response = send(t, e.handler, http.MethodPost, "/api/v1/conversations/"+conv.ID+"/read", "", andrii)
	expectStatus(t, response, http.StatusOK)
	var readResp struct {
		Conversation conversationResponse `json:"conversation"`
		ReadCount    int                  `json:"readCount"`
	}
	decodeJSON(t, response, &readResp)
	expect.Equal(t, "messages marked read", readResp.ReadCount, 1)
	expect.Equal(t, "unread count after reading", readResp.Conversation.UnreadCount, 0)

	// Andrii accepts the request: the conversation becomes an EXCHANGE chat and keeps history.
	accepted := sendAndDecode[exchangeEnvelope](t, e, http.MethodPost, requestsPath+"/"+reqID+"/accept", "", andrii, http.StatusOK)

	response = send(t, e.handler, http.MethodGet, "/api/v1/exchanges/"+accepted.Exchange.ID+"/conversation", "", olha)
	expectStatus(t, response, http.StatusOK)
	decodeJSON(t, response, &openBody)
	linked := openBody.Conversation
	expect.Equal(t, "conversation after acceptance", linked.ID, conv.ID)
	expect.Equal(t, "conversation type after acceptance", linked.Type, "EXCHANGE")
	expect.Equal(t, "exchange ID", valueOf(linked.ExchangeID), accepted.Exchange.ID)
	expect.Equal(t, "exchange status", valueOf(linked.ExchangeStatus), "ACTIVE")
	expect.Equal(t, "Olha's unread count", linked.UnreadCount, 1)
	expect.Equal(t, "last message", valueOf(linked.LastMessage).ID, sent2.Message.ID)

	// /me/conversations lists the conversation with the last message preview.
	list := sendAndDecode[conversationPageResponse](t, e, http.MethodGet, "/api/v1/me/conversations", "", olha, http.StatusOK)
	expect.Equal(t, "Olha's conversations total", list.Total, 1)
	expect.Equal(t, "Olha's conversations", len(list.Items), 1)
	expect.Equal(t, "listed conversation", list.Items[0].ID, conv.ID)

	// Full message history is preserved after acceptance.
	history := sendAndDecode[messagePageResponse](t, e, http.MethodGet, msgPath, "", olha, http.StatusOK)
	expect.Equal(t, "history total", history.Total, 2)
	expect.Equal(t, "history messages", len(history.Items), 2)
	expect.Equal(t, "first history message readAt is set", history.Items[0].ReadAt != nil, true)
}

func TestConversationAccessValidationAndBlocking(t *testing.T) {
	e, olha, andrii, andriiID := newRequestEnv(t)
	stranger := e.student(t, "marko@students.example.test", `["ONLINE"]`, "Київ", datatest.PhotoshopSkillID, datatest.GuitarSkillID)

	response := send(t, e.handler, http.MethodPost, requestsPath,
		requestBody(andriiID, datatest.GuitarSkillID, datatest.PhotoshopSkillID), olha)
	expectStatus(t, response, http.StatusCreated)
	var createdReq struct {
		Request exchangeRequest `json:"request"`
	}
	decodeJSON(t, response, &createdReq)
	reqID := createdReq.Request.ID

	response = send(t, e.handler, http.MethodGet, requestsPath+"/"+reqID+"/conversation", "", olha)
	expectStatus(t, response, http.StatusOK)
	var openBody struct {
		Conversation conversationResponse `json:"conversation"`
	}
	decodeJSON(t, response, &openBody)
	convID := openBody.Conversation.ID
	msgPath := "/api/v1/conversations/" + convID + "/messages"

	// Unauthenticated and unverified checks.
	assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/me/conversations", "", nil), http.StatusUnauthorized, "unauthenticated")
	assertProblem(t, send(t, e.handler, http.MethodGet, msgPath, "", nil), http.StatusUnauthorized, "unauthenticated")
	assertProblem(t, send(t, e.handler, http.MethodPost, msgPath, `{"body":"Привіт"}`, nil), http.StatusUnauthorized, "unauthenticated")
	assertProblem(t, send(t, e.handler, http.MethodPost, msgPath, `{"body":"Привіт"}`, &session{cookie: olha.cookie}),
		http.StatusForbidden, "csrf_invalid")

	foreignReq := httptest.NewRequest(http.MethodPost, msgPath, strings.NewReader(`{"body":"Привіт"}`))
	foreignReq.Header.Set("Origin", "https://evil.example")
	foreignReq.Header.Set("Content-Type", "application/json")
	foreignReq.Header.Set(csrfHeaderName, olha.csrf)
	foreignReq.AddCookie(olha.cookie)
	foreignResp := httptest.NewRecorder()
	e.handler.ServeHTTP(foreignResp, foreignReq)
	assertProblem(t, foreignResp, http.StatusForbidden, "origin_forbidden")

	// Third-party access is denied with 404.
	assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/conversations/"+convID, "", stranger),
		http.StatusNotFound, "conversation_not_found")
	assertProblem(t, send(t, e.handler, http.MethodGet, msgPath, "", stranger),
		http.StatusNotFound, "conversation_not_found")
	assertProblem(t, send(t, e.handler, http.MethodPost, msgPath, `{"body":"Привіт"}`, stranger),
		http.StatusNotFound, "conversation_not_found")
	assertProblem(t, send(t, e.handler, http.MethodPost, "/api/v1/conversations/"+convID+"/read", "", stranger),
		http.StatusNotFound, "conversation_not_found")
	assertProblem(t, send(t, e.handler, http.MethodGet, requestsPath+"/"+reqID+"/conversation", "", stranger),
		http.StatusNotFound, "request_not_found")
	assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/exchanges/80000000-0000-0000-0000-000000000999/conversation", "", stranger),
		http.StatusNotFound, "exchange_not_found")

	// Validation errors on messages.
	assertFieldError(t, send(t, e.handler, http.MethodPost, msgPath, `{"body":"   "}`, olha), "body")
	assertFieldError(t, send(t, e.handler, http.MethodPost, msgPath,
		fmt.Sprintf(`{"body":%q}`, strings.Repeat("ї", 2001)), olha), "body")
	assertFieldError(t, send(t, e.handler, http.MethodGet, msgPath+"?after=not-a-uuid", "", olha), "after")
	assertProblem(t, send(t, e.handler, http.MethodGet, msgPath+"?page=0", "", olha), http.StatusBadRequest, "invalid_page")

	// Personal block stops new messages with 409 conversation_blocked.
	e.store.SetBlocked(andriiID, e.meID(t, olha))
	assertProblem(t, send(t, e.handler, http.MethodPost, msgPath, `{"body":"Привіт"}`, olha),
		http.StatusConflict, "conversation_blocked")
	assertProblem(t, send(t, e.handler, http.MethodPost, msgPath, `{"body":"Привіт"}`, andrii),
		http.StatusConflict, "conversation_blocked")
}

func TestNotificationsEndpoints(t *testing.T) {
	e, olha, andrii, andriiID := newRequestEnv(t)
	olhaID := e.meID(t, olha)

	// Olha sends a request -> Andrii receives EXCHANGE_REQUEST notification.
	response := send(t, e.handler, http.MethodPost, requestsPath,
		requestBody(andriiID, datatest.GuitarSkillID, datatest.PhotoshopSkillID), olha)
	expectStatus(t, response, http.StatusCreated)
	var createdReq struct {
		Request exchangeRequest `json:"request"`
	}
	decodeJSON(t, response, &createdReq)
	reqID := createdReq.Request.ID

	andriiPage := sendAndDecode[notificationPageResponse](t, e, http.MethodGet, "/api/v1/me/notifications?unreadOnly=true", "", andrii, http.StatusOK)
	expect.Equal(t, "Andrii's notifications total", andriiPage.Total, 1)
	expect.Equal(t, "Andrii's unread count", andriiPage.UnreadCount, 1)
	expect.Equal(t, "Andrii's notifications", len(andriiPage.Items), 1)
	reqNotif := andriiPage.Items[0]
	expect.Equal(t, "request notification type", reqNotif.EventType, "EXCHANGE_REQUEST")
	expect.Equal(t, "request notification is read", reqNotif.IsRead, false)
	expect.Equal(t, "request notification readAt is set", reqNotif.ReadAt != nil, false)
	expect.Equal(t, "request notification actor", valueOf(reqNotif.Actor).ID, olhaID)
	expect.Equal(t, "request notification request", valueOf(reqNotif.RequestID), reqID)
	expect.Equal(t, "request notification conversation is set", reqNotif.ConversationID != nil, true)
	expect.Equal(t, "request notification target kind", reqNotif.Target.Kind, "EXCHANGE_REQUEST")
	expect.Equal(t, "request notification target", reqNotif.Target.ID, reqID)

	// Olha has no notifications yet.
	response = send(t, e.handler, http.MethodGet, "/api/v1/me/notifications", "", olha)
	expectStatus(t, response, http.StatusOK)
	assertJSONEqual(t, response.Body.Bytes(), `{"items":[],"page":1,"pageSize":20,"total":0,"unreadCount":0}`)

	// Olha sends a chat message -> Andrii now has 2 unread notifications.
	convID := *reqNotif.ConversationID
	expectStatus(t, send(t, e.handler, http.MethodPost, "/api/v1/conversations/"+convID+"/messages",
		`{"body":"Привіт у чаті!"}`, olha), http.StatusCreated)

	andriiPage = sendAndDecode[notificationPageResponse](t, e, http.MethodGet, "/api/v1/me/notifications?unreadOnly=true", "", andrii, http.StatusOK)
	expect.Equal(t, "Andrii's notifications total after the message", andriiPage.Total, 2)
	expect.Equal(t, "Andrii's unread count after the message", andriiPage.UnreadCount, 2)
	chatNotif := andriiPage.Items[0]
	expect.Equal(t, "chat notification type", chatNotif.EventType, "CHAT_MESSAGE")
	expect.Equal(t, "chat notification message is set", chatNotif.MessageID != nil, true)
	expect.Equal(t, "chat notification target kind", chatNotif.Target.Kind, "CONVERSATION")
	expect.Equal(t, "chat notification target", chatNotif.Target.ID, convID)

	// Olha cannot mark Andrii's notification as read.
	assertProblem(t, send(t, e.handler, http.MethodPost, "/api/v1/me/notifications/"+reqNotif.ID+"/read", "", olha),
		http.StatusNotFound, "notification_not_found")

	// Andrii marks the request notification as read -> unreadCount drops from 2 to 1.
	response = send(t, e.handler, http.MethodPost, "/api/v1/me/notifications/"+reqNotif.ID+"/read", "", andrii)
	expectStatus(t, response, http.StatusOK)
	var marked struct {
		Notification notificationResponse `json:"notification"`
		UnreadCount  int                  `json:"unreadCount"`
	}
	decodeJSON(t, response, &marked)
	expect.Equal(t, "notification is read", marked.Notification.IsRead, true)
	expect.Equal(t, "notification readAt is set", marked.Notification.ReadAt != nil, true)
	expect.Equal(t, "unread count after marking one", marked.UnreadCount, 1)

	// Andrii marks all remaining notifications as read -> unreadCount becomes 0.
	response = send(t, e.handler, http.MethodPost, "/api/v1/me/notifications/read-all", "", andrii)
	expectStatus(t, response, http.StatusOK)
	var readAll struct {
		MarkedCount int `json:"markedCount"`
		UnreadCount int `json:"unreadCount"`
	}
	decodeJSON(t, response, &readAll)
	expect.Equal(t, "notifications marked read", readAll.MarkedCount, 1)
	expect.Equal(t, "unread count after marking all", readAll.UnreadCount, 0)

	// Andrii accepts the request (twice to check idempotency) -> Olha receives 1 REQUEST_RESPONSE notification.
	expectStatus(t, send(t, e.handler, http.MethodPost, requestsPath+"/"+reqID+"/accept", "", andrii), http.StatusOK)
	expectStatus(t, send(t, e.handler, http.MethodPost, requestsPath+"/"+reqID+"/accept", "", andrii), http.StatusOK)

	olhaPage := sendAndDecode[notificationPageResponse](t, e, http.MethodGet, "/api/v1/me/notifications", "", olha, http.StatusOK)
	expect.Equal(t, "Olha's notifications total", olhaPage.Total, 1)
	expect.Equal(t, "Olha's unread count", olhaPage.UnreadCount, 1)
	expect.Equal(t, "Olha's notifications", len(olhaPage.Items), 1)
	acceptedNotif := olhaPage.Items[0]
	expect.Equal(t, "accepted notification type", acceptedNotif.EventType, "REQUEST_RESPONSE")
	expect.Equal(t, "accepted notification exchange is set", acceptedNotif.ExchangeID != nil, true)
	expect.Equal(t, "accepted notification target kind", acceptedNotif.Target.Kind, "EXCHANGE")
	expect.Equal(t, "accepted notification target", acceptedNotif.Target.ID, *acceptedNotif.ExchangeID)

	// Filter validation and auth errors.
	assertFieldError(t, send(t, e.handler, http.MethodGet, "/api/v1/me/notifications?unreadOnly=maybe", "", olha), "unreadOnly")
	assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/me/notifications?page=0", "", olha), http.StatusBadRequest, "invalid_page")
	assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/me/notifications", "", nil), http.StatusUnauthorized, "unauthenticated")
	assertProblem(t, send(t, e.handler, http.MethodPost, "/api/v1/me/notifications/read-all", "", &session{cookie: olha.cookie}),
		http.StatusForbidden, "csrf_invalid")
}
