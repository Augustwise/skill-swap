package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"skillswap/backend/internal/data/datatest"
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
	if conv.ID == "" || conv.Type != "REQUEST" || conv.RequestID == nil || *conv.RequestID != reqID ||
		conv.ExchangeID != nil || conv.RequestStatus == nil || *conv.RequestStatus != "PENDING" ||
		conv.ExchangeStatus != nil || conv.Partner.ID != andriiID || !conv.CanSend || conv.UnreadCount != 0 ||
		conv.LastMessage != nil || conv.LastMessageAt != nil {
		t.Fatalf("request conversation = %+v", conv)
	}
	for _, fragment := range []string{`"exchangeId":null`, `"exchangeStatus":null`, `"lastMessage":null`, `"lastMessageAt":null`} {
		if !strings.Contains(response.Body.String(), fragment) {
			t.Fatalf("body %s has no %s", response.Body.String(), fragment)
		}
	}

	// Andrii sees Olha as the partner on the same conversation.
	response = send(t, e.handler, http.MethodGet, "/api/v1/conversations/"+conv.ID, "", andrii)
	expectStatus(t, response, http.StatusOK)
	decodeJSON(t, response, &openBody)
	if openBody.Conversation.ID != conv.ID || openBody.Conversation.Partner.ID != olhaID {
		t.Fatalf("Andrii conversation view = %+v", openBody.Conversation)
	}

	// Olha sends the first message, Andrii replies.
	msgPath := "/api/v1/conversations/" + conv.ID + "/messages"
	response = send(t, e.handler, http.MethodPost, msgPath,
		`{"body":"  Привіт! Зустрінемося в Google Meet: https://meet.example.test/abc  "}`, olha)
	expectStatus(t, response, http.StatusCreated)
	var sent1 struct {
		Message messageResponse `json:"message"`
	}
	decodeJSON(t, response, &sent1)
	if sent1.Message.ID == "" || sent1.Message.ConversationID != conv.ID || sent1.Message.Author == nil ||
		sent1.Message.Author.ID != olhaID || sent1.Message.Type != "TEXT" ||
		sent1.Message.Body != "Привіт! Зустрінемося в Google Meet: https://meet.example.test/abc" ||
		sent1.Message.ReadAt != nil {
		t.Fatalf("first message = %+v", sent1.Message)
	}

	response = send(t, e.handler, http.MethodPost, msgPath, `{"body":"Домовилися, підходить!"}`, andrii)
	expectStatus(t, response, http.StatusCreated)
	var sent2 struct {
		Message messageResponse `json:"message"`
	}
	decodeJSON(t, response, &sent2)

	// Andrii polls only messages after sent1.
	response = send(t, e.handler, http.MethodGet, msgPath+"?after="+sent1.Message.ID, "", andrii)
	expectStatus(t, response, http.StatusOK)
	var polled messagePageResponse
	decodeJSON(t, response, &polled)
	if polled.Total != 1 || len(polled.Items) != 1 || polled.Items[0].ID != sent2.Message.ID ||
		polled.PollIntervalSeconds != 5 || polled.PageSize != 20 {
		t.Fatalf("polled messages = %+v", polled)
	}

	// Andrii marks the conversation read.
	response = send(t, e.handler, http.MethodPost, "/api/v1/conversations/"+conv.ID+"/read", "", andrii)
	expectStatus(t, response, http.StatusOK)
	var readResp struct {
		Conversation conversationResponse `json:"conversation"`
		ReadCount    int                  `json:"readCount"`
	}
	decodeJSON(t, response, &readResp)
	if readResp.ReadCount != 1 || readResp.Conversation.UnreadCount != 0 {
		t.Fatalf("mark conversation read = %+v", readResp)
	}

	// Andrii accepts the request: the conversation becomes an EXCHANGE chat and keeps history.
	response = send(t, e.handler, http.MethodPost, requestsPath+"/"+reqID+"/accept", "", andrii)
	expectStatus(t, response, http.StatusOK)
	var accepted struct {
		Exchange exchangeResponse `json:"exchange"`
	}
	decodeJSON(t, response, &accepted)

	response = send(t, e.handler, http.MethodGet, "/api/v1/exchanges/"+accepted.Exchange.ID+"/conversation", "", olha)
	expectStatus(t, response, http.StatusOK)
	decodeJSON(t, response, &openBody)
	linked := openBody.Conversation
	if linked.ID != conv.ID || linked.Type != "EXCHANGE" || linked.ExchangeID == nil ||
		*linked.ExchangeID != accepted.Exchange.ID || linked.ExchangeStatus == nil || *linked.ExchangeStatus != "ACTIVE" ||
		linked.UnreadCount != 1 || linked.LastMessage == nil || linked.LastMessage.ID != sent2.Message.ID {
		t.Fatalf("exchange conversation = %+v", linked)
	}

	// /me/conversations lists the conversation with the last message preview.
	response = send(t, e.handler, http.MethodGet, "/api/v1/me/conversations", "", olha)
	expectStatus(t, response, http.StatusOK)
	var list conversationPageResponse
	decodeJSON(t, response, &list)
	if list.Total != 1 || len(list.Items) != 1 || list.Items[0].ID != conv.ID {
		t.Fatalf("conversations list = %+v", list)
	}

	// Full message history is preserved after acceptance.
	response = send(t, e.handler, http.MethodGet, msgPath, "", olha)
	expectStatus(t, response, http.StatusOK)
	var history messagePageResponse
	decodeJSON(t, response, &history)
	if history.Total != 2 || len(history.Items) != 2 || history.Items[0].ReadAt == nil {
		t.Fatalf("message history = %+v", history)
	}
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

	response = send(t, e.handler, http.MethodGet, "/api/v1/me/notifications?unreadOnly=true", "", andrii)
	expectStatus(t, response, http.StatusOK)
	var andriiPage notificationPageResponse
	decodeJSON(t, response, &andriiPage)
	if andriiPage.Total != 1 || andriiPage.UnreadCount != 1 || len(andriiPage.Items) != 1 {
		t.Fatalf("Andrii notifications = %+v", andriiPage)
	}
	reqNotif := andriiPage.Items[0]
	if reqNotif.EventType != "EXCHANGE_REQUEST" || reqNotif.IsRead || reqNotif.ReadAt != nil ||
		reqNotif.Actor == nil || reqNotif.Actor.ID != olhaID ||
		reqNotif.RequestID == nil || *reqNotif.RequestID != reqID ||
		reqNotif.ConversationID == nil || reqNotif.Target.Kind != "EXCHANGE_REQUEST" || reqNotif.Target.ID != reqID {
		t.Fatalf("request notification = %+v", reqNotif)
	}

	// Olha has no notifications yet.
	response = send(t, e.handler, http.MethodGet, "/api/v1/me/notifications", "", olha)
	expectStatus(t, response, http.StatusOK)
	assertJSONEqual(t, response.Body.Bytes(), `{"items":[],"page":1,"pageSize":20,"total":0,"unreadCount":0}`)

	// Olha sends a chat message -> Andrii now has 2 unread notifications.
	convID := *reqNotif.ConversationID
	expectStatus(t, send(t, e.handler, http.MethodPost, "/api/v1/conversations/"+convID+"/messages",
		`{"body":"Привіт у чаті!"}`, olha), http.StatusCreated)

	response = send(t, e.handler, http.MethodGet, "/api/v1/me/notifications?unreadOnly=true", "", andrii)
	expectStatus(t, response, http.StatusOK)
	decodeJSON(t, response, &andriiPage)
	if andriiPage.Total != 2 || andriiPage.UnreadCount != 2 {
		t.Fatalf("Andrii unread notifications = %+v", andriiPage)
	}
	chatNotif := andriiPage.Items[0]
	if chatNotif.EventType != "CHAT_MESSAGE" || chatNotif.MessageID == nil ||
		chatNotif.Target.Kind != "CONVERSATION" || chatNotif.Target.ID != convID {
		t.Fatalf("chat notification = %+v", chatNotif)
	}

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
	if !marked.Notification.IsRead || marked.Notification.ReadAt == nil || marked.UnreadCount != 1 {
		t.Fatalf("marked notification = %+v", marked)
	}

	// Andrii marks all remaining notifications as read -> unreadCount becomes 0.
	response = send(t, e.handler, http.MethodPost, "/api/v1/me/notifications/read-all", "", andrii)
	expectStatus(t, response, http.StatusOK)
	var readAll struct {
		MarkedCount int `json:"markedCount"`
		UnreadCount int `json:"unreadCount"`
	}
	decodeJSON(t, response, &readAll)
	if readAll.MarkedCount != 1 || readAll.UnreadCount != 0 {
		t.Fatalf("readAll = %+v", readAll)
	}

	// Andrii accepts the request (twice to check idempotency) -> Olha receives 1 REQUEST_RESPONSE notification.
	expectStatus(t, send(t, e.handler, http.MethodPost, requestsPath+"/"+reqID+"/accept", "", andrii), http.StatusOK)
	expectStatus(t, send(t, e.handler, http.MethodPost, requestsPath+"/"+reqID+"/accept", "", andrii), http.StatusOK)

	response = send(t, e.handler, http.MethodGet, "/api/v1/me/notifications", "", olha)
	expectStatus(t, response, http.StatusOK)
	var olhaPage notificationPageResponse
	decodeJSON(t, response, &olhaPage)
	if olhaPage.Total != 1 || olhaPage.UnreadCount != 1 || len(olhaPage.Items) != 1 {
		t.Fatalf("Olha notifications = %+v", olhaPage)
	}
	acceptedNotif := olhaPage.Items[0]
	if acceptedNotif.EventType != "REQUEST_RESPONSE" || acceptedNotif.ExchangeID == nil ||
		acceptedNotif.Target.Kind != "EXCHANGE" || acceptedNotif.Target.ID != *acceptedNotif.ExchangeID {
		t.Fatalf("accepted notification = %+v", acceptedNotif)
	}

	// Filter validation and auth errors.
	assertFieldError(t, send(t, e.handler, http.MethodGet, "/api/v1/me/notifications?unreadOnly=maybe", "", olha), "unreadOnly")
	assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/me/notifications?page=0", "", olha), http.StatusBadRequest, "invalid_page")
	assertProblem(t, send(t, e.handler, http.MethodGet, "/api/v1/me/notifications", "", nil), http.StatusUnauthorized, "unauthenticated")
	assertProblem(t, send(t, e.handler, http.MethodPost, "/api/v1/me/notifications/read-all", "", &session{cookie: olha.cookie}),
		http.StatusForbidden, "csrf_invalid")
}
