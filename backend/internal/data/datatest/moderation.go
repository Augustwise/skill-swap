package datatest

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"skillswap/backend/internal/data"
)

var _ data.IModerationData = (*Memory)(nil)

var categoryNames = map[string]string{
	"20000000-0000-0000-0000-000000000001": "Музика",
	"20000000-0000-0000-0000-000000000002": "Дизайн",
}

type storedReport struct {
	id         string
	reporterID string
	reportedID string
	reason     string
	status     string
	createdAt  time.Time
	resolvedAt *time.Time
}

type storedAction struct {
	id        string
	action    data.NewModerationAction
	createdAt time.Time
}

// SetAdmin gives the user the administrator role.
func (m *Memory) SetAdmin(userID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if a, ok := m.users[userID]; ok {
		a.user.Role = data.RoleAdmin
	}
}

// IsDeleted reports whether the account was deleted and anonymized.
func (m *Memory) IsDeleted(userID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.users[userID]
	return ok && a.deleted
}

func (m *Memory) LockUsers(_ context.Context, userA, userB string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.users[userA] == nil && m.users[userB] == nil {
		return data.ErrNotFound
	}
	return nil
}

func (m *Memory) KnownStudent(_ context.Context, viewerID, studentID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := m.users[studentID]
	if c == nil || c.deleted || studentID == viewerID {
		return false, nil
	}
	if _, blocked := m.blocks[[2]string{viewerID, studentID}]; blocked || m.visible(viewerID, studentID, c) {
		return true, nil
	}
	for _, r := range m.requests {
		if pairOf(r.Requester.UserID, r.Recipient.UserID) == pairOf(viewerID, studentID) {
			return true, nil
		}
	}
	return false, nil
}

func pairOf(a, b string) [2]string {
	return [2]string{min(a, b), max(a, b)}
}

func (m *Memory) CreateBlock(_ context.Context, blockerID, blockedID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := [2]string{blockerID, blockedID}
	if _, ok := m.blocks[key]; ok {
		return false, nil
	}
	m.blocks[key] = time.Now().UTC()
	return true, nil
}

func (m *Memory) DeleteBlock(_ context.Context, blockerID, blockedID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.blocks, [2]string{blockerID, blockedID})
	return nil
}

// userRef must be called with m.mu held.
func (m *Memory) userRef(userID string) data.UserRef {
	ref := data.UserRef{ID: userID}
	if a, ok := m.users[userID]; ok {
		ref.FirstName, ref.LastName = a.profile.FirstName, a.profile.LastName
	}
	return ref
}

func (m *Memory) BlockByUsers(_ context.Context, blockerID, blockedID string) (data.Block, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	at, ok := m.blocks[[2]string{blockerID, blockedID}]
	if !ok {
		return data.Block{}, data.ErrNotFound
	}
	return data.Block{User: m.userRef(blockedID), CreatedAt: at}, nil
}

func (m *Memory) UserBlocks(_ context.Context, blockerID string, limit, offset int) ([]data.Block, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	found := []data.Block{}
	for key, at := range m.blocks {
		if key[0] == blockerID {
			found = append(found, data.Block{User: m.userRef(key[1]), CreatedAt: at})
		}
	}
	slices.SortFunc(found, func(a, b data.Block) int {
		return cmp.Or(b.CreatedAt.Compare(a.CreatedAt), cmp.Compare(b.User.ID, a.User.ID))
	})
	total := len(found)
	return append([]data.Block{}, found[min(offset, total):min(offset+limit, total)]...), total, nil
}

// withPartner reports whether a pair of participants includes the user and, when it is set, the partner.
func withPartner(a, b, userID, partnerID string) bool {
	return (a == userID && (partnerID == "" || b == partnerID)) || (b == userID && (partnerID == "" || a == partnerID))
}

