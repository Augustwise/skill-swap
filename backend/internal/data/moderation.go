package data

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	AccountActive    = "ACTIVE"
	AccountSuspended = "SUSPENDED"

	RoleAdmin = "ADMIN"

	ReportOpen      = "OPEN"
	ReportResolved  = "RESOLVED"
	ReportDismissed = "DISMISSED"

	ActionDismissReport  = "DISMISS_REPORT"
	ActionResolveReport  = "RESOLVE_REPORT"
	ActionSuspendAccount = "SUSPEND_ACCOUNT"
	ActionRestoreAccount = "RESTORE_ACCOUNT"
	ActionCreateSkill    = "CREATE_SKILL"
	ActionUpdateSkill    = "UPDATE_SKILL"

	// DeletedFirstName and DeletedLastName replace the name of a deleted account (FR-15).
	DeletedFirstName = "Видалений"
	DeletedLastName  = "користувач"
	// DeletedMessageBody replaces the text of every message written by a deleted account.
	DeletedMessageBody = "Повідомлення видалено"
)

var (
	ErrReportExists     = errors.New("an open report about this user already exists")
	ErrSkillExists      = errors.New("a skill with this name or slug already exists")
	ErrCategoryNotFound = errors.New("skill category does not exist")
)

type UserRef struct {
	ID        string
	FirstName string
	LastName  string
}

type SkillRef struct {
	ID   string
	Name string
}

// Block is a personal block made by the current user.
type Block struct {
	User      UserRef
	CreatedAt time.Time
}

type Report struct {
	ID           string
	Reporter     UserRef
	ReportedUser UserRef
	// ReportedAccountStatus is ACTIVE or SUSPENDED.
	ReportedAccountStatus string
	Reason                string
	Status                string
	CreatedAt             time.Time
	ResolvedAt            *time.Time
	// Decision is the moderation action that closed the report; nil while it is open.
	Decision *ModerationAction
}

// ModerationAction is one entry of the administrator log: who did what, when and why.
type ModerationAction struct {
	ID        string
	ReportID  string // empty when the action does not decide a report
	Moderator UserRef
	Subject   *UserRef  // nil for catalog changes
	Skill     *SkillRef // nil for account actions
	Action    string
	Reason    string
	CreatedAt time.Time
}

type NewModerationAction struct {
	ReportID      string
	ModeratorID   string
	SubjectUserID string
	SkillID       string
	Action        string
	Reason        string
}

// CatalogSkill is a skill as the administrator sees it, active or not.
type CatalogSkill struct {
	ID           string
	CategoryID   string
	CategoryName string
	Name         string
	Slug         string
	Description  string
	IsActive     bool
}

type SkillInput struct {
	CategoryID  string
	Name        string
	Slug        string
	Description string
	IsActive    bool
}

type IModerationData interface {
	LockUsers(ctx context.Context, userA, userB string) error
	UserByID(ctx context.Context, userID string) (User, error)
	RevokeUserSessions(ctx context.Context, userID string) error
	KnownStudent(ctx context.Context, viewerID, studentID string) (bool, error)

	CreateBlock(ctx context.Context, blockerID, blockedID string) (bool, error)
	DeleteBlock(ctx context.Context, blockerID, blockedID string) error
	BlockByUsers(ctx context.Context, blockerID, blockedID string) (Block, error)
	UserBlocks(ctx context.Context, blockerID string, limit, offset int) ([]Block, int, error)
	ActiveExchangeIDs(ctx context.Context, userID, partnerID string) ([]string, error)
	PendingRequests(ctx context.Context, userID, partnerID string) ([]RequestState, error)

	CreateReport(ctx context.Context, reporterID, reportedUserID, reason string) (string, error)
	ReportByID(ctx context.Context, reportID string) (Report, error)
	LockReport(ctx context.Context, reportID string) (Report, error)
	UserReports(ctx context.Context, reporterID string, limit, offset int) ([]Report, int, error)
	Reports(ctx context.Context, status string, limit, offset int) ([]Report, int, error)
	CloseReport(ctx context.Context, reportID, status string) error
	CreateModerationAction(ctx context.Context, action NewModerationAction) (string, error)
	ModerationActionByID(ctx context.Context, actionID string) (ModerationAction, error)
	ModerationActions(ctx context.Context, subjectUserID string, limit, offset int) ([]ModerationAction, int, error)
	UpdateAccountStatus(ctx context.Context, userID, status string) error
	AnonymizeUser(ctx context.Context, userID string) error

	CatalogSkills(ctx context.Context, query string, limit, offset int) ([]CatalogSkill, int, error)
	CatalogSkillByID(ctx context.Context, skillID string) (CatalogSkill, error)
	CreateSkill(ctx context.Context, skill SkillInput) (string, error)
	UpdateSkill(ctx context.Context, skillID string, skill SkillInput) error
}

