package core

import (
	"context"

	"skillswap/backend/internal/communication"
	"skillswap/backend/internal/data"
	"skillswap/backend/internal/discovery"
	"skillswap/backend/internal/exchange"
	"skillswap/backend/internal/profile"
	"skillswap/backend/internal/session"
	"skillswap/backend/internal/trust"
)

type IApplication interface {
	Universities(ctx context.Context) ([]data.University, error)
	Faculties(ctx context.Context, universityID string) ([]data.Faculty, error)
	SkillCategories(ctx context.Context) ([]data.SkillCategory, error)
	Skills(ctx context.Context, query string, limit int) ([]data.Skill, error)

	Profile(ctx context.Context, userID string) (data.Profile, error)
	UpdateProfile(ctx context.Context, userID string, patch profile.ProfilePatch) (data.Profile, error)
	AddSkill(ctx context.Context, userID string, list data.SkillList, skillID, level string) (data.Profile, error)
	UpdateSkillLevel(ctx context.Context, userID string, list data.SkillList, skillID, level string) (data.Profile, error)
	RemoveSkill(ctx context.Context, userID string, list data.SkillList, skillID string) (data.Profile, error)

	MutualMatches(ctx context.Context, userID string, page int) (discovery.MatchPage, error)
	SearchStudents(ctx context.Context, userID string, filter data.StudentFilter, page int) (discovery.StudentPage, error)
	StudentProfile(ctx context.Context, userID, studentID string) (data.StudentProfile, error)

	CreateRequest(ctx context.Context, userID string, request exchange.NewRequest) (data.ExchangeRequest, error)
	Requests(ctx context.Context, userID string, filter data.RequestFilter, page int) (exchange.RequestPage, error)
	RequestDetails(ctx context.Context, userID, requestID string) (exchange.RequestDetails, error)
	DeclineRequest(ctx context.Context, userID, requestID string) (data.ExchangeRequest, error)
	WithdrawRequest(ctx context.Context, userID, requestID string) (data.ExchangeRequest, error)
	AcceptRequest(ctx context.Context, userID, requestID string) (exchange.Acceptance, error)
	ExchangeDetails(ctx context.Context, userID, exchangeID string) (exchange.ExchangeDetails, error)
	Exchanges(ctx context.Context, userID string, filter data.ExchangeFilter, page int) (exchange.ExchangePage, error)
	CompleteExchange(ctx context.Context, userID, exchangeID string) (data.Exchange, error)
	CancelExchange(ctx context.Context, userID, exchangeID, reason string) (data.Exchange, error)

	ExchangeSessions(ctx context.Context, userID, exchangeID string) (session.ExchangeSessions, error)
	ProposeExchangeSession(ctx context.Context, userID, exchangeID string, in session.ProposalInput) (session.ProposalResult, error)
	SessionDetails(ctx context.Context, userID, sessionID string) (session.SessionDetails, error)
	ProposeSession(ctx context.Context, userID, sessionID string, in session.ProposalInput) (session.ProposalResult, error)
	AcceptSessionProposal(ctx context.Context, userID, sessionID string) (session.ProposalResult, error)
	DeclineSessionProposal(ctx context.Context, userID, sessionID string) (session.ProposalResult, error)
	CancelSession(ctx context.Context, userID, sessionID, reason string) (data.Session, error)
	ConfirmSessionCompletion(ctx context.Context, userID, sessionID string) (session.CompletionResult, error)
	Schedule(ctx context.Context, userID string, filter data.ScheduleFilter, page int) (session.SchedulePage, error)

	Conversations(ctx context.Context, userID string, page int) (communication.ConversationPage, error)
	Conversation(ctx context.Context, userID, conversationID string) (data.Conversation, error)
	RequestConversation(ctx context.Context, userID, requestID string) (data.Conversation, error)
	ExchangeConversation(ctx context.Context, userID, exchangeID string) (data.Conversation, error)
	Messages(ctx context.Context, userID, conversationID string, query communication.MessageQuery) (communication.MessagePage, error)
	SendMessage(ctx context.Context, userID, conversationID, body string) (data.Message, error)
	MarkConversationRead(ctx context.Context, userID, conversationID string) (communication.ConversationReadResult, error)
	Notifications(ctx context.Context, userID string, unreadOnly bool, page int) (communication.NotificationPage, error)
	MarkNotificationRead(ctx context.Context, userID, notificationID string) (communication.NotificationReadResult, error)
	MarkAllNotificationsRead(ctx context.Context, userID string) (communication.NotificationsReadAllResult, error)

	CreateReview(ctx context.Context, userID, exchangeID string, in trust.ReviewInput) (trust.ReviewResult, error)
	ExchangeReviews(ctx context.Context, userID, exchangeID string) (trust.ExchangeReviews, error)
	UserReviews(ctx context.Context, viewerID, studentID string, page int) (trust.ReviewPage, error)
}

