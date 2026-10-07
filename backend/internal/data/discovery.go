package data

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Match is another student who forms a mutual match with the viewer
type Match struct {
	UserID         string
	FirstName      string
	LastName       string
	UniversityID   string
	UniversityName string
	City           string
	// CommonFormats are formats both students accept; OFFLINE only when they share a city.
	CommonFormats []string
	// CanTeach lists the student's teaching skills that the viewer wants to learn;
	// Level is the student's self-assessment.
	CanTeach []UserSkill
	// WantsToLearn lists the viewer's teaching skills that the student wants to learn;
	// Level is the student's current level.
	WantsToLearn []UserSkill
}

type IDiscoveryData interface {
	MutualMatches(ctx context.Context, viewerID string, limit, offset int) ([]Match, int, error)
}

var _ IDiscoveryData = (*Postgres)(nil)


const visibleStudent = `c.id <> $1 AND c.deleted_at IS NULL AND c.account_status = 'ACTIVE'
	AND EXISTS (SELECT 1 FROM user_verifications v WHERE v.user_id = c.id
		AND v.type = 'UNIVERSITY_EMAIL' AND v.identifier = c.email AND v.status = 'VERIFIED')
	AND NOT EXISTS (SELECT 1 FROM user_profile_settings ps WHERE ps.user_id = c.id AND NOT ps.is_discoverable)
	AND NOT EXISTS (SELECT 1 FROM user_blocks b
		WHERE (b.blocker_id = $1 AND b.blocked_id = c.id) OR (b.blocker_id = c.id AND b.blocked_id = $1))`


const usableFormat = `FROM user_lesson_formats f
	JOIN user_lesson_formats vf ON vf.format = f.format AND vf.user_id = $1
	WHERE f.user_id = c.id AND (f.format <> 'OFFLINE'
		OR NULLIF(lower(btrim(c.city)), '') = (SELECT NULLIF(lower(btrim(u.city)), '') FROM users u WHERE u.id = $1))`


const mutualMatches = `WITH can_teach AS (
	SELECT t.user_id, t.skill_id, t.level::text AS level
	FROM user_teaching_skills t
	JOIN user_learning_skills l ON l.skill_id = t.skill_id AND l.user_id = $1 AND l.is_active
	JOIN skills s ON s.id = t.skill_id AND s.is_active
	WHERE t.is_active AND t.user_id <> $1
), wants_to_learn AS (
	SELECT l.user_id, l.skill_id, l.current_level::text AS level
	FROM user_learning_skills l
	JOIN user_teaching_skills t ON t.skill_id = l.skill_id AND t.user_id = $1 AND t.is_active
	JOIN skills s ON s.id = l.skill_id AND s.is_active
	WHERE l.is_active AND l.user_id <> $1
), matches AS (
	SELECT c.id, c.first_name, c.last_name, c.university_id, c.city,
		(SELECT count(*) FROM can_teach x WHERE x.user_id = c.id)
			+ (SELECT count(*) FROM wants_to_learn x WHERE x.user_id = c.id) AS pairs
	FROM users c
	WHERE c.id IN (SELECT user_id FROM can_teach) AND c.id IN (SELECT user_id FROM wants_to_learn)
		AND ` + visibleStudent + `
		AND EXISTS (SELECT 1 ` + usableFormat + `)
)
`

func matchedSkills(cte string) string {
	return `(SELECT COALESCE(json_agg(json_build_object('SkillID', s.id, 'CategoryID', s.category_id,
		'Name', s.name, 'Level', x.level) ORDER BY s.name, s.id), '[]')
		FROM ` + cte + ` x JOIN skills s ON s.id = x.skill_id WHERE x.user_id = c.id)`
}

func (p *Postgres) MutualMatches(ctx context.Context, viewerID string, limit, offset int) ([]Match, int, error) {
	rows, err := p.conn(ctx).Query(ctx, mutualMatches+`SELECT c.id, c.first_name, c.last_name, c.university_id,
			un.name, COALESCE(c.city, ''),
			ARRAY(SELECT f.format::text `+usableFormat+` ORDER BY f.format),
			`+matchedSkills("can_teach")+`, `+matchedSkills("wants_to_learn")+`,
			count(*) OVER ()
		FROM matches c
		JOIN universities un ON un.id = c.university_id
		ORDER BY c.pairs DESC, c.first_name, c.last_name, c.id
		LIMIT $2 OFFSET $3`, viewerID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	total := 0
	matches, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Match, error) {
		var m Match
		err := row.Scan(&m.UserID, &m.FirstName, &m.LastName, &m.UniversityID, &m.UniversityName, &m.City,
			&m.CommonFormats, &m.CanTeach, &m.WantsToLearn, &total)
		return m, err
	})
	if err != nil {
		return nil, 0, err
	}
	// A page past the end has no rows to carry the total, so count separately.
	if len(matches) == 0 && offset > 0 {
		err = p.conn(ctx).QueryRow(ctx, mutualMatches+`SELECT count(*) FROM matches`, viewerID).Scan(&total)
	}
	return matches, total, err
}
