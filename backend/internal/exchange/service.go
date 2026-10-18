package exchange

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"skillswap/backend/internal/communication"
	"skillswap/backend/internal/data"
	"skillswap/backend/internal/profile"
	"skillswap/backend/internal/validate"
)

const (
	MaxSessions = 20
	MinDuration = 15
	MaxDuration = 240

	// PageSize is the number of requests on one page.
	PageSize = 20
	MaxPage  = 500

	maxMessageLength      = 500
	maxCancelReasonLength = 500
)

var (
	formats          = []string{data.FormatOnline, data.FormatOffline}
	statuses         = []string{data.RequestPending, data.RequestAccepted, data.RequestDeclined, data.RequestWithdrawn}
	exchangeStatuses = []string{data.ExchangeActive, data.ExchangeCompleted, data.ExchangeCancelled}
)

var (
	ErrStudentNotFound    = errors.New("student was not found or is not visible")
	ErrRequestsClosed     = errors.New("student does not accept requests")
	ErrSameUniversityOnly = errors.New("student accepts requests only from their university")
	ErrDuplicateRequest   = errors.New("a pending request for this pair of skills already exists")
	ErrInvalidPage        = errors.New("page is out of range")
	ErrRequestNotFound    = errors.New("request was not found or the user does not take part in it")
	ErrActionNotAllowed   = errors.New("this action is not available for the user's role in the request")
	ErrRequestNotPending  = errors.New("request has already been answered")
	ErrRequestOutdated    = errors.New("request can no longer be accepted")
	ErrExchangeNotFound   = errors.New("exchange was not found or the user does not take part in it")
	ErrExchangeNotActive  = errors.New("exchange is no longer active")
	ErrExchangeBlocked    = errors.New("exchange actions are blocked because a participant is unavailable or blocked")
	ErrPlanNotCompleted   = errors.New("not all planned sessions have been confirmed in both directions")
)

type ICommunication interface {
	RecordRequestCreated(ctx context.Context, request data.ExchangeRequest) error
	RecordRequestAccepted(ctx context.Context, request data.ExchangeRequest, exchangeID string) error
	RecordRequestDeclined(ctx context.Context, request data.ExchangeRequest) error
	RecordSessionUpdated(ctx context.Context, event communication.SessionUpdateEvent) error
}

type Service struct {
	profiles  data.IProfileData
	exchanges data.IExchangeData
	tx        data.ITransaction
	comm      ICommunication
}

func NewService(profiles data.IProfileData, exchanges data.IExchangeData, tx data.ITransaction, comm ...ICommunication) *Service {
	var c ICommunication
	if len(comm) > 0 {
		c = comm[0]
	}
	return &Service{profiles: profiles, exchanges: exchanges, tx: tx, comm: c}
}

type NewRequest struct {
	RecipientID          string
	TeachSkillID         string
	LearnSkillID         string
	Format               string
	TeachSessions        int
	TeachDurationMinutes int
	LearnSessions        int
	LearnDurationMinutes int
	Message              string
}

func (s *Service) CreateRequest(ctx context.Context, userID string, in NewRequest) (data.ExchangeRequest, error) {
	request, err := validRequest(userID, in)
	if err != nil {
		return data.ExchangeRequest{}, err
	}
	sender, err := s.profiles.ProfileByUserID(ctx, userID)
	if err != nil {
		return data.ExchangeRequest{}, err
	}
	recipient, err := s.exchanges.RequestRecipient(ctx, userID, request.RecipientID)
	switch {
	case errors.Is(err, data.ErrNotFound):
		return data.ExchangeRequest{}, ErrStudentNotFound
	case err != nil:
		return data.ExchangeRequest{}, err
	case !recipient.AcceptsRequests:
		return data.ExchangeRequest{}, ErrRequestsClosed
	case recipient.SameUniversityOnly && recipient.UniversityID != sender.UniversityID:
		return data.ExchangeRequest{}, ErrSameUniversityOnly
	}
	if fields := termsMismatch(sender, recipient.Profile, request); len(fields) > 0 {
		return data.ExchangeRequest{}, &profile.ValidationError{Fields: fields}
	}

	var created data.ExchangeRequest
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		id, err := s.exchanges.CreateRequest(ctx, request)
		switch {
		case errors.Is(err, data.ErrDuplicateRequest):
			return ErrDuplicateRequest
		case errors.Is(err, data.ErrNotFound):
			// A skill was removed after the checks above.
			return &profile.ValidationError{Fields: map[string]string{
				"teachSkillId": "Skill lists have changed; reload the profile and try again",
			}}
		case err != nil:
			return err
		}
		created, err = s.exchanges.RequestByID(ctx, id)
		if err != nil {
			return err
		}
		if s.comm != nil {
			return s.comm.RecordRequestCreated(ctx, created)
		}
		return nil
	})
	return created, err
}

