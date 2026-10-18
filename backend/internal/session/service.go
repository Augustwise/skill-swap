package session

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"
	_ "time/tzdata"

	"skillswap/backend/internal/communication"
	"skillswap/backend/internal/data"
	"skillswap/backend/internal/profile"
	"skillswap/backend/internal/validate"
)

const (
	// PageSize is the number of sessions on one schedule page.
	PageSize = 20
	MaxPage  = 500

	MaxTitleLength        = 255
	MaxDescriptionLength  = 1000
	MaxMeetingPlaceLength = 180
	MaxMeetingURLLength   = 500
	MaxCancelReasonLength = 500

	MinDuration = 15 * time.Minute
	MaxDuration = 240 * time.Minute
)

var (
	formats          = []string{data.FormatOnline, data.FormatOffline}
	scheduleStatuses = []string{
		data.SessionScheduled,
		data.SessionProposed,
		data.SessionCompleted,
		data.SessionCancelled,
		data.ScheduleAll,
	}
)

var (
	ErrInvalidPage           = errors.New("page is out of range")
	ErrExchangeNotFound      = errors.New("exchange was not found or the user does not take part in it")
	ErrSessionNotFound       = errors.New("session was not found or the user does not take part in it")
	ErrExchangeNotActive     = errors.New("exchange is no longer active")
	ErrExchangeBlocked       = errors.New("session actions are blocked because a participant is unavailable or blocked")
	ErrPlanLimitReached      = errors.New("all planned sessions for this teacher are already scheduled or proposed")
	ErrSessionNotSchedulable = errors.New("session can no longer be scheduled")
	ErrSessionNotProposed    = errors.New("session does not have a pending proposal")
	ErrSessionNotCancellable = errors.New("only proposed or scheduled sessions can be cancelled")
	ErrActionNotAllowed      = errors.New("this action is not available for your role in the session proposal")
	ErrSessionInPast         = errors.New("session start time is in the past")
	ErrSessionConflict       = errors.New("one of the participants already has a scheduled session at this time")
	ErrSessionNotConfirmable = errors.New("only scheduled sessions can be confirmed")
	ErrSessionNotFinished    = errors.New("session has not ended yet")
)

type ICommunication interface {
	RecordSessionUpdated(ctx context.Context, event communication.SessionUpdateEvent) error
}

type Service struct {
	exchanges data.IExchangeData
	sessions  data.ISessionData
	tx        data.ITransaction
	comm      ICommunication
	now       func() time.Time
}

func NewService(exchanges data.IExchangeData, sessions data.ISessionData, tx data.ITransaction, comm ...ICommunication) *Service {
	var c ICommunication
	if len(comm) > 0 {
		c = comm[0]
	}
	return &Service{
		exchanges: exchanges,
		sessions:  sessions,
		tx:        tx,
		comm:      c,
		now:       func() time.Time { return time.Now().UTC() },
	}
}

type ExchangeSessions struct {
	Exchange data.Exchange
	Items    []data.Session
}

func (s *Service) ExchangeSessions(ctx context.Context, userID, exchangeID string) (ExchangeSessions, error) {
	if !validate.UUID(exchangeID) {
		return ExchangeSessions{}, ErrExchangeNotFound
	}
	id := strings.ToLower(exchangeID)
	ex, err := s.exchanges.ExchangeByID(ctx, id)
	switch {
	case errors.Is(err, data.ErrNotFound):
		return ExchangeSessions{}, ErrExchangeNotFound
	case err != nil:
		return ExchangeSessions{}, err
	case userID != ex.Requester.UserID && userID != ex.Recipient.UserID:
		return ExchangeSessions{}, ErrExchangeNotFound
	}
	items, err := s.sessions.ExchangeSessions(ctx, ex.ID)
	if err != nil {
		return ExchangeSessions{}, err
	}
	return ExchangeSessions{Exchange: ex, Items: items}, nil
}

type SessionDetails struct {
	Session   data.Session
	Proposals []data.SessionProposal
}