type Application struct {
	profiles      *profile.Service
	discovery     *discovery.Service
	exchanges     *exchange.Service
	sessions      *session.Service
	communication *communication.Service
	trust         *trust.Service
}

var _ IApplication = (*Application)(nil)

func NewApplication(profiles *profile.Service, discovery *discovery.Service, exchanges *exchange.Service,
	comm *communication.Service, sessions *session.Service, trust *trust.Service) *Application {
	return &Application{
		profiles:      profiles,
		discovery:     discovery,
		exchanges:     exchanges,
		sessions:      sessions,
		communication: comm,
		trust:         trust,
	}
}

func (a *Application) Universities(ctx context.Context) ([]data.University, error) {
	return a.profiles.Universities(ctx)
}

func (a *Application) Faculties(ctx context.Context, universityID string) ([]data.Faculty, error) {
	return a.profiles.Faculties(ctx, universityID)
}

func (a *Application) SkillCategories(ctx context.Context) ([]data.SkillCategory, error) {
	return a.profiles.SkillCategories(ctx)
}

func (a *Application) Skills(ctx context.Context, query string, limit int) ([]data.Skill, error) {
	return a.profiles.Skills(ctx, query, limit)
}

func (a *Application) Profile(ctx context.Context, userID string) (data.Profile, error) {
	return a.profiles.Profile(ctx, userID)
}

func (a *Application) UpdateProfile(ctx context.Context, userID string, patch profile.ProfilePatch) (data.Profile, error) {
	return a.profiles.UpdateProfile(ctx, userID, patch)
}

func (a *Application) AddSkill(ctx context.Context, userID string, list data.SkillList, skillID, level string) (data.Profile, error) {
	return a.profiles.AddSkill(ctx, userID, list, skillID, level)
}

func (a *Application) UpdateSkillLevel(ctx context.Context, userID string, list data.SkillList, skillID, level string) (data.Profile, error) {
	return a.profiles.UpdateSkillLevel(ctx, userID, list, skillID, level)
}

func (a *Application) RemoveSkill(ctx context.Context, userID string, list data.SkillList, skillID string) (data.Profile, error) {
	return a.profiles.RemoveSkill(ctx, userID, list, skillID)
}

func (a *Application) MutualMatches(ctx context.Context, userID string, page int) (discovery.MatchPage, error) {
	return a.discovery.MutualMatches(ctx, userID, page)
}

func (a *Application) SearchStudents(ctx context.Context, userID string, filter data.StudentFilter, page int) (discovery.StudentPage, error) {
	return a.discovery.SearchStudents(ctx, userID, filter, page)
}

func (a *Application) StudentProfile(ctx context.Context, userID, studentID string) (data.StudentProfile, error) {
	return a.discovery.StudentProfile(ctx, userID, studentID)
}

func (a *Application) CreateRequest(ctx context.Context, userID string, request exchange.NewRequest) (data.ExchangeRequest, error) {
	return a.exchanges.CreateRequest(ctx, userID, request)
}

func (a *Application) Requests(ctx context.Context, userID string, filter data.RequestFilter, page int) (exchange.RequestPage, error) {
	return a.exchanges.Requests(ctx, userID, filter, page)
}

func (a *Application) RequestDetails(ctx context.Context, userID, requestID string) (exchange.RequestDetails, error) {
	return a.exchanges.RequestDetails(ctx, userID, requestID)
}

func (a *Application) DeclineRequest(ctx context.Context, userID, requestID string) (data.ExchangeRequest, error) {
	return a.exchanges.DeclineRequest(ctx, userID, requestID)
}

func (a *Application) WithdrawRequest(ctx context.Context, userID, requestID string) (data.ExchangeRequest, error) {
	return a.exchanges.WithdrawRequest(ctx, userID, requestID)
}

func (a *Application) AcceptRequest(ctx context.Context, userID, requestID string) (exchange.Acceptance, error) {
	return a.exchanges.AcceptRequest(ctx, userID, requestID)
}

func (a *Application) ExchangeDetails(ctx context.Context, userID, exchangeID string) (exchange.ExchangeDetails, error) {
	return a.exchanges.ExchangeDetails(ctx, userID, exchangeID)
}

func (a *Application) Exchanges(ctx context.Context, userID string, filter data.ExchangeFilter, page int) (exchange.ExchangePage, error) {
	return a.exchanges.Exchanges(ctx, userID, filter, page)
}

func (a *Application) CompleteExchange(ctx context.Context, userID, exchangeID string) (data.Exchange, error) {
	return a.exchanges.CompleteExchange(ctx, userID, exchangeID)
}

