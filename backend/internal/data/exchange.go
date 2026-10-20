package data

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	RequestPending   = "PENDING"
	RequestAccepted  = "ACCEPTED"
	RequestDeclined  = "DECLINED"
	RequestWithdrawn = "WITHDRAWN"

	ExchangePlanned   = "PLANNED"
	ExchangeActive    = "ACTIVE"
	ExchangePaused    = "PAUSED"
	ExchangeCompleted = "COMPLETED"
	ExchangeCancelled = "CANCELLED"
)

var ErrDuplicateRequest = errors.New("a pending request for this pair of skills already exists")

// RequestRecipient is a visible student with the settings that decide who may send them requests.
type RequestRecipient struct {
	Profile
	AcceptsRequests    bool
	SameUniversityOnly bool
}

type NewRequest struct {
	RequesterID string
	RecipientID string
	// TeachSkillID is the catalog skill the requester teaches; LearnSkillID the one the recipient teaches.
	TeachSkillID         string
	LearnSkillID         string
	Format               string
	TeachSessions        int
	TeachDurationMinutes int
	LearnSessions        int
	LearnDurationMinutes int
	Message              string
}

type RequestParty struct {
	UserID         string
	FirstName      string
	LastName       string
	UniversityID   string
	UniversityName string
	City           string
}

// RequestTerms describe one direction of the exchange: who teaches which skill and how much.
type RequestTerms struct {
	SkillID         string
	CategoryID      string
	Name            string
	TeacherLevel    string
	LearnerLevel    string
	Sessions        int
	DurationMinutes int
}

type ExchangeRequest struct {
	ID               string
	Status           string
	Format           string
	TotalSessions    int
	Message          string // empty when there is no message
	Requester        RequestParty
	Recipient        RequestParty
	RequesterTeaches RequestTerms
	RecipientTeaches RequestTerms
	ExchangeID       string // empty until the request is accepted
	CreatedAt        time.Time
	RespondedAt      *time.Time
}

type RequestDirection string

const (
	Incoming RequestDirection = "incoming"
	Outgoing RequestDirection = "outgoing"
)

type RequestFilter struct {
	Direction RequestDirection
	Status    string // empty for any status
}

type RequestStatusChange struct {
	Status string
	// ChangedBy fields are empty when the change was not made by a user.
	ChangedByID        string
	ChangedByFirstName string
	ChangedByLastName  string
	CreatedAt          time.Time
}

// RequestState is the part of a request that decides who may change its status.
type RequestState struct {
	ID          string
	RequesterID string
	RecipientID string
	Status      string
}

type ExchangeFilter struct {
	Status string // empty for any status
}

type ExchangeProgress struct {
	CompletedSessions          int
	TotalSessions              int
	RequesterCompletedSessions int
	RequesterPlannedSessions   int
	RecipientCompletedSessions int
	RecipientPlannedSessions   int
	ScheduledSessions          int
	ProposedSessions           int
	DraftSessions              int
	CancelledSessions          int
	// AwaitingConfirmationSessions are scheduled sessions that have already ended.
	AwaitingConfirmationSessions int
	Percent                      int
}

// PlanCompleted reports whether all planned sessions in both directions have been confirmed.
func (p ExchangeProgress) PlanCompleted() bool {
	requesterDone := p.RequesterPlannedSessions > 0 && p.RequesterCompletedSessions >= p.RequesterPlannedSessions
	recipientDone := p.RecipientPlannedSessions > 0 && p.RecipientCompletedSessions >= p.RecipientPlannedSessions
	return requesterDone && recipientDone
}

type NextSessionSummary struct {
	ID           string
	PlanOrder    int
	Title        string
	Status       string
	Format       string
	StartsAt     time.Time
	EndsAt       time.Time
	Teacher      RequestParty
	Learner      RequestParty
	MeetingPlace string
	MeetingURL   string
}

