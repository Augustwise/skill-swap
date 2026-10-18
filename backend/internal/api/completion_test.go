package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"skillswap/backend/internal/data"
	"skillswap/backend/internal/data/datatest"
)

type exchangeDetailsResponse struct {
	Exchange exchangeResponse       `json:"exchange"`
	Sessions []sessionResponse      `json:"sessions"`
	History  []exchangeStatusChange `json:"history"`
}

type confirmationResponse struct {
	Session  sessionResponse  `json:"session"`
	Exchange exchangeResponse `json:"exchange"`
}

type exchangeEnvelope struct {
	Exchange exchangeResponse `json:"exchange"`
}

// expectEqual stops the test with a readable message when a value is not the expected one.
func expectEqual[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}

// valueOf returns the value behind a nullable response field, or the zero value for null.
func valueOf[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

// sendAndDecode sends a request, checks the status and decodes the JSON response.
func sendAndDecode[T any](t *testing.T, e *discoveryEnv, method, path, body string, s *session, status int) T {
	t.Helper()
	response := send(t, e.handler, method, path, body, s)
	expectStatus(t, response, status)
	var decoded T
	decodeJSON(t, response, &decoded)
	return decoded
}

func getExchangeDetails(t *testing.T, e *discoveryEnv, exchangeID string, s *session) exchangeDetailsResponse {
	t.Helper()
	return sendAndDecode[exchangeDetailsResponse](t, e, http.MethodGet, "/api/v1/exchanges/"+exchangeID, "", s, http.StatusOK)
}

// scheduleLesson lets Olha propose a guitar lesson for tomorrow and Andrii accept it.
func scheduleLesson(t *testing.T, e *discoveryEnv, exchangeID, olhaID string, olha, andrii *session) string {
	t.Helper()
	start := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Minute)
	body := fmt.Sprintf(`{"title":"Акорди","teacherId":%q,"startsAt":%q,"endsAt":%q,
		"format":"ONLINE","meetingUrl":"https://meet.example.test/guitar"}`,
		olhaID, start.Format(time.RFC3339), start.Add(time.Hour).Format(time.RFC3339))
	proposed := sendAndDecode[struct {
		Session sessionResponse `json:"session"`
	}](t, e, http.MethodPost, "/api/v1/exchanges/"+exchangeID+"/sessions", body, olha, http.StatusCreated)
	expectStatus(t, send(t, e.handler, http.MethodPost, "/api/v1/sessions/"+proposed.Session.ID+"/accept", "", andrii), http.StatusOK)
	return proposed.Session.ID
}

func TestConfirmSessionRoute(t *testing.T) {
	e, olha, andrii, olhaID, _, exchangeID := newExchangeEnv(t)
	marko := e.student(t, "marko@students.example.test", `["ONLINE"]`, "Київ", datatest.PhotoshopSkillID, datatest.GuitarSkillID)
	lessonID := scheduleLesson(t, e, exchangeID, olhaID, olha, andrii)
	confirmPath := "/api/v1/sessions/" + lessonID + "/confirm"

	assertProblem(t, send(t, e.handler, http.MethodPost, confirmPath, "", olha), http.StatusConflict, "session_not_finished")

	// The lesson ended two hours ago.
	ended := time.Now().UTC().Add(-2 * time.Hour)
	e.store.SetSessionTime(lessonID, ended.Add(-time.Hour), ended)

	before := getExchangeDetails(t, e, exchangeID, olha)
	expectEqual(t, "sessions awaiting confirmation", before.Exchange.Progress.AwaitingConfirmationSessions, 1)
	expectEqual(t, "Olha can confirm the lesson", before.Sessions[0].CanConfirmCompletion, true)

	draftPath := "/api/v1/sessions/" + before.Sessions[1].ID + "/confirm"
	assertProblem(t, send(t, e.handler, http.MethodPost, confirmPath, "", marko), http.StatusNotFound, "session_not_found")
	assertProblem(t, send(t, e.handler, http.MethodPost, draftPath, "", olha), http.StatusConflict, "session_not_confirmable")

	// Olha confirms first: the lesson waits for Andrii.
	byOlha := sendAndDecode[confirmationResponse](t, e, http.MethodPost, confirmPath, "", olha, http.StatusOK)
	expectEqual(t, "status after Olha's confirmation", byOlha.Session.Status, data.SessionScheduled)
	expectEqual(t, "confirmed by Olha", byOlha.Session.CompletionConfirmedByMe, true)
	expectEqual(t, "confirmed by Andrii", byOlha.Session.CompletionConfirmedByPartner, false)
	expectEqual(t, "Olha can confirm again", byOlha.Session.CanConfirmCompletion, false)
	expectEqual(t, "completed sessions after Olha's confirmation", byOlha.Exchange.Progress.CompletedSessions, 0)

	// Andrii confirms too: the lesson is completed and counts toward the progress.
	byAndrii := sendAndDecode[confirmationResponse](t, e, http.MethodPost, confirmPath, "", andrii, http.StatusOK)
	expectEqual(t, "status after both confirmations", byAndrii.Session.Status, data.SessionCompleted)
	expectEqual(t, "completedAt is set", byAndrii.Session.CompletedAt != nil, true)
	expectEqual(t, "confirmed by Olha, as Andrii sees it", byAndrii.Session.CompletionConfirmedByPartner, true)
	expectEqual(t, "completed sessions after both confirmations", byAndrii.Exchange.Progress.CompletedSessions, 1)
	expectEqual(t, "sessions awaiting confirmation", byAndrii.Exchange.Progress.AwaitingConfirmationSessions, 0)
	expectEqual(t, "plan completed", byAndrii.Exchange.Progress.PlanCompleted, false)
}