func (s *Service) SessionDetails(ctx context.Context, userID, sessionID string) (SessionDetails, error) {
	if !validate.UUID(sessionID) {
		return SessionDetails{}, ErrSessionNotFound
	}
	found, err := s.sessions.SessionByID(ctx, strings.ToLower(sessionID))
	switch {
	case errors.Is(err, data.ErrNotFound):
		return SessionDetails{}, ErrSessionNotFound
	case err != nil:
		return SessionDetails{}, err
	case userID != found.Requester.UserID && userID != found.Recipient.UserID:
		return SessionDetails{}, ErrSessionNotFound
	}
	proposals, err := s.sessions.SessionProposals(ctx, found.ID)
	if err != nil {
		return SessionDetails{}, err
	}
	return SessionDetails{Session: found, Proposals: proposals}, nil
}

type ProposalInput struct {
	SessionID    string
	Title        string
	Description  string
	TeacherID    string
	StartsAt     string
	EndsAt       string
	Format       string
	MeetingPlace string
	MeetingURL   string
}

type ProposalResult struct {
	Session  data.Session
	Proposal data.SessionProposal
}

// ProposeExchangeSession proposes a lesson on an active exchange. When SessionID is omitted,
// it picks the first available DRAFT lesson slot for the chosen teacher.
func (s *Service) ProposeExchangeSession(ctx context.Context, userID, exchangeID string, in ProposalInput) (ProposalResult, error) {
	if !validate.UUID(exchangeID) {
		return ProposalResult{}, ErrExchangeNotFound
	}
	exID := strings.ToLower(exchangeID)
	if strings.TrimSpace(in.SessionID) != "" {
		if !validate.UUID(in.SessionID) {
			return ProposalResult{}, &profile.ValidationError{Fields: map[string]string{
				"sessionId": "Choose a valid session of this exchange",
			}}
		}
		target, err := s.sessions.SessionByID(ctx, strings.ToLower(in.SessionID))
		switch {
		case errors.Is(err, data.ErrNotFound):
			return ProposalResult{}, ErrSessionNotFound
		case err != nil:
			return ProposalResult{}, err
		case userID != target.Requester.UserID && userID != target.Recipient.UserID:
			return ProposalResult{}, ErrSessionNotFound
		case target.ExchangeID != exID:
			return ProposalResult{}, &profile.ValidationError{Fields: map[string]string{
				"sessionId": "Choose a session that belongs to this exchange",
			}}
		}
		return s.ProposeSession(ctx, userID, target.ID, in)
	}

	var result ProposalResult
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		ex, err := s.exchanges.LockExchange(ctx, exID)
		switch {
		case errors.Is(err, data.ErrNotFound):
			return ErrExchangeNotFound
		case err != nil:
			return err
		case userID != ex.Requester.UserID && userID != ex.Recipient.UserID:
			return ErrExchangeNotFound
		case ex.Status != data.ExchangeActive:
			return ErrExchangeNotActive
		}

		parsed, err := validateProposalInput(in, "", "", ex.Format, ex.Requester.UserID, ex.Recipient.UserID, true, s.now())
		if err != nil {
			return err
		}

		slot, err := s.sessions.FindDraftSessionForTeacher(ctx, ex.ID, parsed.TeacherID)
		switch {
		case errors.Is(err, data.ErrNotFound):
			return ErrPlanLimitReached
		case err != nil:
			return err
		case !slot.CanInteract:
			return ErrExchangeBlocked
		}

		conflict, err := s.sessions.HasScheduleConflict(ctx, slot.ID, ex.Requester.UserID, ex.Recipient.UserID, parsed.StartsAt, parsed.EndsAt)
		if err != nil {
			return err
		}
		if conflict {
			return ErrSessionConflict
		}

		parsed.SessionID = slot.ID
		parsed.ProposerID = userID
		result.Proposal, err = s.sessions.SaveSessionProposal(ctx, parsed)
		if err != nil {
			return err
		}
		result.Session, err = s.sessions.SessionByID(ctx, slot.ID)
		if err != nil {
			return err
		}
		return s.notifySessionProposal(ctx, userID, result.Session, result.Proposal)
	})
	return result, err
}