type RequestPage struct {
	Items []data.ExchangeRequest
	Page  int
	Total int
}

func (s *Service) Requests(ctx context.Context, userID string, filter data.RequestFilter, page int) (RequestPage, error) {
	if page < 1 || page > MaxPage {
		return RequestPage{}, ErrInvalidPage
	}
	fields := map[string]string{}
	if filter.Direction != data.Incoming && filter.Direction != data.Outgoing {
		fields["direction"] = "Direction must be incoming or outgoing"
	}
	if filter.Status != "" && !slices.Contains(statuses, filter.Status) {
		fields["status"] = "Status must be PENDING, ACCEPTED, DECLINED or WITHDRAWN"
	}
	if len(fields) > 0 {
		return RequestPage{}, &profile.ValidationError{Fields: fields}
	}
	items, total, err := s.exchanges.UserRequests(ctx, userID, filter, PageSize, (page-1)*PageSize)
	if err != nil {
		return RequestPage{}, err
	}
	return RequestPage{Items: items, Page: page, Total: total}, nil
}

type RequestDetails struct {
	Request data.ExchangeRequest
	History []data.RequestStatusChange
}

// RequestDetails stays available to both participants even after one of them hides
// the profile or blocks the other.
func (s *Service) RequestDetails(ctx context.Context, userID, requestID string) (RequestDetails, error) {
	if !validate.UUID(requestID) {
		return RequestDetails{}, ErrRequestNotFound
	}
	request, err := s.exchanges.RequestByID(ctx, strings.ToLower(requestID))
	switch {
	case errors.Is(err, data.ErrNotFound):
		return RequestDetails{}, ErrRequestNotFound
	case err != nil:
		return RequestDetails{}, err
	case userID != request.Requester.UserID && userID != request.Recipient.UserID:
		return RequestDetails{}, ErrRequestNotFound
	}
	history, err := s.exchanges.RequestHistory(ctx, request.ID)
	if err != nil {
		return RequestDetails{}, err
	}
	return RequestDetails{Request: request, History: history}, nil
}

type role int

const (
	requester role = iota
	recipient
)

func (s *Service) DeclineRequest(ctx context.Context, userID, requestID string) (data.ExchangeRequest, error) {
	return s.answerRequest(ctx, userID, requestID, recipient, data.RequestDeclined)
}

func (s *Service) WithdrawRequest(ctx context.Context, userID, requestID string) (data.ExchangeRequest, error) {
	return s.answerRequest(ctx, userID, requestID, requester, data.RequestWithdrawn)
}

func (s *Service) answerRequest(ctx context.Context, userID, requestID string, actor role, status string) (data.ExchangeRequest, error) {
	var answered data.ExchangeRequest
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		request, err := s.lockRequest(ctx, userID, requestID, actor)
		if err != nil {
			return err
		}
		switch request.Status {
		case status:
			// The same action repeated, e.g. after a lost response, changes nothing.
		case data.RequestPending:
			if err := s.exchanges.UpdateRequestStatus(ctx, request.ID, status, userID); err != nil {
				return err
			}
		default:
			return ErrRequestNotPending
		}
		answered, err = s.exchanges.RequestByID(ctx, request.ID)
		if err != nil {
			return err
		}
		if status == data.RequestDeclined && s.comm != nil {
			return s.comm.RecordRequestDeclined(ctx, answered)
		}
		return nil
	})
	return answered, err
}

type Acceptance struct {
	Request  data.ExchangeRequest
	Exchange data.Exchange
}

