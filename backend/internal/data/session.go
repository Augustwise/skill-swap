package data

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	SessionDraft     = "DRAFT"
	SessionProposed  = "PROPOSED"
	SessionScheduled = "SCHEDULED"
	SessionCompleted = "COMPLETED"
	SessionCancelled = "CANCELLED"

	ProposalPending    = "PENDING"
	ProposalAccepted   = "ACCEPTED"
	ProposalDeclined   = "DECLINED"
	ProposalSuperseded = "SUPERSEDED"

	ConfirmSchedule   = "SCHEDULE"
	ConfirmCompletion = "COMPLETION"

	ScheduleAll = "ALL"
)

type SessionProposal struct {
	ID                   string
	SessionID            string
	Status               string
	ProposedByID         string
	ProposedByFirstName  string
	ProposedByLastName   string
	StartsAt             time.Time
	EndsAt               time.Time
	Format               string
	MeetingPlace         string
	MeetingURL           string
	RespondedByID        string
	RespondedByFirstName string
	RespondedByLastName  string
	RespondedAt          *time.Time
	CreatedAt            time.Time
}

type Session struct {
	ID                     string
	ExchangeID             string
	ExchangeStatus         string
	ExchangeFormat         string
	CanInteract            bool
	PlanOrder              int
	Title                  string
	Description            string
	Status                 string
	Format                 string
	SkillID                string
	SkillCategoryID        string
	SkillName              string
	PlannedDurationMinutes int
	Requester              RequestParty
	Recipient              RequestParty
	Teacher                RequestParty
	Learner                RequestParty
	StartsAt               *time.Time
	EndsAt                 *time.Time
	MeetingPlace           string
	MeetingURL             string
	ActiveProposal         *SessionProposal
	ConfirmedUserIDs       []string
	// CompletionConfirmedUserIDs are the participants who confirmed that the session took place.
	CompletionConfirmedUserIDs []string
	CompletedAt                *time.Time
	CancelledAt                *time.Time
	CancellationReason         string
	CreatedAt                  time.Time
	UpdatedAt                  time.Time
}

type NewSessionProposal struct {
	SessionID    string
	ProposerID   string
	TeacherID    string
	LearnerID    string
	Title        string
	Description  string
	StartsAt     time.Time
	EndsAt       time.Time
	Format       string
	MeetingPlace string
	MeetingURL   string
}

type ScheduleFilter struct {
	Status       string
	UpcomingOnly bool
	Now          time.Time
}

type ISessionData interface {
	ExchangeSessions(ctx context.Context, exchangeID string) ([]Session, error)
	SessionByID(ctx context.Context, sessionID string) (Session, error)
	SessionProposals(ctx context.Context, sessionID string) ([]SessionProposal, error)
	LockSession(ctx context.Context, sessionID string) (Session, error)
	FindDraftSessionForTeacher(ctx context.Context, exchangeID, teacherID string) (Session, error)
	SwapDraftSessionTeacher(ctx context.Context, exchangeID, sessionID, newTeacherID string) error
	HasScheduleConflict(ctx context.Context, excludeSessionID, userA, userB string, startsAt, endsAt time.Time) (bool, error)
	SaveSessionProposal(ctx context.Context, proposal NewSessionProposal) (SessionProposal, error)
	AcceptSessionProposal(ctx context.Context, sessionID, responderID string) (SessionProposal, error)
	DeclineSessionProposal(ctx context.Context, sessionID, responderID string) (SessionProposal, error)
	CancelSession(ctx context.Context, sessionID, cancelledByID, reason string) error
	ConfirmSessionCompletion(ctx context.Context, sessionID, userID string) error
	CompleteSession(ctx context.Context, sessionID string) error
	UserSchedule(ctx context.Context, userID string, filter ScheduleFilter, limit, offset int) ([]Session, int, int, string, error)
}

var _ ISessionData = (*Postgres)(nil)