// ProposeSession proposes or reschedules a specific session (DRAFT, PROPOSED or SCHEDULED).
func (s *Service) ProposeSession(ctx context.Context, userID, sessionID string, in ProposalInput) (ProposalResult, error) {
	if !validate.UUID(sessionID) {
		return ProposalResult{}, ErrSessionNotFound
	}
	id := strings.ToLower(sessionID)

	var result ProposalResult
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		current, err := s.sessions.LockSession(ctx, id)
		switch {
		case errors.Is(err, data.ErrNotFound):
			return ErrSessionNotFound
		case err != nil:
			return err
		case userID != current.Requester.UserID && userID != current.Recipient.UserID:
			return ErrSessionNotFound
		case current.ExchangeStatus != data.ExchangeActive:
			return ErrExchangeNotActive
		case !current.CanInteract:
			return ErrExchangeBlocked
		case current.Status != data.SessionDraft && current.Status != data.SessionProposed && current.Status != data.SessionScheduled:
			return ErrSessionNotSchedulable
		}

		parsed, err := validateProposalInput(
			in,
			current.Title,
			current.Teacher.UserID,
			current.Format,
			current.Requester.UserID,
			current.Recipient.UserID,
			false,
			s.now(),
		)
		if err != nil {
			return err
		}
		if current.Status != data.SessionDraft && parsed.TeacherID != current.Teacher.UserID {
			return &profile.ValidationError{Fields: map[string]string{
				"teacherId": "Teacher cannot be changed for an already proposed or scheduled session",
			}}
		}
		if current.Status == data.SessionDraft && parsed.TeacherID != current.Teacher.UserID {
			err := s.sessions.SwapDraftSessionTeacher(ctx, current.ExchangeID, current.ID, parsed.TeacherID)
			switch {
			case errors.Is(err, data.ErrNotFound):
				return ErrPlanLimitReached
			case err != nil:
				return err
			}
			// Reload default title if the user omitted title and we swapped the draft slot.
			if strings.TrimSpace(in.Title) == "" {
				reloaded, err := s.sessions.SessionByID(ctx, current.ID)
				if err != nil {
					return err
				}
				parsed.Title = reloaded.Title
			}
		}

		conflict, err := s.sessions.HasScheduleConflict(ctx, current.ID, current.Requester.UserID, current.Recipient.UserID, parsed.StartsAt, parsed.EndsAt)
		if err != nil {
			return err
		}
		if conflict {
			return ErrSessionConflict
		}

		parsed.SessionID = current.ID
		parsed.ProposerID = userID
		result.Proposal, err = s.sessions.SaveSessionProposal(ctx, parsed)
		if err != nil {
			return err
		}
		result.Session, err = s.sessions.SessionByID(ctx, current.ID)
		if err != nil {
			return err
		}
		return s.notifySessionProposal(ctx, userID, result.Session, result.Proposal)
	})
	return result, err
}