// AcceptRequest returns the same exchange when the acceptance is repeated. A request that
// is no longer current stays pending, so its author can still withdraw it.
func (s *Service) AcceptRequest(ctx context.Context, userID, requestID string) (Acceptance, error) {
	var accepted Acceptance
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		request, err := s.lockRequest(ctx, userID, requestID, recipient)
		if err != nil {
			return err
		}
		switch request.Status {
		case data.RequestAccepted:
		case data.RequestPending:
			current, err := s.exchanges.RequestIsCurrent(ctx, request.ID)
			if err != nil {
				return err
			}
			if !current {
				return ErrRequestOutdated
			}
			if err := s.exchanges.UpdateRequestStatus(ctx, request.ID, data.RequestAccepted, userID); err != nil {
				return err
			}
			if _, err := s.exchanges.CreateExchange(ctx, request.ID, userID); err != nil {
				return err
			}
		default:
			return ErrRequestNotPending
		}
		if accepted.Request, err = s.exchanges.RequestByID(ctx, request.ID); err != nil {
			return err
		}
		if accepted.Exchange, err = s.exchanges.ExchangeByID(ctx, accepted.Request.ExchangeID); err != nil {
			return err
		}
		if s.comm != nil {
			return s.comm.RecordRequestAccepted(ctx, accepted.Request, accepted.Exchange.ID)
		}
		return nil
	})
	return accepted, err
}

type ExchangeDetails struct {
	Exchange data.Exchange
	History  []data.ExchangeStatusChange
}

func (s *Service) ExchangeDetails(ctx context.Context, userID, exchangeID string) (ExchangeDetails, error) {
	if !validate.UUID(exchangeID) {
		return ExchangeDetails{}, ErrExchangeNotFound
	}
	exchange, err := s.exchanges.ExchangeByID(ctx, strings.ToLower(exchangeID))
	switch {
	case errors.Is(err, data.ErrNotFound):
		return ExchangeDetails{}, ErrExchangeNotFound
	case err != nil:
		return ExchangeDetails{}, err
	case userID != exchange.Requester.UserID && userID != exchange.Recipient.UserID:
		return ExchangeDetails{}, ErrExchangeNotFound
	}
	history, err := s.exchanges.ExchangeHistory(ctx, exchange.ID)
	if err != nil {
		return ExchangeDetails{}, err
	}
	return ExchangeDetails{Exchange: exchange, History: history}, nil
}

// CompleteExchange records the final confirmation of the user. The exchange is completed once
// the plan is done in both directions and both participants have confirmed.
func (s *Service) CompleteExchange(ctx context.Context, userID, exchangeID string) (data.Exchange, error) {
	var completed data.Exchange
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		current, err := s.lockExchange(ctx, userID, exchangeID)
		if err != nil {
			return err
		}
		switch {
		case current.Status == data.ExchangeCompleted:
			// Both participants have already confirmed it, so a repeat changes nothing.
		case current.Status != data.ExchangeActive:
			return ErrExchangeNotActive
		case !current.CanInteract:
			return ErrExchangeBlocked
		case !current.Progress.PlanCompleted():
			return ErrPlanNotCompleted
		case !slices.Contains(current.ConfirmedUserIDs, userID):
			if err := s.exchanges.ConfirmExchangeCompletion(ctx, current.ID, userID); err != nil {
				return err
			}
			partnerConfirmed := slices.Contains(current.ConfirmedUserIDs, partnerID(current, userID))
			if partnerConfirmed {
				if err := s.exchanges.CompleteExchange(ctx, current.ID, userID); err != nil {
					return err
				}
			}
			if err := s.notifyCompletion(ctx, userID, current, partnerConfirmed); err != nil {
				return err
			}
		}
		completed, err = s.exchanges.ExchangeByID(ctx, current.ID)
		return err
	})
	return completed, err
}

// CancelExchange keeps the history and completed sessions, and cancels the sessions that have
// not taken place yet.
func (s *Service) CancelExchange(ctx context.Context, userID, exchangeID, rawReason string) (data.Exchange, error) {
	if !validate.UUID(exchangeID) {
		return data.Exchange{}, ErrExchangeNotFound
	}
	reason, ok := validate.Text(rawReason, maxCancelReasonLength)
	if !ok || reason == "" {
		return data.Exchange{}, &profile.ValidationError{Fields: map[string]string{
			"reason": "Give a reason of 1 to 500 characters",
		}}
	}
	var cancelled data.Exchange
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		current, err := s.lockExchange(ctx, userID, exchangeID)
		if err != nil {
			return err
		}
		switch current.Status {
		case data.ExchangeCancelled:
			// The same action repeated, e.g. after a lost response, changes nothing.
		case data.ExchangeActive:
			if err := s.exchanges.CancelExchange(ctx, current.ID, userID, reason); err != nil {
				return err
			}
			if err := s.notifyCancellation(ctx, userID, current, reason); err != nil {
				return err
			}
		default:
			return ErrExchangeNotActive
		}
		cancelled, err = s.exchanges.ExchangeByID(ctx, current.ID)
		return err
	})
	return cancelled, err
}