func (a *Application) CancelExchange(ctx context.Context, userID, exchangeID, reason string) (data.Exchange, error) {
	return a.exchanges.CancelExchange(ctx, userID, exchangeID, reason)
}

func (a *Application) ExchangeSessions(ctx context.Context, userID, exchangeID string) (session.ExchangeSessions, error) {
	return a.sessions.ExchangeSessions(ctx, userID, exchangeID)
}

func (a *Application) ProposeExchangeSession(ctx context.Context, userID, exchangeID string, in session.ProposalInput) (session.ProposalResult, error) {
	return a.sessions.ProposeExchangeSession(ctx, userID, exchangeID, in)
}

func (a *Application) SessionDetails(ctx context.Context, userID, sessionID string) (session.SessionDetails, error) {
	return a.sessions.SessionDetails(ctx, userID, sessionID)
}

func (a *Application) ProposeSession(ctx context.Context, userID, sessionID string, in session.ProposalInput) (session.ProposalResult, error) {
	return a.sessions.ProposeSession(ctx, userID, sessionID, in)
}

func (a *Application) AcceptSessionProposal(ctx context.Context, userID, sessionID string) (session.ProposalResult, error) {
	return a.sessions.AcceptSessionProposal(ctx, userID, sessionID)
}

func (a *Application) DeclineSessionProposal(ctx context.Context, userID, sessionID string) (session.ProposalResult, error) {
	return a.sessions.DeclineSessionProposal(ctx, userID, sessionID)
}

func (a *Application) CancelSession(ctx context.Context, userID, sessionID, reason string) (data.Session, error) {
	return a.sessions.CancelSession(ctx, userID, sessionID, reason)
}

func (a *Application) ConfirmSessionCompletion(ctx context.Context, userID, sessionID string) (session.CompletionResult, error) {
	return a.sessions.ConfirmSessionCompletion(ctx, userID, sessionID)
}

func (a *Application) Schedule(ctx context.Context, userID string, filter data.ScheduleFilter, page int) (session.SchedulePage, error) {
	return a.sessions.Schedule(ctx, userID, filter, page)
}

func (a *Application) Conversations(ctx context.Context, userID string, page int) (communication.ConversationPage, error) {
	return a.communication.Conversations(ctx, userID, page)
}

func (a *Application) Conversation(ctx context.Context, userID, conversationID string) (data.Conversation, error) {
	return a.communication.Conversation(ctx, userID, conversationID)
}

func (a *Application) RequestConversation(ctx context.Context, userID, requestID string) (data.Conversation, error) {
	return a.communication.RequestConversation(ctx, userID, requestID)
}

func (a *Application) ExchangeConversation(ctx context.Context, userID, exchangeID string) (data.Conversation, error) {
	return a.communication.ExchangeConversation(ctx, userID, exchangeID)
}

func (a *Application) Messages(ctx context.Context, userID, conversationID string, query communication.MessageQuery) (communication.MessagePage, error) {
	return a.communication.Messages(ctx, userID, conversationID, query)
}

func (a *Application) SendMessage(ctx context.Context, userID, conversationID, body string) (data.Message, error) {
	return a.communication.SendMessage(ctx, userID, conversationID, body)
}

func (a *Application) MarkConversationRead(ctx context.Context, userID, conversationID string) (communication.ConversationReadResult, error) {
	return a.communication.MarkConversationRead(ctx, userID, conversationID)
}

func (a *Application) Notifications(ctx context.Context, userID string, unreadOnly bool, page int) (communication.NotificationPage, error) {
	return a.communication.Notifications(ctx, userID, unreadOnly, page)
}

func (a *Application) MarkNotificationRead(ctx context.Context, userID, notificationID string) (communication.NotificationReadResult, error) {
	return a.communication.MarkNotificationRead(ctx, userID, notificationID)
}

func (a *Application) MarkAllNotificationsRead(ctx context.Context, userID string) (communication.NotificationsReadAllResult, error) {
	return a.communication.MarkAllNotificationsRead(ctx, userID)
}

func (a *Application) CreateReview(ctx context.Context, userID, exchangeID string, in trust.ReviewInput) (trust.ReviewResult, error) {
	return a.trust.CreateReview(ctx, userID, exchangeID, in)
}

func (a *Application) ExchangeReviews(ctx context.Context, userID, exchangeID string) (trust.ExchangeReviews, error) {
	return a.trust.ExchangeReviews(ctx, userID, exchangeID)
}

func (a *Application) UserReviews(ctx context.Context, viewerID, studentID string, page int) (trust.ReviewPage, error) {
	return a.trust.UserReviews(ctx, viewerID, studentID, page)
}