func (s *Service) AcceptSessionProposal(ctx context.Context, userID, sessionID string) (ProposalResult, error) {
	if !validate.UUID(sessionID) {
		return ProposalResult{}, ErrSessionNotFound
	}
	id := strings.ToLower(sessionID)

	var result ProposalResult
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		current, err := s.sessions.LockSession(ctx, id)
		switch {
		case errors.Is(err, data.ErrNotFound):
			return ErrSessionNotFound
		case err != nil:
			return err
		case userID != current.Requester.UserID && userID != current.Recipient.UserID:
			return ErrSessionNotFound
		}

		// Idempotent repeat: the partner already accepted the current proposal.
		if current.Status == data.SessionScheduled && current.ActiveProposal != nil &&
			current.ActiveProposal.Status == data.ProposalAccepted &&
			current.ActiveProposal.RespondedByID == userID {
			result.Session = current
			result.Proposal = *current.ActiveProposal
			return nil
		}

		switch {
		case current.ExchangeStatus != data.ExchangeActive:
			return ErrExchangeNotActive
		case !current.CanInteract:
			return ErrExchangeBlocked
		case current.Status != data.SessionProposed || current.ActiveProposal == nil || current.ActiveProposal.Status != data.ProposalPending:
			return ErrSessionNotProposed
		case current.ActiveProposal.ProposedByID == userID:
			return ErrActionNotAllowed
		case !current.ActiveProposal.StartsAt.After(s.now()):
			return ErrSessionInPast
		}

		conflict, err := s.sessions.HasScheduleConflict(
			ctx,
			current.ID,
			current.Requester.UserID,
			current.Recipient.UserID,
			current.ActiveProposal.StartsAt,
			current.ActiveProposal.EndsAt,
		)
		if err != nil {
			return err
		}
		if conflict {
			return ErrSessionConflict
		}

		result.Proposal, err = s.sessions.AcceptSessionProposal(ctx, current.ID, userID)
		if err != nil {
			return err
		}
		result.Session, err = s.sessions.SessionByID(ctx, current.ID)
		if err != nil {
			return err
		}
		if s.comm != nil {
			actor := actorParty(result.Session, userID)
			return s.comm.RecordSessionUpdated(ctx, communication.SessionUpdateEvent{
				RecipientID: result.Proposal.ProposedByID,
				ActorID:     userID,
				EventKey:    "session-accepted:" + result.Proposal.ID,
				Title:       "Заняття погоджено",
				Body:        fmt.Sprintf("%s погодив заняття «%s».", fullName(actor.FirstName, actor.LastName), result.Session.Title),
				ExchangeID:  result.Session.ExchangeID,
				SessionID:   result.Session.ID,
			})
		}
		return nil
	})
	return result, err
}

func (s *Service) DeclineSessionProposal(ctx context.Context, userID, sessionID string) (ProposalResult, error) {
	if !validate.UUID(sessionID) {
		return ProposalResult{}, ErrSessionNotFound
	}
	id := strings.ToLower(sessionID)

	var result ProposalResult
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		current, err := s.sessions.LockSession(ctx, id)
		switch {
		case errors.Is(err, data.ErrNotFound):
			return ErrSessionNotFound
		case err != nil:
			return err
		case userID != current.Requester.UserID && userID != current.Recipient.UserID:
			return ErrSessionNotFound
		}

		// Idempotent repeat: the partner already declined the latest proposal.
		if current.Status == data.SessionDraft && current.ActiveProposal != nil &&
			current.ActiveProposal.Status == data.ProposalDeclined &&
			current.ActiveProposal.RespondedByID == userID {
			result.Session = current
			result.Proposal = *current.ActiveProposal
			return nil
		}

		switch {
		case current.ExchangeStatus != data.ExchangeActive:
			return ErrExchangeNotActive
		case current.Status != data.SessionProposed || current.ActiveProposal == nil || current.ActiveProposal.Status != data.ProposalPending:
			return ErrSessionNotProposed
		case current.ActiveProposal.ProposedByID == userID:
			return ErrActionNotAllowed
		}

		result.Proposal, err = s.sessions.DeclineSessionProposal(ctx, current.ID, userID)
		if err != nil {
			return err
		}
		result.Session, err = s.sessions.SessionByID(ctx, current.ID)
		if err != nil {
			return err
		}
		if s.comm != nil {
			actor := actorParty(result.Session, userID)
			return s.comm.RecordSessionUpdated(ctx, communication.SessionUpdateEvent{
				RecipientID: result.Proposal.ProposedByID,
				ActorID:     userID,
				EventKey:    "session-declined:" + result.Proposal.ID,
				Title:       "Пропозицію заняття відхилено",
				Body:        fmt.Sprintf("%s відхилив пропозицію заняття «%s».", fullName(actor.FirstName, actor.LastName), result.Session.Title),
				ExchangeID:  result.Session.ExchangeID,
				SessionID:   result.Session.ID,
			})
		}
		return nil
	})
	return result, err
}

