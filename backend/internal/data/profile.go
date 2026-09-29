package data

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

const (
	FormatOnline  = "ONLINE"
	FormatOffline = "OFFLINE"
)

type SkillList string

const (
	TeachingList SkillList = "teaching"
	LearningList SkillList = "learning"
)

type Faculty struct {
	ID   string
	Name string
}

type UserSkill struct {
	SkillID    string
	CategoryID string
	Name       string
	Level      string
}

type Profile struct {
	UserID         string
	FirstName      string
	LastName       string
	UniversityID   string
	UniversityName string
	FacultyID      string // empty when no faculty is chosen
	FacultyName    string
	Course         int // 0 when the course is not set
	City           string
	Bio            string
	Formats        []string
	TeachingSkills []UserSkill
	LearningSkills []UserSkill
}

type ProfileUpdate struct {
	FirstName string
	LastName  string
	FacultyID string
	Course    int
	City      string
	Bio       string
	Formats   []string
}

type IProfileData interface {
	ProfileByUserID(ctx context.Context, userID string) (Profile, error)
	UpdateProfile(ctx context.Context, userID string, update ProfileUpdate) error
	FacultiesByUniversity(ctx context.Context, universityID string) ([]Faculty, error)
	ActiveSkillExists(ctx context.Context, skillID string) (bool, error)
	AddUserSkill(ctx context.Context, list SkillList, userID, skillID, level string) error
	UpdateUserSkillLevel(ctx context.Context, list SkillList, userID, skillID, level string) error
	RemoveUserSkill(ctx context.Context, list SkillList, userID, skillID string) error
}

var _ IProfileData = (*Postgres)(nil)

var skillTables = map[SkillList]struct{ table, level string }{
	TeachingList: {table: "user_teaching_skills", level: "level"},
	LearningList: {table: "user_learning_skills", level: "current_level"},
}

func (p *Postgres) ProfileByUserID(ctx context.Context, userID string) (Profile, error) {
	var profile Profile
	err := p.conn(ctx).QueryRow(ctx, `SELECT u.id, u.first_name, u.last_name, u.university_id, un.name,
			COALESCE(u.faculty_id::text, ''), COALESCE(f.name, ''), COALESCE(u.academic_year, 0),
			COALESCE(u.city, ''), COALESCE(u.bio, ''),
			ARRAY(SELECT lf.format::text FROM user_lesson_formats lf WHERE lf.user_id = u.id ORDER BY lf.format)
		FROM users u
		JOIN universities un ON un.id = u.university_id
		LEFT JOIN faculties f ON f.id = u.faculty_id
		WHERE u.id = $1 AND u.deleted_at IS NULL`, userID).Scan(
		&profile.UserID, &profile.FirstName, &profile.LastName, &profile.UniversityID, &profile.UniversityName,
		&profile.FacultyID, &profile.FacultyName, &profile.Course, &profile.City, &profile.Bio, &profile.Formats)
	if err != nil {
		return Profile{}, notFound(err)
	}
	if profile.TeachingSkills, err = p.userSkills(ctx, TeachingList, userID); err != nil {
		return Profile{}, err
	}
	if profile.LearningSkills, err = p.userSkills(ctx, LearningList, userID); err != nil {
		return Profile{}, err
	}
	return profile, nil
}

func (p *Postgres) userSkills(ctx context.Context, list SkillList, userID string) ([]UserSkill, error) {
	t := skillTables[list]
	rows, err := p.conn(ctx).Query(ctx, fmt.Sprintf(`SELECT us.skill_id, s.category_id, s.name, us.%s::text
		FROM %s us JOIN skills s ON s.id = us.skill_id
		WHERE us.user_id = $1 AND us.is_active
		ORDER BY s.name, s.id`, t.level, t.table), userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (UserSkill, error) {
		var item UserSkill
		err := row.Scan(&item.SkillID, &item.CategoryID, &item.Name, &item.Level)
		return item, err
	})
}

func (p *Postgres) UpdateProfile(ctx context.Context, userID string, update ProfileUpdate) error {
	return p.WithinTx(ctx, func(ctx context.Context) error {
		tag, err := p.conn(ctx).Exec(ctx, `UPDATE users SET first_name = $2, last_name = $3,
				faculty_id = NULLIF($4, '')::uuid, academic_year = NULLIF($5, 0),
				city = NULLIF($6, ''), bio = NULLIF($7, ''), updated_at = now()
			WHERE id = $1 AND deleted_at IS NULL`,
			userID, update.FirstName, update.LastName, update.FacultyID, update.Course, update.City, update.Bio)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		if _, err := p.conn(ctx).Exec(ctx, `DELETE FROM user_lesson_formats WHERE user_id = $1`, userID); err != nil {
			return err
		}
		_, err = p.conn(ctx).Exec(ctx, `INSERT INTO user_lesson_formats (user_id, format)
			SELECT $1::uuid, unnest($2::text[])::lesson_format`, userID, update.Formats)
		return err
	})
}

func (p *Postgres) FacultiesByUniversity(ctx context.Context, universityID string) ([]Faculty, error) {
	rows, err := p.conn(ctx).Query(ctx, `SELECT id, name FROM faculties
		WHERE university_id = $1 ORDER BY name, id`, universityID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Faculty, error) {
		var item Faculty
		err := row.Scan(&item.ID, &item.Name)
		return item, err
	})
}

func (p *Postgres) ActiveSkillExists(ctx context.Context, skillID string) (bool, error) {
	var exists bool
	err := p.conn(ctx).QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM skills s
		JOIN skill_categories c ON c.id = s.category_id
		WHERE s.id = $1 AND s.is_active AND c.is_active)`, skillID).Scan(&exists)
	return exists, err
}

func (p *Postgres) AddUserSkill(ctx context.Context, list SkillList, userID, skillID, level string) error {
	t := skillTables[list]
	tag, err := p.conn(ctx).Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s AS us (user_id, skill_id, %[2]s)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, skill_id) DO UPDATE
		SET %[2]s = EXCLUDED.%[2]s, is_active = true, updated_at = now()
		WHERE us.is_active = false`, t.table, t.level), userID, skillID, level)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrSkillAlreadyAdded
	}
	return nil
}

func (p *Postgres) UpdateUserSkillLevel(ctx context.Context, list SkillList, userID, skillID, level string) error {
	t := skillTables[list]
	tag, err := p.conn(ctx).Exec(ctx, fmt.Sprintf(`UPDATE %s SET %s = $3, updated_at = now()
		WHERE user_id = $1 AND skill_id = $2 AND is_active`, t.table, t.level), userID, skillID, level)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *Postgres) RemoveUserSkill(ctx context.Context, list SkillList, userID, skillID string) error {
	t := skillTables[list]
	tag, err := p.conn(ctx).Exec(ctx, fmt.Sprintf(`UPDATE %s SET is_active = false, updated_at = now()
		WHERE user_id = $1 AND skill_id = $2 AND is_active`, t.table), userID, skillID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
