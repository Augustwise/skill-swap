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

	ExchangeActive = "ACTIVE"
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

// Exchange terms come from the snapshots taken when the request was accepted, so later
// profile changes do not alter them.
type Exchange struct {
	ID               string
	RequestID        string
	Status           string
	Format           string
	TotalSessions    int
	Requester        RequestParty
	Recipient        RequestParty
	RequesterTeaches RequestTerms
	RecipientTeaches RequestTerms
	StartedAt        *time.Time
	CreatedAt        time.Time
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

// LockRequest holds the request row until the surrounding transaction ends.
func (p *Postgres) LockRequest(ctx context.Context, requestID string) (RequestState, error) {
	var r RequestState
	err := p.conn(ctx).QueryRow(ctx, `SELECT id, requester_id, recipient_id, status::text
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

// CreateExchange starts the exchange agreed in the request and copies the skill names and
// levels as they are now.
func (p *Postgres) CreateExchange(ctx context.Context, requestID, acceptedByID string) (string, error) {
	var id string
	err := p.conn(ctx).QueryRow(ctx, `WITH r AS (
			SELECT * FROM exchange_requests WHERE id = $1
		), created AS (
			INSERT INTO exchanges (source_request_id, user_a_id, user_b_id, status, total_sessions, format, started_at)
			SELECT id, requester_id, recipient_id, 'ACTIVE', total_sessions, format, now() FROM r
			RETURNING id, created_at
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

func (p *Postgres) ExchangeByID(ctx context.Context, exchangeID string) (Exchange, error) {
	var e Exchange
	err := p.conn(ctx).QueryRow(ctx, `SELECT e.id, COALESCE(e.source_request_id::text, ''), e.status::text,
			e.format::text, e.total_sessions, `+requestParty("ua")+`, `+requestParty("ub")+`,
			`+commitmentTerms("e.user_a_id")+`, `+commitmentTerms("e.user_b_id")+`, e.started_at, e.created_at
		FROM exchanges e
		JOIN users ua ON ua.id = e.user_a_id
		JOIN users ub ON ub.id = e.user_b_id
		WHERE e.id = $1`, exchangeID).Scan(&e.ID, &e.RequestID, &e.Status, &e.Format, &e.TotalSessions,
		&e.Requester, &e.Recipient, &e.RequesterTeaches, &e.RecipientTeaches, &e.StartedAt, &e.CreatedAt)
	if err != nil {
		return Exchange{}, notFound(err)
	}
	return e, nil
}