func (s *Service) CancelSession(ctx context.Context, userID, sessionID, rawReason string) (data.Session, error) {
	if !validate.UUID(sessionID) {
		return data.Session{}, ErrSessionNotFound
	}
	reason, ok := validate.Text(rawReason, MaxCancelReasonLength)
	if !ok {
		return data.Session{}, &profile.ValidationError{Fields: map[string]string{
			"reason": "Cancellation reason must be at most 500 characters",
		}}
	}
	id := strings.ToLower(sessionID)

	var cancelled data.Session
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		current, err := s.sessions.LockSession(ctx, id)
		switch {
		case errors.Is(err, data.ErrNotFound):
			return ErrSessionNotFound
		case err != nil:
			return err
		case userID != current.Requester.UserID && userID != current.Recipient.UserID:
			return ErrSessionNotFound
		case current.Status == data.SessionCancelled:
			cancelled = current
			return nil
		case current.ExchangeStatus != data.ExchangeActive:
			return ErrExchangeNotActive
		case current.Status != data.SessionProposed && current.Status != data.SessionScheduled:
			return ErrSessionNotCancellable
		}

		if err := s.sessions.CancelSession(ctx, current.ID, userID, reason); err != nil {
			return err
		}
		cancelled, err = s.sessions.SessionByID(ctx, current.ID)
		if err != nil {
			return err
		}
		if s.comm != nil {
			actor := actorParty(cancelled, userID)
			recipientID := otherPartyID(cancelled, userID)
			return s.comm.RecordSessionUpdated(ctx, communication.SessionUpdateEvent{
				RecipientID: recipientID,
				ActorID:     userID,
				EventKey:    "session-cancelled:" + cancelled.ID,
				Title:       "Заняття скасовано",
				Body:        fmt.Sprintf("%s скасував заняття «%s».", fullName(actor.FirstName, actor.LastName), cancelled.Title),
				ExchangeID:  cancelled.ExchangeID,
				SessionID:   cancelled.ID,
			})
		}
		return nil
	})
	return cancelled, err
}

type CompletionResult struct {
	Session  data.Session
	Exchange data.Exchange
}

// ConfirmSessionCompletion records that the user confirms the session took place. The session
// counts toward the progress only after both participants confirm it.
func (s *Service) ConfirmSessionCompletion(ctx context.Context, userID, sessionID string) (CompletionResult, error) {
	if !validate.UUID(sessionID) {
		return CompletionResult{}, ErrSessionNotFound
	}
	id := strings.ToLower(sessionID)

	var result CompletionResult
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		current, err := s.sessions.LockSession(ctx, id)
		switch {
		case errors.Is(err, data.ErrNotFound):
			return ErrSessionNotFound
		case err != nil:
			return err
		case userID != current.Requester.UserID && userID != current.Recipient.UserID:
			return ErrSessionNotFound
		}

		switch {
		case current.Status == data.SessionCompleted:
			// Both participants have already confirmed it, so a repeat changes nothing.
		case current.ExchangeStatus != data.ExchangeActive:
			return ErrExchangeNotActive
		case !current.CanInteract:
			return ErrExchangeBlocked
		case current.Status != data.SessionScheduled:
			return ErrSessionNotConfirmable
		case current.EndsAt == nil || current.EndsAt.After(s.now()):
			return ErrSessionNotFinished
		case !slices.Contains(current.CompletionConfirmedUserIDs, userID):
			if err := s.sessions.ConfirmSessionCompletion(ctx, current.ID, userID); err != nil {
				return err
			}
			partnerConfirmed := slices.Contains(current.CompletionConfirmedUserIDs, otherPartyID(current, userID))
			if partnerConfirmed {
				if err := s.sessions.CompleteSession(ctx, current.ID); err != nil {
					return err
				}
			}
			if err := s.notifyCompletionConfirmed(ctx, userID, current, partnerConfirmed); err != nil {
				return err
			}
		}

		if result.Session, err = s.sessions.SessionByID(ctx, current.ID); err != nil {
			return err
		}
		result.Exchange, err = s.exchanges.ExchangeByID(ctx, current.ExchangeID)
		return err
	})
	return result, err
}