type ExchangePage struct {
	Items []data.Exchange
	Page  int
	Total int
}

func (s *Service) Exchanges(ctx context.Context, userID string, filter data.ExchangeFilter, page int) (ExchangePage, error) {
	if page < 1 || page > MaxPage {
		return ExchangePage{}, ErrInvalidPage
	}
	if filter.Status != "" && !slices.Contains(exchangeStatuses, filter.Status) {
		return ExchangePage{}, &profile.ValidationError{Fields: map[string]string{
			"status": "Status must be ACTIVE, COMPLETED or CANCELLED",
		}}
	}
	items, total, err := s.exchanges.UserExchanges(ctx, userID, filter, PageSize, (page-1)*PageSize)
	if err != nil {
		return ExchangePage{}, err
	}
	return ExchangePage{Items: items, Page: page, Total: total}, nil
}

// lockRequest must run inside a transaction: the row stays locked until it ends.
func (s *Service) lockRequest(ctx context.Context, userID, requestID string, actor role) (data.RequestState, error) {
	if !validate.UUID(requestID) {
		return data.RequestState{}, ErrRequestNotFound
	}
	request, err := s.exchanges.LockRequest(ctx, strings.ToLower(requestID))
	switch {
	case errors.Is(err, data.ErrNotFound):
		return data.RequestState{}, ErrRequestNotFound
	case err != nil:
		return data.RequestState{}, err
	case userID != request.RequesterID && userID != request.RecipientID:
		return data.RequestState{}, ErrRequestNotFound
	case actor == requester && userID != request.RequesterID, actor == recipient && userID != request.RecipientID:
		return data.RequestState{}, ErrActionNotAllowed
	}
	return request, nil
}

// lockExchange must run inside a transaction: the rows stay locked until it ends.
func (s *Service) lockExchange(ctx context.Context, userID, exchangeID string) (data.Exchange, error) {
	if !validate.UUID(exchangeID) {
		return data.Exchange{}, ErrExchangeNotFound
	}
	exchange, err := s.exchanges.LockExchange(ctx, strings.ToLower(exchangeID))
	switch {
	case errors.Is(err, data.ErrNotFound):
		return data.Exchange{}, ErrExchangeNotFound
	case err != nil:
		return data.Exchange{}, err
	case userID != exchange.Requester.UserID && userID != exchange.Recipient.UserID:
		return data.Exchange{}, ErrExchangeNotFound
	}
	return exchange, nil
}

func (s *Service) notifyCompletion(ctx context.Context, userID string, e data.Exchange, completed bool) error {
	if s.comm == nil {
		return nil
	}
	event := communication.SessionUpdateEvent{
		RecipientID: partnerID(e, userID),
		ActorID:     userID,
		EventKey:    "exchange-completion:" + e.ID + ":" + userID,
		Title:       "Підтвердіть завершення обміну",
		Body:        fmt.Sprintf("%s пропонує завершити обмін «%s».", actorName(e, userID), skillPair(e)),
		ExchangeID:  e.ID,
	}
	if completed {
		event.EventKey = "exchange-completed:" + e.ID
		event.Title = "Обмін завершено"
		event.Body = fmt.Sprintf("%s теж підтвердив завершення обміну «%s».", actorName(e, userID), skillPair(e))
	}
	return s.comm.RecordSessionUpdated(ctx, event)
}

func (s *Service) notifyCancellation(ctx context.Context, userID string, e data.Exchange, reason string) error {
	if s.comm == nil {
		return nil
	}
	return s.comm.RecordSessionUpdated(ctx, communication.SessionUpdateEvent{
		RecipientID: partnerID(e, userID),
		ActorID:     userID,
		EventKey:    "exchange-cancelled:" + e.ID,
		Title:       "Обмін скасовано",
		Body:        fmt.Sprintf("%s скасував обмін «%s». Причина: %s", actorName(e, userID), skillPair(e), reason),
		ExchangeID:  e.ID,
	})
}