const participantsCanInteract = `(ua.deleted_at IS NULL AND ua.account_status = 'ACTIVE'
	AND ub.deleted_at IS NULL AND ub.account_status = 'ACTIVE'
	AND NOT EXISTS (SELECT 1 FROM user_blocks b
		WHERE (b.blocker_id = ua.id AND b.blocked_id = ub.id)
			OR (b.blocker_id = ub.id AND b.blocked_id = ua.id)))`

const sessionActiveProposal = `(SELECT json_build_object(
		'ID', sp.id,
		'SessionID', sp.session_id,
		'Status', sp.status::text,
		'ProposedByID', sp.proposed_by_user_id::text,
		'ProposedByFirstName', COALESCE(pb.first_name, ''),
		'ProposedByLastName', COALESCE(pb.last_name, ''),
		'StartsAt', sp.proposed_start,
		'EndsAt', sp.proposed_end,
		'Format', sp.format::text,
		'MeetingPlace', COALESCE(pmp.name, ''),
		'MeetingURL', COALESCE(sp.meeting_url, ''),
		'RespondedByID', COALESCE(sp.responded_by_user_id::text, ''),
		'RespondedByFirstName', COALESCE(rb.first_name, ''),
		'RespondedByLastName', COALESCE(rb.last_name, ''),
		'RespondedAt', sp.responded_at,
		'CreatedAt', sp.created_at
	)
	FROM session_schedule_proposals sp
	LEFT JOIN users pb ON pb.id = sp.proposed_by_user_id
	LEFT JOIN users rb ON rb.id = sp.responded_by_user_id
	LEFT JOIN meeting_places pmp ON pmp.id = sp.meeting_place_id
	WHERE sp.session_id = s.id
	ORDER BY CASE WHEN sp.status = 'PENDING' THEN 0 ELSE 1 END, sp.created_at DESC, sp.id DESC
	LIMIT 1)`

func sessionConfirmedUsers(kind string) string {
	return `COALESCE((
	SELECT array_agg(sc.user_id::text ORDER BY sc.confirmed_at ASC, sc.user_id ASC)
	FROM session_confirmations sc
	WHERE sc.session_id = s.id AND sc.kind = '` + kind + `'
), ARRAY[]::text[])`
}

var sessionColumns = `s.id, s.exchange_id, e.status::text, e.format::text, ` + participantsCanInteract + `,
	s.plan_order, s.title, COALESCE(s.description, ''), s.status::text,
	COALESCE(s.format::text, e.format::text),
	COALESCE(cm.skill_id::text, ''), COALESCE(cm.category_id::text, ''),
	COALESCE(cm.skill_name_snapshot, ''), COALESCE(cm.duration_minutes::int, 60),
	` + requestParty("ua") + `, ` + requestParty("ub") + `,
	` + requestParty("st") + `, ` + requestParty("sl") + `,
	s.scheduled_start, s.scheduled_end,
	COALESCE(mp.name, ''), COALESCE(s.meeting_url, ''),
	` + sessionActiveProposal + `, ` + sessionConfirmedUsers(ConfirmSchedule) + `,
	` + sessionConfirmedUsers(ConfirmCompletion) + `,
	s.completed_at, s.cancelled_at, COALESCE(s.cancellation_reason, ''),
	s.created_at, s.updated_at`

const sessionFrom = `
	FROM sessions s
	JOIN exchanges e ON e.id = s.exchange_id
	JOIN users ua ON ua.id = e.user_a_id
	JOIN users ub ON ub.id = e.user_b_id
	JOIN users st ON st.id = s.teacher_id
	JOIN users sl ON sl.id = s.learner_id
	LEFT JOIN meeting_places mp ON mp.id = s.meeting_place_id
	LEFT JOIN LATERAL (
		SELECT c.skill_name_snapshot, c.duration_minutes, sk.id AS skill_id, sk.category_id
		FROM exchange_skill_commitments c
		JOIN user_teaching_skills ts ON ts.id = c.teaching_skill_id
		JOIN skills sk ON sk.id = ts.skill_id
		WHERE c.exchange_id = s.exchange_id AND c.teacher_id = s.teacher_id
		LIMIT 1
	) cm ON true`