func (s *Service) notifyCompletionConfirmed(ctx context.Context, userID string, session data.Session, completed bool) error {
	if s.comm == nil {
		return nil
	}
	party := actorParty(session, userID)
	actor := fullName(party.FirstName, party.LastName)
	// Rescheduling clears confirmations and accepts a new proposal, so the next confirmation
	// gets its own notification.
	attempt := session.ID
	if session.ActiveProposal != nil {
		attempt = session.ActiveProposal.ID
	}
	event := communication.SessionUpdateEvent{
		RecipientID: otherPartyID(session, userID),
		ActorID:     userID,
		EventKey:    "session-completion:" + attempt + ":" + userID,
		Title:       "Підтвердіть проведення заняття",
		Body:        fmt.Sprintf("%s підтвердив проведення заняття «%s». Підтвердіть і ви, щоб його зарахувати.", actor, session.Title),
		ExchangeID:  session.ExchangeID,
		SessionID:   session.ID,
	}
	if completed {
		event.EventKey = "session-completed:" + session.ID
		event.Title = "Заняття зараховано"
		event.Body = fmt.Sprintf("%s теж підтвердив проведення заняття «%s», тож його зараховано до прогресу.", actor, session.Title)
	}
	return s.comm.RecordSessionUpdated(ctx, event)
}

type SchedulePage struct {
	Items                 []data.Session
	Page                  int
	Total                 int
	Timezone              string
	PendingProposalsCount int
}

func (s *Service) Schedule(ctx context.Context, userID string, filter data.ScheduleFilter, page int) (SchedulePage, error) {
	if page < 1 || page > MaxPage {
		return SchedulePage{}, ErrInvalidPage
	}
	if filter.Status != "" && !slices.Contains(scheduleStatuses, filter.Status) {
		return SchedulePage{}, &profile.ValidationError{Fields: map[string]string{
			"status": "Status must be SCHEDULED, PROPOSED, COMPLETED, CANCELLED or ALL",
		}}
	}
	if filter.Now.IsZero() {
		filter.Now = s.now()
	}
	items, total, pendingCount, timezone, err := s.sessions.UserSchedule(ctx, userID, filter, PageSize, (page-1)*PageSize)
	if err != nil {
		return SchedulePage{}, err
	}
	return SchedulePage{
		Items:                 items,
		Page:                  page,
		Total:                 total,
		Timezone:              timezone,
		PendingProposalsCount: pendingCount,
	}, nil
}

func (s *Service) notifySessionProposal(ctx context.Context, userID string, session data.Session, proposal data.SessionProposal) error {
	if s.comm == nil {
		return nil
	}
	actor := actorParty(session, userID)
	recipientID := otherPartyID(session, userID)
	return s.comm.RecordSessionUpdated(ctx, communication.SessionUpdateEvent{
		RecipientID: recipientID,
		ActorID:     userID,
		EventKey:    "session-proposal:" + proposal.ID,
		Title:       "Нова пропозиція заняття",
		Body:        fmt.Sprintf("%s пропонує заняття «%s».", fullName(actor.FirstName, actor.LastName), session.Title),
		ExchangeID:  session.ExchangeID,
		SessionID:   session.ID,
	})
}