// Exchange terms come from the snapshots taken when the request was accepted, so later
// profile changes do not alter them.
type Exchange struct {
	ID               string
	RequestID        string
	Status           string
	Format           string
	TotalSessions    int
	CanInteract      bool
	Requester        RequestParty
	Recipient        RequestParty
	RequesterTeaches RequestTerms
	RecipientTeaches RequestTerms
	Progress         ExchangeProgress
	NextSession      *NextSessionSummary
	// ConfirmedUserIDs are the participants who confirmed that the exchange is finished.
	ConfirmedUserIDs []string
	// ReviewAuthorIDs are the participants who have reviewed their partner.
	ReviewAuthorIDs    []string
	StartedAt          *time.Time
	CompletedAt        *time.Time
	CancelledAt        *time.Time
	CancellationReason string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type ExchangeStatusChange struct {
	Status string
	// ChangedBy fields are empty when the change was not made by a user.
	ChangedByID        string
	ChangedByFirstName string
	ChangedByLastName  string
	Reason             string
	CreatedAt          time.Time
}

type IExchangeData interface {
	RequestRecipient(ctx context.Context, senderID, recipientID string) (RequestRecipient, error)
	CreateRequest(ctx context.Context, request NewRequest) (string, error)
	RequestByID(ctx context.Context, requestID string) (ExchangeRequest, error)
	UserRequests(ctx context.Context, userID string, filter RequestFilter, limit, offset int) ([]ExchangeRequest, int, error)
	RequestHistory(ctx context.Context, requestID string) ([]RequestStatusChange, error)
	LockRequest(ctx context.Context, requestID string) (RequestState, error)
	UpdateRequestStatus(ctx context.Context, requestID, status, changedByID string) error
	RequestIsCurrent(ctx context.Context, requestID string) (bool, error)
	CreateExchange(ctx context.Context, requestID, acceptedByID string) (string, error)
	ExchangeByID(ctx context.Context, exchangeID string) (Exchange, error)
	UserExchanges(ctx context.Context, userID string, filter ExchangeFilter, limit, offset int) ([]Exchange, int, error)
	ExchangeHistory(ctx context.Context, exchangeID string) ([]ExchangeStatusChange, error)
	LockExchange(ctx context.Context, exchangeID string) (Exchange, error)
	ConfirmExchangeCompletion(ctx context.Context, exchangeID, userID string) error
	CompleteExchange(ctx context.Context, exchangeID, completedByID string) error
	CancelExchange(ctx context.Context, exchangeID, cancelledByID, reason string) error
}

var _ IExchangeData = (*Postgres)(nil)

func (p *Postgres) RequestRecipient(ctx context.Context, senderID, recipientID string) (RequestRecipient, error) {
	var recipient RequestRecipient
	err := p.conn(ctx).QueryRow(ctx, `SELECT COALESCE(ps.accepts_requests, true),
			COALESCE(ps.request_scope = 'SAME_UNIVERSITY', false)
		FROM users c
		LEFT JOIN user_profile_settings ps ON ps.user_id = c.id
		WHERE c.id = $2 AND `+visibleStudent, senderID, recipientID).Scan(&recipient.AcceptsRequests, &recipient.SameUniversityOnly)
	if err != nil {
		return RequestRecipient{}, notFound(err)
	}
	if recipient.Profile, err = p.ProfileByUserID(ctx, recipientID); err != nil {
		return RequestRecipient{}, err
	}
	return recipient, nil
}

// CreateRequest finds the four active skill rows itself and returns ErrNotFound when one of them is gone.
func (p *Postgres) CreateRequest(ctx context.Context, r NewRequest) (string, error) {
	var id string
	err := p.conn(ctx).QueryRow(ctx, `WITH created AS (
			INSERT INTO exchange_requests (requester_id, recipient_id,
				requester_teaching_skill_id, recipient_learning_skill_id, recipient_teaching_skill_id, requester_learning_skill_id,
				format, total_sessions, requester_sessions, recipient_sessions,
				requester_duration_minutes, recipient_duration_minutes, message)
			SELECT rt.user_id, ct.user_id, rt.id, cl.id, ct.id, rl.id,
				$5::lesson_format, $6::smallint + $8::smallint, $6, $8, $7, $9, NULLIF($10, '')
			FROM user_teaching_skills rt
			JOIN user_learning_skills cl ON cl.user_id = $2 AND cl.skill_id = $3 AND cl.is_active
			JOIN user_teaching_skills ct ON ct.user_id = $2 AND ct.skill_id = $4 AND ct.is_active
			JOIN user_learning_skills rl ON rl.user_id = $1 AND rl.skill_id = $4 AND rl.is_active
			WHERE rt.user_id = $1 AND rt.skill_id = $3 AND rt.is_active
			RETURNING id, requester_id, created_at
		)
		INSERT INTO exchange_request_status_history (request_id, status, changed_by_user_id, created_at)
		SELECT id, 'PENDING', requester_id, created_at FROM created
		RETURNING request_id`,
		r.RequesterID, r.RecipientID, r.TeachSkillID, r.LearnSkillID, r.Format,
		r.TeachSessions, r.TeachDurationMinutes, r.LearnSessions, r.LearnDurationMinutes, r.Message).Scan(&id)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "idx_exchange_requests_pending_skill_pair" {
		return "", ErrDuplicateRequest
	}
	if err != nil {
		return "", notFound(err)
	}
	return id, nil
}

func requestParty(user string) string {
	return `json_build_object('UserID', ` + user + `.id, 'FirstName', ` + user + `.first_name,
		'LastName', ` + user + `.last_name, 'UniversityID', ` + user + `.university_id,
		'UniversityName', (SELECT un.name FROM universities un WHERE un.id = ` + user + `.university_id),
		'City', COALESCE(` + user + `.city, ''))`
}

func requestTerms(teaching, learning, sessions, duration string) string {
	return `(SELECT json_build_object('SkillID', s.id, 'CategoryID', s.category_id, 'Name', s.name,
		'TeacherLevel', t.level, 'LearnerLevel', l.current_level, 'Sessions', r.` + sessions + `,
		'DurationMinutes', r.` + duration + `)
		FROM user_teaching_skills t
		JOIN user_learning_skills l ON l.id = r.` + learning + `
		JOIN skills s ON s.id = t.skill_id
		WHERE t.id = r.` + teaching + `)`
}

var requestColumns = `r.id, r.status::text, r.format::text, r.total_sessions, COALESCE(r.message, ''),
		` + requestParty("rq") + `, ` + requestParty("rc") + `,
		` + requestTerms("requester_teaching_skill_id", "recipient_learning_skill_id", "requester_sessions", "requester_duration_minutes") + `,
		` + requestTerms("recipient_teaching_skill_id", "requester_learning_skill_id", "recipient_sessions", "recipient_duration_minutes") + `,
		COALESCE((SELECT e.id::text FROM exchanges e WHERE e.source_request_id = r.id), ''),
		r.created_at, r.responded_at`

const requestFrom = `
	FROM exchange_requests r
	JOIN users rq ON rq.id = r.requester_id
	JOIN users rc ON rc.id = r.recipient_id`

var requestSelect = `SELECT ` + requestColumns + requestFrom

func (r *ExchangeRequest) scanTargets(extra ...any) []any {
	return append([]any{&r.ID, &r.Status, &r.Format, &r.TotalSessions, &r.Message, &r.Requester, &r.Recipient,
		&r.RequesterTeaches, &r.RecipientTeaches, &r.ExchangeID, &r.CreatedAt, &r.RespondedAt}, extra...)
}

func (p *Postgres) RequestByID(ctx context.Context, requestID string) (ExchangeRequest, error) {
	var r ExchangeRequest
	if err := p.conn(ctx).QueryRow(ctx, requestSelect+` WHERE r.id = $1`, requestID).Scan(r.scanTargets()...); err != nil {
		return ExchangeRequest{}, notFound(err)
	}
	return r, nil
}

var requestOwner = map[RequestDirection]string{Incoming: "r.recipient_id", Outgoing: "r.requester_id"}

func (p *Postgres) UserRequests(ctx context.Context, userID string, filter RequestFilter, limit, offset int) ([]ExchangeRequest, int, error) {
	owner, ok := requestOwner[filter.Direction]
	if !ok {
		return nil, 0, errors.New("unknown request direction " + string(filter.Direction))
	}
	where := ` WHERE ` + owner + ` = $1 AND ($2 = '' OR r.status::text = $2)`
	rows, err := p.conn(ctx).Query(ctx, `SELECT `+requestColumns+`, count(*) OVER ()`+requestFrom+where+`
		ORDER BY r.created_at DESC, r.id DESC
		LIMIT $3 OFFSET $4`, userID, filter.Status, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	total := 0
	requests, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (ExchangeRequest, error) {
		var r ExchangeRequest
		err := row.Scan(r.scanTargets(&total)...)
		return r, err
	})
	if err != nil {
		return nil, 0, err
	}
	// A page past the end has no rows to carry the total, so count separately.
	if len(requests) == 0 && offset > 0 {
		err = p.conn(ctx).QueryRow(ctx, `SELECT count(*) FROM exchange_requests r`+where,
			userID, filter.Status).Scan(&total)
	}
	return requests, total, err
}

func (p *Postgres) RequestHistory(ctx context.Context, requestID string) ([]RequestStatusChange, error) {
	rows, err := p.conn(ctx).Query(ctx, `SELECT h.status::text, COALESCE(u.id::text, ''),
			COALESCE(u.first_name, ''), COALESCE(u.last_name, ''), h.created_at
		FROM exchange_request_status_history h
		LEFT JOIN users u ON u.id = h.changed_by_user_id
		WHERE h.request_id = $1
		ORDER BY h.created_at, h.id`, requestID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (RequestStatusChange, error) {
		var c RequestStatusChange
		err := row.Scan(&c.Status, &c.ChangedByID, &c.ChangedByFirstName, &c.ChangedByLastName, &c.CreatedAt)
		return c, err
	})
}

// LockRequest holds the participant and request rows until the surrounding transaction ends.
func (p *Postgres) LockRequest(ctx context.Context, requestID string) (RequestState, error) {
	var r RequestState
	err := p.conn(ctx).QueryRow(ctx, `SELECT id, requester_id, recipient_id, status::text
		FROM exchange_requests WHERE id = $1`, requestID).Scan(&r.ID, &r.RequesterID, &r.RecipientID, &r.Status)
	if err != nil {
		return RequestState{}, notFound(err)
	}
	if err := p.LockUsers(ctx, r.RequesterID, r.RecipientID); err != nil {
		return RequestState{}, err
	}
	err = p.conn(ctx).QueryRow(ctx, `SELECT id, requester_id, recipient_id, status::text
		FROM exchange_requests WHERE id = $1 FOR UPDATE`, requestID).Scan(&r.ID, &r.RequesterID, &r.RecipientID, &r.Status)
	if err != nil {
		return RequestState{}, notFound(err)
	}
	return r, nil
}

func (p *Postgres) UpdateRequestStatus(ctx context.Context, requestID, status, changedByID string) error {
	tag, err := p.conn(ctx).Exec(ctx, `WITH updated AS (
			UPDATE exchange_requests SET status = $2::exchange_request_status, responded_at = now()
			WHERE id = $1
			RETURNING id, status, responded_at
		)
		INSERT INTO exchange_request_status_history (request_id, status, changed_by_user_id, created_at)
		SELECT id, status, $3, responded_at FROM updated`, requestID, status, changedByID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RequestIsCurrent reports whether both students are still active, neither has blocked the
// other and all four skill rows are still in their lists.
func (p *Postgres) RequestIsCurrent(ctx context.Context, requestID string) (bool, error) {
	var current bool
	err := p.conn(ctx).QueryRow(ctx, `SELECT EXISTS (SELECT 1
		FROM exchange_requests r
		JOIN users rq ON rq.id = r.requester_id AND rq.deleted_at IS NULL AND rq.account_status = 'ACTIVE'
		JOIN users rc ON rc.id = r.recipient_id AND rc.deleted_at IS NULL AND rc.account_status = 'ACTIVE'
		JOIN user_teaching_skills rt ON rt.id = r.requester_teaching_skill_id AND rt.is_active
		JOIN user_learning_skills cl ON cl.id = r.recipient_learning_skill_id AND cl.is_active
		JOIN user_teaching_skills ct ON ct.id = r.recipient_teaching_skill_id AND ct.is_active
		JOIN user_learning_skills rl ON rl.id = r.requester_learning_skill_id AND rl.is_active
		WHERE r.id = $1 AND NOT EXISTS (SELECT 1 FROM user_blocks b
			WHERE (b.blocker_id = rq.id AND b.blocked_id = rc.id) OR (b.blocker_id = rc.id AND b.blocked_id = rq.id)))`,
		requestID).Scan(&current)
	return current, err
}

// CreateExchange starts the exchange agreed in the request, copies the skill names and
// levels as they are now, and creates the initial DRAFT lesson slots (LR2 5.1, Figure 3).
func (p *Postgres) CreateExchange(ctx context.Context, requestID, acceptedByID string) (string, error) {
	var id string
	err := p.conn(ctx).QueryRow(ctx, `WITH r AS (
			SELECT * FROM exchange_requests WHERE id = $1
		), created AS (
			INSERT INTO exchanges (source_request_id, user_a_id, user_b_id, status, total_sessions, format, started_at)
			SELECT id, requester_id, recipient_id, 'ACTIVE', total_sessions, format, now() FROM r
			RETURNING id, format, created_at
		), commitments AS (
			INSERT INTO exchange_skill_commitments (exchange_id, teacher_id, learner_id, teaching_skill_id,
				learning_skill_id, planned_sessions, duration_minutes, skill_name_snapshot,
				teaching_level_snapshot, learning_level_snapshot)
			SELECT created.id, t.user_id, l.user_id, t.id, l.id, x.sessions, x.duration, s.name, t.level, l.current_level
			FROM created CROSS JOIN r
			CROSS JOIN LATERAL (VALUES
				(r.requester_teaching_skill_id, r.recipient_learning_skill_id, r.requester_sessions, r.requester_duration_minutes),
				(r.recipient_teaching_skill_id, r.requester_learning_skill_id, r.recipient_sessions, r.recipient_duration_minutes)
			) AS x (teaching_id, learning_id, sessions, duration)
			JOIN user_teaching_skills t ON t.id = x.teaching_id
			JOIN user_learning_skills l ON l.id = x.learning_id
			JOIN skills s ON s.id = t.skill_id
		), draft_sessions AS (
			INSERT INTO sessions (exchange_id, plan_order, title, teacher_id, learner_id, status, format, created_at, updated_at)
			SELECT created.id,
				x.offset_order + gs.n,
				s.name || ' — заняття ' || gs.n,
				t.user_id,
				l.user_id,
				'DRAFT',
				created.format,
				created.created_at,
				created.created_at
			FROM created CROSS JOIN r
			CROSS JOIN LATERAL (VALUES
				(r.requester_teaching_skill_id, r.recipient_learning_skill_id, r.requester_sessions::int, 0),
				(r.recipient_teaching_skill_id, r.requester_learning_skill_id, r.recipient_sessions::int, r.requester_sessions::int)
			) AS x (teaching_id, learning_id, sessions, offset_order)
			JOIN user_teaching_skills t ON t.id = x.teaching_id
			JOIN user_learning_skills l ON l.id = x.learning_id
			JOIN skills s ON s.id = t.skill_id
			CROSS JOIN LATERAL generate_series(1, x.sessions) AS gs(n)
		)
		INSERT INTO exchange_status_history (exchange_id, status, changed_by_user_id, created_at)
		SELECT id, 'ACTIVE', $2, created_at FROM created
		RETURNING exchange_id`, requestID, acceptedByID).Scan(&id)
	if err != nil {
		return "", notFound(err)
	}
	return id, nil
}

func commitmentTerms(teacher string) string {
	return `(SELECT json_build_object('SkillID', s.id, 'CategoryID', s.category_id, 'Name', c.skill_name_snapshot,
		'TeacherLevel', c.teaching_level_snapshot, 'LearnerLevel', c.learning_level_snapshot,
		'Sessions', c.planned_sessions, 'DurationMinutes', c.duration_minutes)
		FROM exchange_skill_commitments c
		JOIN user_teaching_skills t ON t.id = c.teaching_skill_id
		JOIN skills s ON s.id = t.skill_id
		WHERE c.exchange_id = e.id AND c.teacher_id = ` + teacher + `)`
}

const exchangeProgressObject = `json_build_object(
	'CompletedSessions', (SELECT count(*) FROM sessions s WHERE s.exchange_id = e.id AND s.status = 'COMPLETED'),
	'TotalSessions', e.total_sessions,
	'RequesterCompletedSessions', (SELECT count(*) FROM sessions s WHERE s.exchange_id = e.id AND s.status = 'COMPLETED' AND s.teacher_id = e.user_a_id),
	'RequesterPlannedSessions', COALESCE((SELECT c.planned_sessions FROM exchange_skill_commitments c WHERE c.exchange_id = e.id AND c.teacher_id = e.user_a_id), 0),
	'RecipientCompletedSessions', (SELECT count(*) FROM sessions s WHERE s.exchange_id = e.id AND s.status = 'COMPLETED' AND s.teacher_id = e.user_b_id),
	'RecipientPlannedSessions', COALESCE((SELECT c.planned_sessions FROM exchange_skill_commitments c WHERE c.exchange_id = e.id AND c.teacher_id = e.user_b_id), 0),
	'ScheduledSessions', (SELECT count(*) FROM sessions s WHERE s.exchange_id = e.id AND s.status = 'SCHEDULED'),
	'ProposedSessions', (SELECT count(*) FROM sessions s WHERE s.exchange_id = e.id AND s.status = 'PROPOSED'),
	'DraftSessions', (SELECT count(*) FROM sessions s WHERE s.exchange_id = e.id AND s.status = 'DRAFT'),
	'CancelledSessions', (SELECT count(*) FROM sessions s WHERE s.exchange_id = e.id AND s.status = 'CANCELLED'),
	'AwaitingConfirmationSessions', (SELECT count(*) FROM sessions s WHERE s.exchange_id = e.id AND s.status = 'SCHEDULED' AND s.scheduled_end <= now()),
	'Percent', CASE WHEN e.total_sessions > 0
		THEN LEAST(100, ((SELECT count(*) FROM sessions s WHERE s.exchange_id = e.id AND s.status = 'COMPLETED') * 100) / e.total_sessions)
		ELSE 0 END
)`

var exchangeNextSessionObject = `(SELECT json_build_object(
		'ID', s.id,
		'PlanOrder', s.plan_order,
		'Title', s.title,
		'Status', s.status::text,
		'Format', COALESCE(s.format::text, e.format::text),
		'StartsAt', s.scheduled_start,
		'EndsAt', s.scheduled_end,
		'Teacher', ` + requestParty("st") + `,
		'Learner', ` + requestParty("sl") + `,
		'MeetingPlace', COALESCE(mp.name, ''),
		'MeetingURL', COALESCE(s.meeting_url, '')
	)
	FROM sessions s
	JOIN users st ON st.id = s.teacher_id
	JOIN users sl ON sl.id = s.learner_id
	LEFT JOIN meeting_places mp ON mp.id = s.meeting_place_id
	WHERE s.exchange_id = e.id AND s.status = 'SCHEDULED' AND s.scheduled_end >= now()
	ORDER BY s.scheduled_start ASC, s.plan_order ASC, s.id ASC
	LIMIT 1)`

const exchangeConfirmedUsers = `COALESCE((
	SELECT array_agg(c.user_id::text ORDER BY c.confirmed_at ASC, c.user_id ASC)
	FROM exchange_confirmations c
	WHERE c.exchange_id = e.id
), ARRAY[]::text[])`

const exchangeReviewAuthors = `COALESCE((
	SELECT array_agg(r.author_id::text ORDER BY r.created_at ASC, r.author_id ASC)
	FROM exchange_reviews r
	WHERE r.exchange_id = e.id
), ARRAY[]::text[])`

var exchangeColumns = `e.id, COALESCE(e.source_request_id::text, ''), e.status::text,
	e.format::text, e.total_sessions, ` + participantsCanInteract + `,
	` + requestParty("ua") + `, ` + requestParty("ub") + `,
	` + commitmentTerms("e.user_a_id") + `, ` + commitmentTerms("e.user_b_id") + `,
	` + exchangeProgressObject + `, ` + exchangeNextSessionObject + `, ` + exchangeConfirmedUsers + `,
	` + exchangeReviewAuthors + `, e.started_at, e.completed_at, e.cancelled_at, COALESCE(e.cancellation_reason, ''),
	e.created_at, e.updated_at`

const exchangeFrom = `
	FROM exchanges e
	JOIN users ua ON ua.id = e.user_a_id
	JOIN users ub ON ub.id = e.user_b_id`

func (e *Exchange) scanTargets(extra ...any) []any {
	return append([]any{&e.ID, &e.RequestID, &e.Status, &e.Format, &e.TotalSessions, &e.CanInteract,
		&e.Requester, &e.Recipient, &e.RequesterTeaches, &e.RecipientTeaches,
		&e.Progress, &e.NextSession, &e.ConfirmedUserIDs, &e.ReviewAuthorIDs,
		&e.StartedAt, &e.CompletedAt, &e.CancelledAt, &e.CancellationReason,
		&e.CreatedAt, &e.UpdatedAt}, extra...)
}

func (p *Postgres) ExchangeByID(ctx context.Context, exchangeID string) (Exchange, error) {
	var e Exchange
	err := p.conn(ctx).QueryRow(ctx, `SELECT `+exchangeColumns+exchangeFrom+` WHERE e.id = $1`, exchangeID).Scan(e.scanTargets()...)
	if err != nil {
		return Exchange{}, notFound(err)
	}
	return e, nil
}

func (p *Postgres) UserExchanges(ctx context.Context, userID string, filter ExchangeFilter, limit, offset int) ([]Exchange, int, error) {
	where := ` WHERE (e.user_a_id = $1 OR e.user_b_id = $1) AND ($2 = '' OR e.status::text = $2)`
	rows, err := p.conn(ctx).Query(ctx, `SELECT `+exchangeColumns+`, count(*) OVER ()`+exchangeFrom+where+`
		ORDER BY CASE WHEN e.status = 'ACTIVE' THEN 0 ELSE 1 END, e.updated_at DESC, e.created_at DESC, e.id DESC
		LIMIT $3 OFFSET $4`, userID, filter.Status, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	total := 0
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Exchange, error) {
		var e Exchange
		err := row.Scan(e.scanTargets(&total)...)
		return e, err
	})
	if err != nil {
		return nil, 0, err
	}
	if len(items) == 0 && offset > 0 {
		err = p.conn(ctx).QueryRow(ctx, `SELECT count(*) FROM exchanges e`+where, userID, filter.Status).Scan(&total)
	}
	return items, total, err
}

func (p *Postgres) ExchangeHistory(ctx context.Context, exchangeID string) ([]ExchangeStatusChange, error) {
	rows, err := p.conn(ctx).Query(ctx, `SELECT h.status::text, COALESCE(u.id::text, ''),
			COALESCE(u.first_name, ''), COALESCE(u.last_name, ''), COALESCE(h.reason, ''), h.created_at
		FROM exchange_status_history h
		LEFT JOIN users u ON u.id = h.changed_by_user_id
		WHERE h.exchange_id = $1
		ORDER BY h.created_at, h.id`, exchangeID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (ExchangeStatusChange, error) {
		var c ExchangeStatusChange
		err := row.Scan(&c.Status, &c.ChangedByID, &c.ChangedByFirstName, &c.ChangedByLastName, &c.Reason, &c.CreatedAt)
		return c, err
	})
}

// LockExchange holds both participants, in the same order as every other operation on the
// pair, and then the exchange row until the surrounding transaction ends.
func (p *Postgres) LockExchange(ctx context.Context, exchangeID string) (Exchange, error) {
	var userA, userB string
	err := p.conn(ctx).QueryRow(ctx, `SELECT user_a_id, user_b_id FROM exchanges WHERE id = $1`, exchangeID).Scan(&userA, &userB)
	if err != nil {
		return Exchange{}, notFound(err)
	}
	if err := p.LockUsers(ctx, userA, userB); err != nil {
		return Exchange{}, err
	}
	if err := p.conn(ctx).QueryRow(ctx, `SELECT user_a_id, user_b_id FROM exchanges WHERE id = $1 FOR UPDATE`, exchangeID).Scan(&userA, &userB); err != nil {
		return Exchange{}, notFound(err)
	}
	return p.ExchangeByID(ctx, exchangeID)
}

func (p *Postgres) ConfirmExchangeCompletion(ctx context.Context, exchangeID, userID string) error {
	_, err := p.conn(ctx).Exec(ctx, `WITH confirmed AS (
			INSERT INTO exchange_confirmations (exchange_id, user_id)
			VALUES ($1, $2)
			ON CONFLICT (exchange_id, user_id) DO NOTHING
			RETURNING exchange_id
		)
		UPDATE exchanges SET updated_at = now() WHERE id IN (SELECT exchange_id FROM confirmed)`, exchangeID, userID)
	return err
}

func (p *Postgres) CompleteExchange(ctx context.Context, exchangeID, completedByID string) error {
	tag, err := p.conn(ctx).Exec(ctx, `WITH completed AS (
			UPDATE exchanges
			SET status = 'COMPLETED', completed_at = now(), updated_at = now()
			WHERE id = $1 AND status = 'ACTIVE'
			RETURNING id, completed_at
		)
		INSERT INTO exchange_status_history (exchange_id, status, changed_by_user_id, created_at)
		SELECT id, 'COMPLETED', $2, completed_at FROM completed`, exchangeID, completedByID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CancelExchange keeps completed sessions and all history, and cancels every session that has
// not taken place yet.
func (p *Postgres) CancelExchange(ctx context.Context, exchangeID, cancelledByID, reason string) error {
	tag, err := p.conn(ctx).Exec(ctx, `WITH cancelled AS (
			UPDATE exchanges
			SET status = 'CANCELLED', cancelled_at = now(), cancellation_reason = $3, updated_at = now()
			WHERE id = $1 AND status = 'ACTIVE'
			RETURNING id, cancelled_at
		)
		INSERT INTO exchange_status_history (exchange_id, status, changed_by_user_id, reason, created_at)
		SELECT id, 'CANCELLED', $2, $3, cancelled_at FROM cancelled`, exchangeID, cancelledByID, reason)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if _, err := p.conn(ctx).Exec(ctx, `UPDATE session_schedule_proposals sp
		SET status = 'SUPERSEDED', responded_by_user_id = $2, responded_at = now()
		FROM sessions s
		WHERE s.id = sp.session_id AND s.exchange_id = $1 AND sp.status = 'PENDING'`, exchangeID, cancelledByID); err != nil {
		return err
	}
	if _, err := p.conn(ctx).Exec(ctx, `DELETE FROM session_confirmations sc
		USING sessions s
		WHERE s.id = sc.session_id AND s.exchange_id = $1 AND s.status IN ('DRAFT', 'PROPOSED', 'SCHEDULED')`, exchangeID); err != nil {
		return err
	}
	_, err = p.conn(ctx).Exec(ctx, `UPDATE sessions
		SET status = 'CANCELLED', cancelled_at = now(), cancellation_reason = $2, updated_at = now()
		WHERE exchange_id = $1 AND status IN ('DRAFT', 'PROPOSED', 'SCHEDULED')`, exchangeID, reason)
	return err
}