func (m *Memory) ActiveExchangeIDs(_ context.Context, userID, partnerID string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := []string{}
	for id, e := range m.exchanges {
		if e.Status == data.ExchangeActive && withPartner(e.Requester.UserID, e.Recipient.UserID, userID, partnerID) {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return ids, nil
}

func (m *Memory) PendingRequests(_ context.Context, userID, partnerID string) ([]data.RequestState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	found := []data.RequestState{}
	for id, r := range m.requests {
		if r.Status == data.RequestPending && withPartner(r.Requester.UserID, r.Recipient.UserID, userID, partnerID) {
			found = append(found, data.RequestState{ID: id, RequesterID: r.Requester.UserID, RecipientID: r.Recipient.UserID, Status: r.Status})
		}
	}
	slices.SortFunc(found, func(a, b data.RequestState) int { return cmp.Compare(a.ID, b.ID) })
	return found, nil
}

func (m *Memory) CreateReport(_ context.Context, reporterID, reportedUserID, reason string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.reports {
		if r.reporterID == reporterID && r.reportedID == reportedUserID && r.status == data.ReportOpen {
			return "", data.ErrReportExists
		}
	}
	now := time.Now().UTC()
	for _, r := range m.reports {
		if !now.After(r.createdAt) {
			now = r.createdAt.Add(time.Microsecond)
		}
	}
	id := fmt.Sprintf("a0000000-0000-0000-0000-%012d", len(m.reports)+1)
	m.reports[id] = &storedReport{id: id, reporterID: reporterID, reportedID: reportedUserID, reason: reason,
		status: data.ReportOpen, createdAt: now}
	return id, nil
}

// hydrateAction must be called with m.mu held.
func (m *Memory) hydrateAction(a storedAction) data.ModerationAction {
	out := data.ModerationAction{
		ID: a.id, ReportID: a.action.ReportID, Moderator: m.userRef(a.action.ModeratorID),
		Action: a.action.Action, Reason: a.action.Reason, CreatedAt: a.createdAt,
	}
	if a.action.SubjectUserID != "" {
		subject := m.userRef(a.action.SubjectUserID)
		out.Subject = &subject
	}
	if a.action.SkillID != "" {
		out.Skill = &data.SkillRef{ID: a.action.SkillID, Name: m.skills[a.action.SkillID].skill.Name}
	}
	return out
}

// hydrateReport must be called with m.mu held.
func (m *Memory) hydrateReport(r *storedReport) data.Report {
	out := data.Report{
		ID: r.id, Reporter: m.userRef(r.reporterID), ReportedUser: m.userRef(r.reportedID),
		Reason: r.reason, Status: r.status, CreatedAt: r.createdAt, ResolvedAt: r.resolvedAt,
	}
	if a, ok := m.users[r.reportedID]; ok {
		out.ReportedAccountStatus = a.user.Status
	}
	for _, a := range m.moderationActions {
		if a.action.ReportID == r.id {
			decision := m.hydrateAction(a)
			out.Decision = &decision
		}
	}
	return out
}

func (m *Memory) ReportByID(_ context.Context, reportID string) (data.Report, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.reports[reportID]
	if !ok {
		return data.Report{}, data.ErrNotFound
	}
	return m.hydrateReport(r), nil
}

func (m *Memory) LockReport(ctx context.Context, reportID string) (data.Report, error) {
	return m.ReportByID(ctx, reportID)
}

// reportPage must be called with m.mu held.
func (m *Memory) reportPage(keep func(*storedReport) bool, limit, offset int) ([]data.Report, int) {
	found := []data.Report{}
	for _, r := range m.reports {
		if keep(r) {
			found = append(found, m.hydrateReport(r))
		}
	}
	slices.SortFunc(found, func(a, b data.Report) int {
		return cmp.Or(b.CreatedAt.Compare(a.CreatedAt), cmp.Compare(b.ID, a.ID))
	})
	total := len(found)
	return append([]data.Report{}, found[min(offset, total):min(offset+limit, total)]...), total
}

func (m *Memory) UserReports(_ context.Context, reporterID string, limit, offset int) ([]data.Report, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	items, total := m.reportPage(func(r *storedReport) bool { return r.reporterID == reporterID }, limit, offset)
	return items, total, nil
}

func (m *Memory) Reports(_ context.Context, status string, limit, offset int) ([]data.Report, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	items, total := m.reportPage(func(r *storedReport) bool { return status == "" || r.status == status }, limit, offset)
	return items, total, nil
}

func (m *Memory) CloseReport(_ context.Context, reportID, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.reports[reportID]
	if !ok || r.status != data.ReportOpen {
		return data.ErrNotFound
	}
	now := time.Now().UTC()
	r.status, r.resolvedAt = status, &now
	return nil
}

func (m *Memory) CreateModerationAction(_ context.Context, a data.NewModerationAction) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	if n := len(m.moderationActions); n > 0 && !now.After(m.moderationActions[n-1].createdAt) {
		now = m.moderationActions[n-1].createdAt.Add(time.Microsecond)
	}
	id := fmt.Sprintf("b0000000-0000-0000-0000-%012d", len(m.moderationActions)+1)
	m.moderationActions = append(m.moderationActions, storedAction{id: id, action: a, createdAt: now})
	return id, nil
}

func (m *Memory) ModerationActionByID(_ context.Context, actionID string) (data.ModerationAction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range m.moderationActions {
		if a.id == actionID {
			return m.hydrateAction(a), nil
		}
	}
	return data.ModerationAction{}, data.ErrNotFound
}

func (m *Memory) ModerationActions(_ context.Context, subjectUserID string, limit, offset int) ([]data.ModerationAction, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	found := []data.ModerationAction{}
	for i := len(m.moderationActions) - 1; i >= 0; i-- {
		a := m.moderationActions[i]
		if subjectUserID == "" || a.action.SubjectUserID == subjectUserID {
			found = append(found, m.hydrateAction(a))
		}
	}
	total := len(found)
	return append([]data.ModerationAction{}, found[min(offset, total):min(offset+limit, total)]...), total, nil
}

func (m *Memory) UpdateAccountStatus(_ context.Context, userID, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.users[userID]
	if !ok || a.deleted {
		return data.ErrNotFound
	}
	a.user.Status = status
	return nil
}

func (m *Memory) AnonymizeUser(_ context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.users[userID]
	if !ok || a.deleted {
		return data.ErrNotFound
	}
	delete(m.throttles, a.user.Email)
	a.deleted = true
	a.passwordHash = ""
	a.user.Email = "deleted-" + userID + "@deleted.invalid"
	a.user.FirstName, a.user.LastName = data.DeletedFirstName, data.DeletedLastName
	a.profile = data.ProfileUpdate{FirstName: data.DeletedFirstName, LastName: data.DeletedLastName}
	a.skills = map[data.SkillList]map[string]string{data.TeachingList: {}, data.LearningList: {}}
	m.settings[userID] = requestSettings{hidden: true, closed: true}
	for _, s := range m.sessions {
		if s.userID == userID {
			s.revoked = true
		}
	}
	for _, t := range m.tokens {
		if t.userID == userID {
			t.used = true
		}
	}
	for key := range m.blocks {
		if key[0] == userID || key[1] == userID {
			delete(m.blocks, key)
		}
	}
	for _, list := range m.messages {
		for _, msg := range list {
			if msg.AuthorID == userID {
				msg.Body = data.DeletedMessageBody
				msg.AuthorFirstName, msg.AuthorLastName = data.DeletedFirstName, data.DeletedLastName
			}
		}
	}
	for _, r := range m.requests {
		if r.Requester.UserID == userID {
			r.Message = ""
			r.Requester = m.party(a)
		}
		if r.Recipient.UserID == userID {
			r.Recipient = m.party(a)
		}
	}
	for _, n := range m.notifications {
		if n.ActorID == userID {
			n.Body = ""
		}
	}
	return nil
}

// catalogSkill must be called with m.mu held.
func (m *Memory) catalogSkill(id string) data.CatalogSkill {
	s := m.skills[id]
	return data.CatalogSkill{
		ID: id, CategoryID: s.skill.CategoryID, CategoryName: categoryNames[s.skill.CategoryID],
		Name: s.skill.Name, Slug: s.skill.Slug, Description: s.description, IsActive: s.active,
	}
}

func (m *Memory) CatalogSkills(_ context.Context, query string, limit, offset int) ([]data.CatalogSkill, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	query = strings.ToLower(query)
	found := []data.CatalogSkill{}
	for id, s := range m.skills {
		if strings.Contains(strings.ToLower(s.skill.Name), query) || strings.Contains(s.skill.Slug, query) {
			found = append(found, m.catalogSkill(id))
		}
	}
	slices.SortFunc(found, func(a, b data.CatalogSkill) int {
		return cmp.Or(cmp.Compare(a.CategoryID, b.CategoryID), cmp.Compare(a.Name, b.Name), cmp.Compare(a.ID, b.ID))
	})
	total := len(found)
	return append([]data.CatalogSkill{}, found[min(offset, total):min(offset+limit, total)]...), total, nil
}

func (m *Memory) CatalogSkillByID(_ context.Context, skillID string) (data.CatalogSkill, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.skills[skillID]; !ok {
		return data.CatalogSkill{}, data.ErrNotFound
	}
	return m.catalogSkill(skillID), nil
}

// checkSkill must be called with m.mu held; it mirrors the catalog constraints of PostgreSQL.
func (m *Memory) checkSkill(skillID string, in data.SkillInput) error {
	if _, ok := categoryNames[in.CategoryID]; !ok {
		return data.ErrCategoryNotFound
	}
	for id, s := range m.skills {
		if id != skillID && (s.skill.Slug == in.Slug || (s.skill.CategoryID == in.CategoryID && s.skill.Name == in.Name)) {
			return data.ErrSkillExists
		}
	}
	return nil
}

func (m *Memory) CreateSkill(_ context.Context, in data.SkillInput) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.checkSkill("", in); err != nil {
		return "", err
	}
	id := fmt.Sprintf("31000000-0000-0000-0000-%012d", len(m.skills)+1)
	m.skills[id] = catalogSkill{
		skill:       data.Skill{ID: id, CategoryID: in.CategoryID, Name: in.Name, Slug: in.Slug},
		description: in.Description, active: in.IsActive,
	}
	return id, nil
}

func (m *Memory) UpdateSkill(_ context.Context, skillID string, in data.SkillInput) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.skills[skillID]; !ok {
		return data.ErrNotFound
	}
	if err := m.checkSkill(skillID, in); err != nil {
		return err
	}
	m.skills[skillID] = catalogSkill{
		skill:       data.Skill{ID: skillID, CategoryID: in.CategoryID, Name: in.Name, Slug: in.Slug},
		description: in.Description, active: in.IsActive,
	}
	return nil
}