func validateProposalInput(
	in ProposalInput,
	defaultTitle string,
	defaultTeacherID string,
	defaultFormat string,
	requesterID string,
	recipientID string,
	requireTitleAndTeacher bool,
	now time.Time,
) (data.NewSessionProposal, error) {
	fields := map[string]string{}
	var out data.NewSessionProposal

	rawTitle := strings.TrimSpace(in.Title)
	if rawTitle == "" && !requireTitleAndTeacher {
		out.Title = defaultTitle
	} else {
		title, ok := validate.Line(in.Title, MaxTitleLength)
		if !ok || title == "" {
			fields["title"] = "Title must be between 1 and 255 characters"
		} else {
			out.Title = title
		}
	}

	desc, ok := validate.Text(in.Description, MaxDescriptionLength)
	if !ok {
		fields["description"] = "Description must be at most 1000 characters"
	} else {
		out.Description = desc
	}

	rawTeacher := strings.TrimSpace(in.TeacherID)
	if rawTeacher == "" && !requireTitleAndTeacher && defaultTeacherID != "" {
		out.TeacherID = defaultTeacherID
	} else if !validate.UUID(rawTeacher) {
		fields["teacherId"] = "Choose a teacher from the exchange participants"
	} else {
		teacherID := strings.ToLower(rawTeacher)
		if teacherID != requesterID && teacherID != recipientID {
			fields["teacherId"] = "Teacher must be one of the exchange participants"
		} else {
			out.TeacherID = teacherID
		}
	}
	if out.TeacherID == requesterID {
		out.LearnerID = recipientID
	} else if out.TeacherID == recipientID {
		out.LearnerID = requesterID
	}

	startsAt, startOK := parseTimestamp(in.StartsAt)
	if !startOK {
		fields["startsAt"] = "startsAt must be an ISO-8601 timestamp with timezone"
	}
	endsAt, endOK := parseTimestamp(in.EndsAt)
	if !endOK {
		fields["endsAt"] = "endsAt must be an ISO-8601 timestamp with timezone"
	}
	if startOK && !startsAt.After(now) {
		fields["startsAt"] = "Start time must be in the future"
	}
	if startOK && endOK {
		if !endsAt.After(startsAt) {
			fields["endsAt"] = "End time must be after start time"
		} else if d := endsAt.Sub(startsAt); d < MinDuration || d > MaxDuration {
			fields["endsAt"] = "Session duration must be between 15 and 240 minutes"
		}
	}
	out.StartsAt = startsAt
	out.EndsAt = endsAt

	format := strings.TrimSpace(in.Format)
	if format == "" {
		format = defaultFormat
	}
	if !slices.Contains(formats, format) {
		fields["format"] = "Format must be ONLINE or OFFLINE"
	} else {
		out.Format = format
		switch format {
		case data.FormatOnline:
			link, ok := validMeetingURL(in.MeetingURL)
			if !ok {
				fields["meetingUrl"] = "Provide a valid http or https meeting link up to 500 characters"
			} else {
				out.MeetingURL = link
			}
			if strings.TrimSpace(in.MeetingPlace) != "" {
				fields["meetingPlace"] = "Meeting place is only used for offline lessons"
			}
		case data.FormatOffline:
			place, ok := validate.Line(in.MeetingPlace, MaxMeetingPlaceLength)
			if !ok || place == "" {
				fields["meetingPlace"] = "Meeting place is required for offline lessons (1 to 180 characters)"
			} else {
				out.MeetingPlace = place
			}
			if strings.TrimSpace(in.MeetingURL) != "" {
				fields["meetingUrl"] = "Meeting link is only used for online lessons"
			}
		}
	}

	if len(fields) > 0 {
		return data.NewSessionProposal{}, &profile.ValidationError{Fields: fields}
	}
	return out, nil
}

func parseTimestamp(raw string) (time.Time, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, false
	}
	return parsed.UTC(), true
}

func validMeetingURL(raw string) (string, bool) {
	value, ok := validate.Line(raw, MaxMeetingURLLength)
	if !ok || value == "" || strings.ContainsAny(value, " \t\r\n") {
		return "", false
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return "", false
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", false
	}
	return value, true
}

func actorParty(s data.Session, userID string) data.RequestParty {
	if userID == s.Recipient.UserID {
		return s.Recipient
	}
	return s.Requester
}

func otherPartyID(s data.Session, userID string) string {
	if userID == s.Recipient.UserID {
		return s.Requester.UserID
	}
	return s.Recipient.UserID
}

func fullName(firstName, lastName string) string {
	return strings.TrimSpace(firstName + " " + lastName)
}