var _ IModerationData = (*Postgres)(nil)

// KnownStudent reports whether the viewer may block or report the student: the student is
// visible to the viewer, already blocked by the viewer, or shares a request or exchange with them.
func (p *Postgres) KnownStudent(ctx context.Context, viewerID, studentID string) (bool, error) {
	var known bool
	err := p.conn(ctx).QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users c
		WHERE c.id = $2 AND c.id <> $1 AND c.deleted_at IS NULL AND (
			(`+visibleStudent+`)
			OR EXISTS (SELECT 1 FROM user_blocks b WHERE b.blocker_id = $1 AND b.blocked_id = c.id)
			OR EXISTS (SELECT 1 FROM exchange_requests r
				WHERE (r.requester_id = $1 AND r.recipient_id = c.id) OR (r.requester_id = c.id AND r.recipient_id = $1))
			OR EXISTS (SELECT 1 FROM exchanges e
				WHERE (e.user_a_id = $1 AND e.user_b_id = c.id) OR (e.user_a_id = c.id AND e.user_b_id = $1))
		))`, viewerID, studentID).Scan(&known)
	return known, err
}

// CreateBlock reports false when the block already exists.
func (p *Postgres) CreateBlock(ctx context.Context, blockerID, blockedID string) (bool, error) {
	tag, err := p.conn(ctx).Exec(ctx, `INSERT INTO user_blocks (blocker_id, blocked_id) VALUES ($1, $2)
		ON CONFLICT (blocker_id, blocked_id) DO NOTHING`, blockerID, blockedID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (p *Postgres) DeleteBlock(ctx context.Context, blockerID, blockedID string) error {
	_, err := p.conn(ctx).Exec(ctx, `DELETE FROM user_blocks WHERE blocker_id = $1 AND blocked_id = $2`, blockerID, blockedID)
	return err
}

const blockSelect = `SELECT json_build_object('ID', u.id, 'FirstName', u.first_name, 'LastName', u.last_name), b.created_at
	FROM user_blocks b
	JOIN users u ON u.id = b.blocked_id`

func (p *Postgres) BlockByUsers(ctx context.Context, blockerID, blockedID string) (Block, error) {
	var b Block
	err := p.conn(ctx).QueryRow(ctx, blockSelect+` WHERE b.blocker_id = $1 AND b.blocked_id = $2`,
		blockerID, blockedID).Scan(&b.User, &b.CreatedAt)
	if err != nil {
		return Block{}, notFound(err)
	}
	return b, nil
}

// UserBlocks returns one page of the users the blocker has blocked, newest first.
func (p *Postgres) UserBlocks(ctx context.Context, blockerID string, limit, offset int) ([]Block, int, error) {
	var total int
	if err := p.conn(ctx).QueryRow(ctx, `SELECT count(*) FROM user_blocks WHERE blocker_id = $1`, blockerID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := p.conn(ctx).Query(ctx, blockSelect+` WHERE b.blocker_id = $1
		ORDER BY b.created_at DESC, b.blocked_id DESC
		LIMIT $2 OFFSET $3`, blockerID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	blocks, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Block, error) {
		var b Block
		err := row.Scan(&b.User, &b.CreatedAt)
		return b, err
	})
	return blocks, total, err
}

// ActiveExchangeIDs lists the active exchanges of the user; an empty partnerID means with anyone.
func (p *Postgres) ActiveExchangeIDs(ctx context.Context, userID, partnerID string) ([]string, error) {
	rows, err := p.conn(ctx).Query(ctx, `SELECT id::text FROM exchanges
		WHERE status = 'ACTIVE' AND (
			(user_a_id = $1 AND ($2 = '' OR user_b_id::text = $2))
			OR (user_b_id = $1 AND ($2 = '' OR user_a_id::text = $2)))
		ORDER BY id`, userID, partnerID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// PendingRequests lists the pending requests of the user; an empty partnerID means with anyone.
func (p *Postgres) PendingRequests(ctx context.Context, userID, partnerID string) ([]RequestState, error) {
	rows, err := p.conn(ctx).Query(ctx, `SELECT id, requester_id, recipient_id, status::text FROM exchange_requests
		WHERE status = 'PENDING' AND (
			(requester_id = $1 AND ($2 = '' OR recipient_id::text = $2))
			OR (recipient_id = $1 AND ($2 = '' OR requester_id::text = $2)))
		ORDER BY id`, userID, partnerID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (RequestState, error) {
		var r RequestState
		err := row.Scan(&r.ID, &r.RequesterID, &r.RecipientID, &r.Status)
		return r, err
	})
}

// CreateReport returns ErrReportExists while the reporter has an open report about the same user.
func (p *Postgres) CreateReport(ctx context.Context, reporterID, reportedUserID, reason string) (string, error) {
	var id string
	err := p.conn(ctx).QueryRow(ctx, `INSERT INTO reports (reporter_id, reported_user_id, reason)
		VALUES ($1, $2, $3) RETURNING id`, reporterID, reportedUserID, reason).Scan(&id)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "idx_reports_open_pair" {
		return "", ErrReportExists
	}
	return id, err
}

const moderationActionObject = `json_build_object(
	'ID', ma.id,
	'ReportID', COALESCE(ma.report_id::text, ''),
	'Moderator', json_build_object('ID', mo.id, 'FirstName', mo.first_name, 'LastName', mo.last_name),
	'Subject', CASE WHEN su.id IS NULL THEN NULL
		ELSE json_build_object('ID', su.id, 'FirstName', su.first_name, 'LastName', su.last_name) END,
	'Skill', CASE WHEN sk.id IS NULL THEN NULL ELSE json_build_object('ID', sk.id, 'Name', sk.name) END,
	'Action', ma.action,
	'Reason', ma.reason,
	'CreatedAt', ma.created_at
)`

const moderationActionFrom = `
	FROM moderation_actions ma
	JOIN users mo ON mo.id = ma.moderator_id
	LEFT JOIN users su ON su.id = ma.subject_user_id
	LEFT JOIN skills sk ON sk.id = ma.skill_id`

const reportSelect = `SELECT r.id,
		json_build_object('ID', rp.id, 'FirstName', rp.first_name, 'LastName', rp.last_name),
		json_build_object('ID', ru.id, 'FirstName', ru.first_name, 'LastName', ru.last_name),
		ru.account_status::text, r.reason, r.status::text, r.created_at, r.resolved_at,
		(SELECT ` + moderationActionObject + moderationActionFrom + `
			WHERE ma.report_id = r.id ORDER BY ma.created_at DESC, ma.id DESC LIMIT 1)
	FROM reports r
	JOIN users rp ON rp.id = r.reporter_id
	JOIN users ru ON ru.id = r.reported_user_id`

func (r *Report) scanTargets() []any {
	return []any{&r.ID, &r.Reporter, &r.ReportedUser, &r.ReportedAccountStatus, &r.Reason, &r.Status,
		&r.CreatedAt, &r.ResolvedAt, &r.Decision}
}

func (p *Postgres) ReportByID(ctx context.Context, reportID string) (Report, error) {
	var r Report
	if err := p.conn(ctx).QueryRow(ctx, reportSelect+` WHERE r.id = $1`, reportID).Scan(r.scanTargets()...); err != nil {
		return Report{}, notFound(err)
	}
	return r, nil
}

// LockReport holds the report row until the surrounding transaction ends.
func (p *Postgres) LockReport(ctx context.Context, reportID string) (Report, error) {
	var id string
	if err := p.conn(ctx).QueryRow(ctx, `SELECT id FROM reports WHERE id = $1 FOR UPDATE`, reportID).Scan(&id); err != nil {
		return Report{}, notFound(err)
	}
	return p.ReportByID(ctx, id)
}

// reportPage loads one page of reports; where filters by $1 and the page uses $2 and $3.
func (p *Postgres) reportPage(ctx context.Context, where string, filter any, limit, offset int) ([]Report, int, error) {
	var total int
	if err := p.conn(ctx).QueryRow(ctx, `SELECT count(*) FROM reports r `+where, filter).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := p.conn(ctx).Query(ctx, reportSelect+` `+where+`
		ORDER BY r.created_at DESC, r.id DESC
		LIMIT $2 OFFSET $3`, filter, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	reports, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Report, error) {
		var r Report
		err := row.Scan(r.scanTargets()...)
		return r, err
	})
	return reports, total, err
}

// UserReports returns one page of the reports the user has sent, newest first.
func (p *Postgres) UserReports(ctx context.Context, reporterID string, limit, offset int) ([]Report, int, error) {
	return p.reportPage(ctx, `WHERE r.reporter_id = $1`, reporterID, limit, offset)
}

// Reports returns one page of all reports with the status, or with any status when it is empty.
func (p *Postgres) Reports(ctx context.Context, status string, limit, offset int) ([]Report, int, error) {
	return p.reportPage(ctx, `WHERE ($1 = '' OR r.status::text = $1)`, status, limit, offset)
}

// CloseReport moves an open report to RESOLVED or DISMISSED.
func (p *Postgres) CloseReport(ctx context.Context, reportID, status string) error {
	tag, err := p.conn(ctx).Exec(ctx, `UPDATE reports SET status = $2::report_status, resolved_at = now()
		WHERE id = $1 AND status = 'OPEN'`, reportID, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *Postgres) CreateModerationAction(ctx context.Context, a NewModerationAction) (string, error) {
	var id string
	err := p.conn(ctx).QueryRow(ctx, `INSERT INTO moderation_actions (report_id, moderator_id, subject_user_id, skill_id, action, reason)
		VALUES (NULLIF($1, '')::uuid, $2, NULLIF($3, '')::uuid, NULLIF($4, '')::uuid, $5, $6)
		RETURNING id`, a.ReportID, a.ModeratorID, a.SubjectUserID, a.SkillID, a.Action, a.Reason).Scan(&id)
	return id, err
}

func (p *Postgres) ModerationActionByID(ctx context.Context, actionID string) (ModerationAction, error) {
	var a ModerationAction
	err := p.conn(ctx).QueryRow(ctx, `SELECT `+moderationActionObject+moderationActionFrom+` WHERE ma.id = $1`, actionID).Scan(&a)
	if err != nil {
		return ModerationAction{}, notFound(err)
	}
	return a, nil
}

// ModerationActions returns one page of the log, newest first; an empty subjectUserID means all entries.
func (p *Postgres) ModerationActions(ctx context.Context, subjectUserID string, limit, offset int) ([]ModerationAction, int, error) {
	const where = ` WHERE ($1 = '' OR ma.subject_user_id::text = $1)`
	var total int
	if err := p.conn(ctx).QueryRow(ctx, `SELECT count(*) FROM moderation_actions ma`+where, subjectUserID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := p.conn(ctx).Query(ctx, `SELECT `+moderationActionObject+moderationActionFrom+where+`
		ORDER BY ma.created_at DESC, ma.id DESC
		LIMIT $2 OFFSET $3`, subjectUserID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	actions, err := pgx.CollectRows(rows, pgx.RowTo[ModerationAction])
	return actions, total, err
}

func (p *Postgres) UpdateAccountStatus(ctx context.Context, userID, status string) error {
	tag, err := p.conn(ctx).Exec(ctx, `UPDATE users SET account_status = $2::account_status, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL`, userID, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// AnonymizeUser deletes the account in the local MVP (FR-15): sign-in stops working, sessions
// and links are revoked, the name, email and profile contacts are cleared, and the user's own
// chat messages are replaced with a marker. Requests, exchanges and reviews stay for the partners.
func (p *Postgres) AnonymizeUser(ctx context.Context, userID string) error {
	return p.WithinTx(ctx, func(ctx context.Context) error {
		var email string
		err := p.conn(ctx).QueryRow(ctx, `SELECT email FROM users WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, userID).Scan(&email)
		if err != nil {
			return notFound(err)
		}
		steps := []struct {
			query string
			args  []any
		}{
			{`UPDATE users SET email = 'deleted-' || id || '@deleted.invalid', first_name = $2, last_name = $3,
				username = NULL, faculty_id = NULL, academic_year = NULL, birth_date = NULL, city = NULL, bio = NULL,
				avatar_url = NULL, phone_e164 = NULL, telegram_handle = NULL, portfolio_url = NULL,
				deleted_at = now(), updated_at = now()
			WHERE id = $1`, []any{userID, DeletedFirstName, DeletedLastName}},
			{`DELETE FROM auth_identities WHERE user_id = $1`, []any{userID}},
			{`UPDATE user_sessions SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, []any{userID}},
			{`UPDATE one_time_tokens SET used_at = now() WHERE user_id = $1 AND used_at IS NULL`, []any{userID}},
			{`DELETE FROM user_verifications WHERE user_id = $1`, []any{userID}},
			{`DELETE FROM user_lesson_formats WHERE user_id = $1`, []any{userID}},
			{`UPDATE user_teaching_skills SET is_active = false, updated_at = now() WHERE user_id = $1 AND is_active`, []any{userID}},
			{`UPDATE user_learning_skills SET is_active = false, updated_at = now() WHERE user_id = $1 AND is_active`, []any{userID}},
			{`INSERT INTO user_profile_settings (user_id, is_discoverable, accepts_requests) VALUES ($1, false, false)
			ON CONFLICT (user_id) DO UPDATE SET is_discoverable = false, accepts_requests = false, updated_at = now()`, []any{userID}},
			{`DELETE FROM user_blocks WHERE blocker_id = $1 OR blocked_id = $1`, []any{userID}},
			{`UPDATE messages SET body = $2 WHERE author_id = $1 AND type = 'TEXT'`, []any{userID, DeletedMessageBody}},
			{`UPDATE exchange_requests SET message = NULL WHERE requester_id = $1`, []any{userID}},
			// Notification texts quote the author's name and messages.
			{`UPDATE notifications SET body = NULL WHERE actor_id = $1`, []any{userID}},
			{`DELETE FROM login_throttles WHERE email = $1`, []any{email}},
		}
		for _, step := range steps {
			if _, err := p.conn(ctx).Exec(ctx, step.query, step.args...); err != nil {
				return err
			}
		}
		return nil
	})
}

const catalogSkillSelect = `SELECT s.id, s.category_id, c.name, s.name, s.slug, COALESCE(s.description, ''), s.is_active
	FROM skills s
	JOIN skill_categories c ON c.id = s.category_id`

func scanCatalogSkill(row pgx.Row) (CatalogSkill, error) {
	var s CatalogSkill
	err := row.Scan(&s.ID, &s.CategoryID, &s.CategoryName, &s.Name, &s.Slug, &s.Description, &s.IsActive)
	return s, err
}

// CatalogSkills returns one page of all skills, including inactive ones, by category and name.
func (p *Postgres) CatalogSkills(ctx context.Context, query string, limit, offset int) ([]CatalogSkill, int, error) {
	const where = ` WHERE ($1 = '' OR strpos(lower(s.name), lower($1)) > 0 OR strpos(s.slug, lower($1)) > 0)`
	var total int
	if err := p.conn(ctx).QueryRow(ctx, `SELECT count(*) FROM skills s`+where, query).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := p.conn(ctx).Query(ctx, catalogSkillSelect+where+`
		ORDER BY c.sort_order, c.name, s.name, s.id
		LIMIT $2 OFFSET $3`, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	skills, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (CatalogSkill, error) { return scanCatalogSkill(row) })
	return skills, total, err
}

func (p *Postgres) CatalogSkillByID(ctx context.Context, skillID string) (CatalogSkill, error) {
	s, err := scanCatalogSkill(p.conn(ctx).QueryRow(ctx, catalogSkillSelect+` WHERE s.id = $1`, skillID))
	if err != nil {
		return CatalogSkill{}, notFound(err)
	}
	return s, nil
}

func (p *Postgres) CreateSkill(ctx context.Context, s SkillInput) (string, error) {
	var id string
	err := p.conn(ctx).QueryRow(ctx, `INSERT INTO skills (category_id, name, slug, description, is_active)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5) RETURNING id`,
		s.CategoryID, s.Name, s.Slug, s.Description, s.IsActive).Scan(&id)
	return id, catalogError(err)
}

func (p *Postgres) UpdateSkill(ctx context.Context, skillID string, s SkillInput) error {
	tag, err := p.conn(ctx).Exec(ctx, `UPDATE skills
		SET category_id = $2, name = $3, slug = $4, description = NULLIF($5, ''), is_active = $6
		WHERE id = $1`, skillID, s.CategoryID, s.Name, s.Slug, s.Description, s.IsActive)
	if err != nil {
		return catalogError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// catalogError maps a duplicate name or slug and an unknown category to domain errors.
func catalogError(err error) error {
	var pgErr *pgconn.PgError
	switch {
	case errors.As(err, &pgErr) && pgErr.Code == "23505":
		return ErrSkillExists
	case errors.As(err, &pgErr) && pgErr.Code == "23503":
		return ErrCategoryNotFound
	}
	return err
}