func TestCompleteExchangeRoute(t *testing.T) {
	e, olha, andrii, _, _, exchangeID := newExchangeEnv(t)
	completePath := "/api/v1/exchanges/" + exchangeID + "/complete"

	assertProblem(t, send(t, e.handler, http.MethodPost, completePath, "", olha), http.StatusConflict, "plan_not_completed")

	// All three planned lessons took place and were confirmed.
	for _, s := range getExchangeDetails(t, e, exchangeID, olha).Sessions {
		e.store.SetSessionStatus(s.ID, data.SessionCompleted)
	}
	before := getExchangeDetails(t, e, exchangeID, andrii).Exchange
	expectEqual(t, "plan completed", before.Progress.PlanCompleted, true)
	expectEqual(t, "percent", before.Progress.Percent, 100)
	expectEqual(t, "Andrii can confirm the end", before.CanConfirmCompletion, true)

	// Olha confirms the end first: the exchange waits for Andrii.
	byOlha := sendAndDecode[exchangeEnvelope](t, e, http.MethodPost, completePath, "", olha, http.StatusOK).Exchange
	expectEqual(t, "status after Olha's confirmation", byOlha.Status, data.ExchangeActive)
	expectEqual(t, "confirmed by Olha", byOlha.CompletionConfirmedByMe, true)
	expectEqual(t, "Olha can confirm again", byOlha.CanConfirmCompletion, false)

	// Andrii confirms too: the exchange is completed.
	byAndrii := sendAndDecode[exchangeEnvelope](t, e, http.MethodPost, completePath, "", andrii, http.StatusOK).Exchange
	expectEqual(t, "status after both confirmations", byAndrii.Status, data.ExchangeCompleted)
	expectEqual(t, "completedAt is set", byAndrii.CompletedAt != nil, true)
	expectEqual(t, "confirmed by Olha, as Andrii sees it", byAndrii.CompletionConfirmedByPartner, true)
	expectEqual(t, "a completed exchange can be cancelled", byAndrii.CanCancel, false)

	history := getExchangeDetails(t, e, exchangeID, olha).History
	expectEqual(t, "history items", len(history), 2)
	expectEqual(t, "last history status", history[1].Status, data.ExchangeCompleted)
	expectEqual(t, "completion has an author", history[1].ChangedBy != nil, true)

	cancelPath := "/api/v1/exchanges/" + exchangeID + "/cancel"
	assertProblem(t, send(t, e.handler, http.MethodPost, cancelPath, `{"reason":"Пізно"}`, olha), http.StatusConflict, "exchange_not_active")
	assertProblem(t, send(t, e.handler, http.MethodGet, completePath, "", olha), http.StatusMethodNotAllowed, "method_not_allowed")
}

func TestCancelExchangeRoute(t *testing.T) {
	e, olha, andrii, _, andriiID, exchangeID := newExchangeEnv(t)
	marko := e.student(t, "marko@students.example.test", `["ONLINE"]`, "Київ", datatest.PhotoshopSkillID, datatest.GuitarSkillID)
	cancelPath := "/api/v1/exchanges/" + exchangeID + "/cancel"

	assertProblem(t, send(t, e.handler, http.MethodPost, cancelPath, "", andrii), http.StatusUnsupportedMediaType, "unsupported_media_type")
	assertProblem(t, send(t, e.handler, http.MethodPost, cancelPath, `{"reason":"  "}`, andrii), http.StatusUnprocessableEntity, "validation_failed")
	assertProblem(t, send(t, e.handler, http.MethodPost, cancelPath, `{"reason":"Причина"}`, marko), http.StatusNotFound, "exchange_not_found")

	cancelled := sendAndDecode[exchangeEnvelope](t, e, http.MethodPost, cancelPath, `{"reason":"Змінився розклад"}`, andrii, http.StatusOK).Exchange
	expectEqual(t, "status", cancelled.Status, data.ExchangeCancelled)
	expectEqual(t, "cancelledAt is set", cancelled.CancelledAt != nil, true)
	expectEqual(t, "cancellation reason", valueOf(cancelled.CancellationReason), "Змінився розклад")
	expectEqual(t, "a cancelled exchange can be cancelled", cancelled.CanCancel, false)
	expectEqual(t, "cancelled sessions", cancelled.Progress.CancelledSessions, 3)

	// A repeated cancellation returns the exchange as it is.
	expectStatus(t, send(t, e.handler, http.MethodPost, cancelPath, `{"reason":"Ще раз"}`, olha), http.StatusOK)

	details := getExchangeDetails(t, e, exchangeID, olha)
	last := details.History[len(details.History)-1]
	expectEqual(t, "last history status", last.Status, data.ExchangeCancelled)
	expectEqual(t, "cancelled by", valueOf(last.ChangedBy).ID, andriiID)
	expectEqual(t, "reason in history", valueOf(last.Reason), "Змінився розклад")
	for _, s := range details.Sessions {
		expectEqual(t, "session status after cancellation", s.Status, data.SessionCancelled)
	}

	notifications := sendAndDecode[notificationPageResponse](t, e, http.MethodGet, "/api/v1/me/notifications", "", olha, http.StatusOK)
	expectEqual(t, "Olha has notifications", len(notifications.Items) > 0, true)
	expectEqual(t, "newest notification opens", notifications.Items[0].Target.Kind, "EXCHANGE")
	expectEqual(t, "newest notification exchange", notifications.Items[0].Target.ID, exchangeID)
}
