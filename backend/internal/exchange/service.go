package exchange

import (
	"context"
	"errors"
	"slices"
	"strings"

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

	maxMessageLength = 500
)

var (
	formats  = []string{data.FormatOnline, data.FormatOffline}
	statuses = []string{data.RequestPending, data.RequestAccepted, data.RequestDeclined, data.RequestWithdrawn}
)

var (
	ErrStudentNotFound    = errors.New("student was not found or is not visible")
	ErrRequestsClosed     = errors.New("student does not accept requests")
	ErrSameUniversityOnly = errors.New("student accepts requests only from their university")
	ErrDuplicateRequest   = errors.New("a pending request for this pair of skills already exists")
	ErrInvalidPage        = errors.New("page is out of range")
	ErrRequestNotFound    = errors.New("request was not found or the user does not take part in it")
)

type Service struct {
	profiles  data.IProfileData
	exchanges data.IExchangeData
	tx        data.ITransaction
}

func NewService(profiles data.IProfileData, exchanges data.IExchangeData, tx data.ITransaction) *Service {
	return &Service{profiles: profiles, exchanges: exchanges, tx: tx}
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
		return err
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