var sessionSelect = `SELECT ` + sessionColumns + sessionFrom

func (s *Session) scanTargets(extra ...any) []any {
	return append([]any{
		&s.ID, &s.ExchangeID, &s.ExchangeStatus, &s.ExchangeFormat, &s.CanInteract,
		&s.PlanOrder, &s.Title, &s.Description, &s.Status, &s.Format,
		&s.SkillID, &s.SkillCategoryID, &s.SkillName, &s.PlannedDurationMinutes,
		&s.Requester, &s.Recipient, &s.Teacher, &s.Learner,
		&s.StartsAt, &s.EndsAt, &s.MeetingPlace, &s.MeetingURL,
		&s.ActiveProposal, &s.ConfirmedUserIDs, &s.CompletionConfirmedUserIDs,
		&s.CompletedAt, &s.CancelledAt, &s.CancellationReason,
		&s.CreatedAt, &s.UpdatedAt,
	}, extra...)
}

func (p *Postgres) ExchangeSessions(ctx context.Context, exchangeID string) ([]Session, error) {
	rows, err := p.conn(ctx).Query(ctx, sessionSelect+`
		WHERE s.exchange_id = $1
		ORDER BY s.plan_order ASC, s.id ASC`, exchangeID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Session, error) {
		var s Session
		err := row.Scan(s.scanTargets()...)
		return s, err
	})
}

func (p *Postgres) SessionByID(ctx context.Context, sessionID string) (Session, error) {
	var s Session
	err := p.conn(ctx).QueryRow(ctx, sessionSelect+` WHERE s.id = $1`, sessionID).Scan(s.scanTargets()...)
	if err != nil {
		return Session{}, notFound(err)
	}
	return s, nil
}

const proposalColumns = `sp.id, sp.session_id, sp.status::text,
	sp.proposed_by_user_id::text, COALESCE(pb.first_name, ''), COALESCE(pb.last_name, ''),
	sp.proposed_start, sp.proposed_end, sp.format::text,
	COALESCE(mp.name, ''), COALESCE(sp.meeting_url, ''),
	COALESCE(sp.responded_by_user_id::text, ''), COALESCE(rb.first_name, ''), COALESCE(rb.last_name, ''),
	sp.responded_at, sp.created_at`

const proposalFrom = `
	FROM session_schedule_proposals sp
	LEFT JOIN users pb ON pb.id = sp.proposed_by_user_id
	LEFT JOIN users rb ON rb.id = sp.responded_by_user_id
	LEFT JOIN meeting_places mp ON mp.id = sp.meeting_place_id`

func (sp *SessionProposal) scanTargets() []any {
	return []any{
		&sp.ID, &sp.SessionID, &sp.Status,
		&sp.ProposedByID, &sp.ProposedByFirstName, &sp.ProposedByLastName,
		&sp.StartsAt, &sp.EndsAt, &sp.Format,
		&sp.MeetingPlace, &sp.MeetingURL,
		&sp.RespondedByID, &sp.RespondedByFirstName, &sp.RespondedByLastName,
		&sp.RespondedAt, &sp.CreatedAt,
	}
}

func (p *Postgres) SessionProposals(ctx context.Context, sessionID string) ([]SessionProposal, error) {
	rows, err := p.conn(ctx).Query(ctx, `SELECT `+proposalColumns+proposalFrom+`
		WHERE sp.session_id = $1
		ORDER BY sp.created_at ASC, sp.id ASC`, sessionID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (SessionProposal, error) {
		var sp SessionProposal
		err := row.Scan(sp.scanTargets()...)
		return sp, err
	})
}

func (p *Postgres) proposalByID(ctx context.Context, proposalID string) (SessionProposal, error) {
	var sp SessionProposal
	err := p.conn(ctx).QueryRow(ctx, `SELECT `+proposalColumns+proposalFrom+` WHERE sp.id = $1`, proposalID).Scan(sp.scanTargets()...)
	if err != nil {
		return SessionProposal{}, notFound(err)
	}
	return sp, nil
}