func partnerID(e data.Exchange, userID string) string {
	if userID == e.Requester.UserID {
		return e.Recipient.UserID
	}
	return e.Requester.UserID
}

func actorName(e data.Exchange, userID string) string {
	actor := e.Requester
	if userID == e.Recipient.UserID {
		actor = e.Recipient
	}
	return strings.TrimSpace(actor.FirstName + " " + actor.LastName)
}

func skillPair(e data.Exchange) string {
	return e.RequesterTeaches.Name + " ↔ " + e.RecipientTeaches.Name
}

func validRequest(userID string, in NewRequest) (data.NewRequest, error) {
	fields := map[string]string{}
	out := data.NewRequest{
		RequesterID: userID, RecipientID: strings.ToLower(in.RecipientID),
		TeachSkillID: strings.ToLower(in.TeachSkillID), LearnSkillID: strings.ToLower(in.LearnSkillID),
		Format:        in.Format,
		TeachSessions: in.TeachSessions, TeachDurationMinutes: in.TeachDurationMinutes,
		LearnSessions: in.LearnSessions, LearnDurationMinutes: in.LearnDurationMinutes,
	}
	switch {
	case !validate.UUID(out.RecipientID):
		fields["recipientId"] = "Choose a student"
	case out.RecipientID == userID:
		fields["recipientId"] = "You cannot send a request to yourself"
	}
	if !validate.UUID(out.TeachSkillID) {
		fields["teachSkillId"] = "Choose a skill you teach"
	}
	switch {
	case !validate.UUID(out.LearnSkillID):
		fields["learnSkillId"] = "Choose a skill you want to learn"
	case out.LearnSkillID == out.TeachSkillID:
		fields["learnSkillId"] = "Choose two different skills"
	}
	if !slices.Contains(formats, out.Format) {
		fields["format"] = "Format must be ONLINE or OFFLINE"
	}
	for field, value := range map[string]int{"teachSessions": out.TeachSessions, "learnSessions": out.LearnSessions} {
		if value < 1 || value > MaxSessions {
			fields[field] = "Sessions must be between 1 and 20"
		}
	}
	for field, value := range map[string]int{
		"teachDurationMinutes": out.TeachDurationMinutes, "learnDurationMinutes": out.LearnDurationMinutes,
	} {
		if value < MinDuration || value > MaxDuration {
			fields[field] = "Duration must be between 15 and 240 minutes"
		}
	}
	if len(fields) == 0 && out.TeachSessions*out.TeachDurationMinutes != out.LearnSessions*out.LearnDurationMinutes {
		fields["learnSessions"] = "Both directions must have the same total time"
	}
	var ok bool
	if out.Message, ok = validate.Text(in.Message, maxMessageLength); !ok {
		fields["message"] = "Message must be at most 500 characters"
	}
	if len(fields) > 0 {
		return data.NewRequest{}, &profile.ValidationError{Fields: fields}
	}
	return out, nil
}

func termsMismatch(sender, recipient data.Profile, r data.NewRequest) map[string]string {
	fields := map[string]string{}
	switch {
	case !hasSkill(sender.TeachingSkills, r.TeachSkillID):
		fields["teachSkillId"] = `Choose a skill from your "I teach" list`
	case !hasSkill(recipient.LearningSkills, r.TeachSkillID):
		fields["teachSkillId"] = "The student does not want to learn this skill"
	}
	switch {
	case !hasSkill(recipient.TeachingSkills, r.LearnSkillID):
		fields["learnSkillId"] = "The student does not teach this skill"
	case !hasSkill(sender.LearningSkills, r.LearnSkillID):
		fields["learnSkillId"] = `Choose a skill from your "I learn" list`
	}
	switch {
	case !slices.Contains(sender.Formats, r.Format):
		fields["format"] = "Add this format to your profile first"
	case !slices.Contains(recipient.Formats, r.Format):
		fields["format"] = "The student does not take lessons in this format"
	case r.Format == data.FormatOffline && !sameCity(sender.City, recipient.City):
		fields["format"] = "Offline lessons need the same city"
	}
	return fields
}

func hasSkill(skills []data.UserSkill, skillID string) bool {
	return slices.ContainsFunc(skills, func(s data.UserSkill) bool { return s.SkillID == skillID })
}

func sameCity(a, b string) bool {
	a = strings.ToLower(strings.TrimSpace(a))
	return a != "" && a == strings.ToLower(strings.TrimSpace(b))
}