// LockSession locks both participants of the session's exchange in canonical UUID order and
// locks the session row.
func (p *Postgres) LockSession(ctx context.Context, sessionID string) (Session, error) {
	var userA, userB string
	err := p.conn(ctx).QueryRow(ctx, `SELECT e.user_a_id, e.user_b_id
		FROM sessions s
		JOIN exchanges e ON e.id = s.exchange_id
		WHERE s.id = $1`, sessionID).Scan(&userA, &userB)
	if err != nil {
		return Session{}, notFound(err)
	}
	if err := p.LockUsers(ctx, userA, userB); err != nil {
		return Session{}, err
	}
	var lockedID string
	if err := p.conn(ctx).QueryRow(ctx, `SELECT id FROM sessions WHERE id = $1 FOR UPDATE`, sessionID).Scan(&lockedID); err != nil {
		return Session{}, notFound(err)
	}
	return p.SessionByID(ctx, sessionID)
}

func (p *Postgres) FindDraftSessionForTeacher(ctx context.Context, exchangeID, teacherID string) (Session, error) {
	var sessionID string
	err := p.conn(ctx).QueryRow(ctx, `SELECT id FROM sessions
		WHERE exchange_id = $1 AND teacher_id = $2 AND status = 'DRAFT'
		ORDER BY plan_order ASC, id ASC
		LIMIT 1 FOR UPDATE`, exchangeID, teacherID).Scan(&sessionID)
	if err != nil {
		return Session{}, notFound(err)
	}
	return p.SessionByID(ctx, sessionID)
}

func (p *Postgres) SwapDraftSessionTeacher(ctx context.Context, exchangeID, sessionID, newTeacherID string) error {
	var otherID, oldTeacherID, oldLearnerID, oldTitle, otherTitle string
	err := p.conn(ctx).QueryRow(ctx, `SELECT id, teacher_id, learner_id, title
		FROM sessions WHERE id = $1 AND exchange_id = $2 AND status = 'DRAFT' FOR UPDATE`,
		sessionID, exchangeID).Scan(&sessionID, &oldTeacherID, &oldLearnerID, &oldTitle)
	if err != nil {
		return notFound(err)
	}
	if oldTeacherID == newTeacherID {
		return nil
	}
	err = p.conn(ctx).QueryRow(ctx, `SELECT id, title
		FROM sessions
		WHERE exchange_id = $1 AND teacher_id = $2 AND status = 'DRAFT' AND id <> $3
		ORDER BY plan_order ASC, id ASC
		LIMIT 1 FOR UPDATE`, exchangeID, newTeacherID, sessionID).Scan(&otherID, &otherTitle)
	if err != nil {
		return notFound(err)
	}
	if _, err := p.conn(ctx).Exec(ctx, `UPDATE sessions
		SET teacher_id = CASE WHEN id = $1 THEN $3::uuid ELSE $4::uuid END,
			learner_id = CASE WHEN id = $1 THEN $4::uuid ELSE $3::uuid END,
			title = CASE WHEN id = $1 THEN $6 ELSE $5 END,
			updated_at = now()
		WHERE id IN ($1, $2)`,
		sessionID, otherID, newTeacherID, oldTeacherID, oldTitle, otherTitle); err != nil {
		return err
	}
	return nil
}

// HasScheduleConflict checks half-open intervals [startsAt, endsAt) against all SCHEDULED
// sessions of both participants across all exchanges (LR2 5.3, 6.2).
func (p *Postgres) HasScheduleConflict(ctx context.Context, excludeSessionID, userA, userB string, startsAt, endsAt time.Time) (bool, error) {
	var conflict bool
	err := p.conn(ctx).QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM sessions s
		WHERE ($1 = '' OR s.id <> $1::uuid)
			AND s.status = 'SCHEDULED'
			AND (s.teacher_id IN ($2, $3) OR s.learner_id IN ($2, $3))
			AND s.scheduled_start < $5
			AND s.scheduled_end > $4
	)`, excludeSessionID, userA, userB, startsAt, endsAt).Scan(&conflict)
	return conflict, err
}

func (p *Postgres) SaveSessionProposal(ctx context.Context, in NewSessionProposal) (SessionProposal, error) {
	var meetingPlaceID string
	if in.Format == FormatOffline && in.MeetingPlace != "" {
		if err := p.conn(ctx).QueryRow(ctx, `INSERT INTO meeting_places (user_id, name)
			VALUES ($1, $2) RETURNING id`, in.ProposerID, in.MeetingPlace).Scan(&meetingPlaceID); err != nil {
			return SessionProposal{}, err
		}
	}
	if _, err := p.conn(ctx).Exec(ctx, `UPDATE session_schedule_proposals
		SET status = 'SUPERSEDED', responded_by_user_id = $2, responded_at = now()
		WHERE session_id = $1 AND status = 'PENDING'`, in.SessionID, in.ProposerID); err != nil {
		return SessionProposal{}, err
	}
	if _, err := p.conn(ctx).Exec(ctx, `DELETE FROM session_confirmations
		WHERE session_id = $1`, in.SessionID); err != nil {
		return SessionProposal{}, err
	}
	if _, err := p.conn(ctx).Exec(ctx, `INSERT INTO session_confirmations (session_id, user_id, kind)
		VALUES ($1, $2, 'SCHEDULE')
		ON CONFLICT (session_id, user_id, kind) DO UPDATE SET confirmed_at = now()`,
		in.SessionID, in.ProposerID); err != nil {
		return SessionProposal{}, err
	}
	var proposalID string
	if err := p.conn(ctx).QueryRow(ctx, `INSERT INTO session_schedule_proposals (
			session_id, proposed_by_user_id, proposed_start, proposed_end,
			format, meeting_place_id, meeting_url, status, created_at
		)
		VALUES (
			$1, $2, $3, $4, $5::lesson_format, NULLIF($6, '')::uuid, NULLIF($7, ''), 'PENDING',
			GREATEST(
				clock_timestamp(),
				COALESCE((SELECT MAX(created_at) + interval '1 microsecond' FROM session_schedule_proposals WHERE session_id = $1), clock_timestamp())
			)
		)
		RETURNING id`,
		in.SessionID, in.ProposerID, in.StartsAt, in.EndsAt, in.Format, meetingPlaceID, in.MeetingURL).Scan(&proposalID); err != nil {
		return SessionProposal{}, err
	}
	if _, err := p.conn(ctx).Exec(ctx, `UPDATE sessions
		SET title = $2,
			description = NULLIF($3, ''),
			teacher_id = $4,
			learner_id = $5,
			status = 'PROPOSED',
			scheduled_start = $6,
			scheduled_end = $7,
			format = $8::lesson_format,
			meeting_place_id = NULLIF($9, '')::uuid,
			meeting_url = NULLIF($10, ''),
			updated_at = now()
		WHERE id = $1`,
		in.SessionID, in.Title, in.Description, in.TeacherID, in.LearnerID,
		in.StartsAt, in.EndsAt, in.Format, meetingPlaceID, in.MeetingURL); err != nil {
		return SessionProposal{}, err
	}
	if _, err := p.conn(ctx).Exec(ctx, `UPDATE exchanges
		SET updated_at = now()
		WHERE id = (SELECT exchange_id FROM sessions WHERE id = $1)`, in.SessionID); err != nil {
		return SessionProposal{}, err
	}
	return p.proposalByID(ctx, proposalID)
}

func (p *Postgres) AcceptSessionProposal(ctx context.Context, sessionID, responderID string) (SessionProposal, error) {
	var proposalID string
	err := p.conn(ctx).QueryRow(ctx, `UPDATE session_schedule_proposals
		SET status = 'ACCEPTED', responded_by_user_id = $2, responded_at = now()
		WHERE session_id = $1 AND status = 'PENDING'
		RETURNING id`, sessionID, responderID).Scan(&proposalID)
	if err != nil {
		return SessionProposal{}, notFound(err)
	}
	if _, err := p.conn(ctx).Exec(ctx, `INSERT INTO session_confirmations (session_id, user_id, kind)
		VALUES ($1, $2, 'SCHEDULE')
		ON CONFLICT (session_id, user_id, kind) DO UPDATE SET confirmed_at = now()`,
		sessionID, responderID); err != nil {
		return SessionProposal{}, err
	}
	if _, err := p.conn(ctx).Exec(ctx, `UPDATE sessions
		SET status = 'SCHEDULED', updated_at = now()
		WHERE id = $1`, sessionID); err != nil {
		return SessionProposal{}, err
	}
	if _, err := p.conn(ctx).Exec(ctx, `UPDATE exchanges
		SET updated_at = now()
		WHERE id = (SELECT exchange_id FROM sessions WHERE id = $1)`, sessionID); err != nil {
		return SessionProposal{}, err
	}
	return p.proposalByID(ctx, proposalID)
}

func (p *Postgres) DeclineSessionProposal(ctx context.Context, sessionID, responderID string) (SessionProposal, error) {
	var proposalID string
	err := p.conn(ctx).QueryRow(ctx, `UPDATE session_schedule_proposals
		SET status = 'DECLINED', responded_by_user_id = $2, responded_at = now()
		WHERE session_id = $1 AND status = 'PENDING'
		RETURNING id`, sessionID, responderID).Scan(&proposalID)
	if err != nil {
		return SessionProposal{}, notFound(err)
	}
	if _, err := p.conn(ctx).Exec(ctx, `DELETE FROM session_confirmations
		WHERE session_id = $1 AND kind = 'SCHEDULE'`, sessionID); err != nil {
		return SessionProposal{}, err
	}
	if _, err := p.conn(ctx).Exec(ctx, `UPDATE sessions s
		SET status = 'DRAFT',
			scheduled_start = NULL,
			scheduled_end = NULL,
			format = (SELECT e.format FROM exchanges e WHERE e.id = s.exchange_id),
			meeting_place_id = NULL,
			meeting_url = NULL,
			updated_at = now()
		WHERE s.id = $1`, sessionID); err != nil {
		return SessionProposal{}, err
	}
	if _, err := p.conn(ctx).Exec(ctx, `UPDATE exchanges
		SET updated_at = now()
		WHERE id = (SELECT exchange_id FROM sessions WHERE id = $1)`, sessionID); err != nil {
		return SessionProposal{}, err
	}
	return p.proposalByID(ctx, proposalID)
}

func (p *Postgres) CancelSession(ctx context.Context, sessionID, cancelledByID, reason string) error {
	if _, err := p.conn(ctx).Exec(ctx, `UPDATE session_schedule_proposals
		SET status = 'SUPERSEDED', responded_by_user_id = $2, responded_at = now()
		WHERE session_id = $1 AND status = 'PENDING'`, sessionID, cancelledByID); err != nil {
		return err
	}
	if _, err := p.conn(ctx).Exec(ctx, `DELETE FROM session_confirmations
		WHERE session_id = $1`, sessionID); err != nil {
		return err
	}
	tag, err := p.conn(ctx).Exec(ctx, `WITH cancelled AS (
			UPDATE sessions
			SET status = 'CANCELLED',
				cancelled_at = now(),
				cancellation_reason = NULLIF($2, ''),
				updated_at = now()
			WHERE id = $1
			RETURNING id, exchange_id, teacher_id, learner_id, title
		), replacement AS (
			INSERT INTO sessions (exchange_id, plan_order, title, teacher_id, learner_id, status, format)
			SELECT c.exchange_id,
				(SELECT COALESCE(MAX(s2.plan_order), 0) + 1 FROM sessions s2 WHERE s2.exchange_id = c.exchange_id),
				COALESCE(
					(SELECT cm.skill_name_snapshot FROM exchange_skill_commitments cm
					 WHERE cm.exchange_id = c.exchange_id AND cm.teacher_id = c.teacher_id LIMIT 1),
					c.title
				) || ' — заняття ' || (
					(SELECT count(*) FROM sessions s3
					 WHERE s3.exchange_id = c.exchange_id AND s3.teacher_id = c.teacher_id
					   AND s3.status <> 'CANCELLED' AND s3.id <> c.id) + 1
				),
				c.teacher_id,
				c.learner_id,
				'DRAFT',
				e.format
			FROM cancelled c
			JOIN exchanges e ON e.id = c.exchange_id
		)
		UPDATE exchanges
		SET updated_at = now()
		WHERE id = (SELECT exchange_id FROM cancelled)`, sessionID, reason)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *Postgres) ConfirmSessionCompletion(ctx context.Context, sessionID, userID string) error {
	_, err := p.conn(ctx).Exec(ctx, `WITH confirmed AS (
			INSERT INTO session_confirmations (session_id, user_id, kind)
			VALUES ($1, $2, 'COMPLETION')
			ON CONFLICT (session_id, user_id, kind) DO NOTHING
			RETURNING session_id
		)
		UPDATE exchanges SET updated_at = now()
		WHERE id = (SELECT s.exchange_id FROM sessions s JOIN confirmed c ON c.session_id = s.id)`, sessionID, userID)
	return err
}

func (p *Postgres) CompleteSession(ctx context.Context, sessionID string) error {
	tag, err := p.conn(ctx).Exec(ctx, `WITH completed AS (
			UPDATE sessions
			SET status = 'COMPLETED', completed_at = now(), updated_at = now()
			WHERE id = $1 AND status = 'SCHEDULED'
			RETURNING exchange_id
		)
		UPDATE exchanges SET updated_at = now() WHERE id IN (SELECT exchange_id FROM completed)`, sessionID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *Postgres) UserSchedule(ctx context.Context, userID string, filter ScheduleFilter, limit, offset int) ([]Session, int, int, string, error) {
	timezone := "Europe/Kyiv"
	if err := p.conn(ctx).QueryRow(ctx, `SELECT COALESCE(NULLIF(timezone, ''), 'Europe/Kyiv')
		FROM users WHERE id = $1`, userID).Scan(&timezone); err != nil {
		return nil, 0, 0, "", notFound(err)
	}
	var pendingCount int
	if err := p.conn(ctx).QueryRow(ctx, `SELECT count(*)
		FROM sessions s
		JOIN exchanges e ON e.id = s.exchange_id AND e.status = 'ACTIVE'
		JOIN session_schedule_proposals sp ON sp.session_id = s.id AND sp.status = 'PENDING'
		WHERE (s.teacher_id = $1 OR s.learner_id = $1)
			AND s.status = 'PROPOSED'
			AND sp.proposed_by_user_id <> $1`, userID).Scan(&pendingCount); err != nil {
		return nil, 0, 0, "", err
	}

	status := filter.Status
	if status == "" {
		status = SessionScheduled
	}
	now := filter.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	where := ` WHERE (s.teacher_id = $1 OR s.learner_id = $1)
		AND (($2 = 'ALL' AND s.status <> 'DRAFT') OR ($2 <> 'ALL' AND s.status::text = $2))
		AND (NOT $3 OR (s.scheduled_end IS NOT NULL AND s.scheduled_end >= $4))`

	rows, err := p.conn(ctx).Query(ctx, `SELECT `+sessionColumns+`, count(*) OVER ()`+sessionFrom+where+`
		ORDER BY s.scheduled_start ASC NULLS LAST, s.plan_order ASC, s.id ASC
		LIMIT $5 OFFSET $6`, userID, status, filter.UpcomingOnly, now, limit, offset)
	if err != nil {
		return nil, 0, 0, "", err
	}
	total := 0
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Session, error) {
		var s Session
		err := row.Scan(s.scanTargets(&total)...)
		return s, err
	})
	if err != nil {
		return nil, 0, 0, "", err
	}
	if len(items) == 0 && offset > 0 {
		if err := p.conn(ctx).QueryRow(ctx, `SELECT count(*) FROM sessions s`+where,
			userID, status, filter.UpcomingOnly, now).Scan(&total); err != nil {
			return nil, 0, 0, "", err
		}
	}
	return items, total, pendingCount, timezone, nil
}
