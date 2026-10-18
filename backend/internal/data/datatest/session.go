package datatest

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	"skillswap/backend/internal/data"
)

var _ data.ISessionData = (*Memory)(nil)

// SetSessionStatus changes the status of a stored session, e.g. to COMPLETED for progress tests.
func (m *Memory) SetSessionStatus(sessionID, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.lessonSessions[sessionID]; ok {
		s.Status = status
		now := time.Now().UTC()
		s.UpdatedAt = now
		if status == data.SessionCompleted {
			s.CompletedAt = &now
		}
	}
}

// SetSessionTime moves a stored session, e.g. into the past so that it can be confirmed.
func (m *Memory) SetSessionTime(sessionID string, startsAt, endsAt time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.lessonSessions[sessionID]; ok {
		starts, ends := startsAt.UTC(), endsAt.UTC()
		s.StartsAt, s.EndsAt = &starts, &ends
	}
}

func (m *Memory) hydrateProposal(sp *data.SessionProposal) data.SessionProposal {
	out := *sp
	if a, ok := m.users[out.ProposedByID]; ok {
		out.ProposedByFirstName = a.profile.FirstName
		out.ProposedByLastName = a.profile.LastName
	}
	if a, ok := m.users[out.RespondedByID]; ok {
		out.RespondedByFirstName = a.profile.FirstName
		out.RespondedByLastName = a.profile.LastName
	}
	return out
}

func (m *Memory) hydrateSession(s *data.Session) data.Session {
	out := *s
	if e, ok := m.exchanges[s.ExchangeID]; ok {
		out.ExchangeStatus = e.Status
		out.ExchangeFormat = e.Format
		out.Requester = e.Requester
		out.Recipient = e.Recipient
		if out.Teacher.UserID == e.Requester.UserID {
			out.SkillID = e.RequesterTeaches.SkillID
			out.SkillCategoryID = e.RequesterTeaches.CategoryID
			out.SkillName = e.RequesterTeaches.Name
			out.PlannedDurationMinutes = e.RequesterTeaches.DurationMinutes
		} else if out.Teacher.UserID == e.Recipient.UserID {
			out.SkillID = e.RecipientTeaches.SkillID
			out.SkillCategoryID = e.RecipientTeaches.CategoryID
			out.SkillName = e.RecipientTeaches.Name
			out.PlannedDurationMinutes = e.RecipientTeaches.DurationMinutes
		}
	}
	if a, ok := m.users[out.Requester.UserID]; ok {
		out.Requester = m.party(a)
	}
	if b, ok := m.users[out.Recipient.UserID]; ok {
		out.Recipient = m.party(b)
	}
	if t, ok := m.users[out.Teacher.UserID]; ok {
		out.Teacher = m.party(t)
	}
	if l, ok := m.users[out.Learner.UserID]; ok {
		out.Learner = m.party(l)
	}
	ua, ub := m.users[out.Requester.UserID], m.users[out.Recipient.UserID]
	out.CanInteract = ua != nil && ub != nil &&
		ua.user.Status == "ACTIVE" && ub.user.Status == "ACTIVE" &&
		!m.blocked(out.Requester.UserID, out.Recipient.UserID)

	out.ConfirmedUserIDs = append([]string{}, m.sessionConfirmations[s.ID]...)
	out.CompletionConfirmedUserIDs = append([]string{}, m.completionConfirmations[s.ID]...)
	proposals := m.proposals[s.ID]
	if len(proposals) > 0 {
		var best *data.SessionProposal
		for _, p := range proposals {
			if p.Status == data.ProposalPending {
				best = p
				break
			}
		}
		if best == nil {
			best = proposals[len(proposals)-1]
		}
		hydrated := m.hydrateProposal(best)
		out.ActiveProposal = &hydrated
	}
	return out
}

func (m *Memory) ExchangeSessions(_ context.Context, exchangeID string) ([]data.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	items := []data.Session{}
	for _, s := range m.lessonSessions {
		if s.ExchangeID == exchangeID {
			items = append(items, m.hydrateSession(s))
		}
	}
	slices.SortFunc(items, func(a, b data.Session) int {
		return cmp.Or(cmp.Compare(a.PlanOrder, b.PlanOrder), cmp.Compare(a.ID, b.ID))
	})
	return items, nil
}

func (m *Memory) SessionByID(_ context.Context, sessionID string) (data.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.lessonSessions[sessionID]
	if !ok {
		return data.Session{}, data.ErrNotFound
	}
	return m.hydrateSession(s), nil
}

func (m *Memory) SessionProposals(_ context.Context, sessionID string) ([]data.SessionProposal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	list := m.proposals[sessionID]
	items := make([]data.SessionProposal, 0, len(list))
	for _, sp := range list {
		items = append(items, m.hydrateProposal(sp))
	}
	slices.SortFunc(items, func(a, b data.SessionProposal) int {
		return cmp.Or(a.CreatedAt.Compare(b.CreatedAt), cmp.Compare(a.ID, b.ID))
	})
	return items, nil
}

func (m *Memory) LockSession(ctx context.Context, sessionID string) (data.Session, error) {
	return m.SessionByID(ctx, sessionID)
}

func (m *Memory) FindDraftSessionForTeacher(_ context.Context, exchangeID, teacherID string) (data.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var best *data.Session
	for _, s := range m.lessonSessions {
		if s.ExchangeID == exchangeID && s.Teacher.UserID == teacherID && s.Status == data.SessionDraft {
			if best == nil || s.PlanOrder < best.PlanOrder || (s.PlanOrder == best.PlanOrder && s.ID < best.ID) {
				best = s
			}
		}
	}
	if best == nil {
		return data.Session{}, data.ErrNotFound
	}
	return m.hydrateSession(best), nil
}

func (m *Memory) SwapDraftSessionTeacher(_ context.Context, exchangeID, sessionID, newTeacherID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.lessonSessions[sessionID]
	if !ok || s.ExchangeID != exchangeID || s.Status != data.SessionDraft {
		return data.ErrNotFound
	}
	if s.Teacher.UserID == newTeacherID {
		return nil
	}
	var other *data.Session
	for _, candidate := range m.lessonSessions {
		if candidate.ExchangeID == exchangeID && candidate.ID != sessionID &&
			candidate.Status == data.SessionDraft && candidate.Teacher.UserID == newTeacherID {
			if other == nil || candidate.PlanOrder < other.PlanOrder ||
				(candidate.PlanOrder == other.PlanOrder && candidate.ID < other.ID) {
				other = candidate
			}
		}
	}
	if other == nil {
		return data.ErrNotFound
	}
	s.Teacher, other.Teacher = other.Teacher, s.Teacher
	s.Learner, other.Learner = other.Learner, s.Learner
	s.Title, other.Title = other.Title, s.Title
	now := time.Now().UTC()
	s.UpdatedAt, other.UpdatedAt = now, now
	return nil
}

func (m *Memory) HasScheduleConflict(_ context.Context, excludeSessionID, userA, userB string, startsAt, endsAt time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.lessonSessions {
		if s.ID == excludeSessionID || s.Status != data.SessionScheduled || s.StartsAt == nil || s.EndsAt == nil {
			continue
		}
		involvesParticipant := s.Teacher.UserID == userA || s.Teacher.UserID == userB ||
			s.Learner.UserID == userA || s.Learner.UserID == userB
		if !involvesParticipant {
			continue
		}
		if s.StartsAt.Before(endsAt) && s.EndsAt.After(startsAt) {
			return true, nil
		}
	}
	return false, nil
}

func (m *Memory) SaveSessionProposal(_ context.Context, in data.NewSessionProposal) (data.SessionProposal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.lessonSessions[in.SessionID]
	if !ok {
		return data.SessionProposal{}, data.ErrNotFound
	}
	now := time.Now().UTC()
	var totalProposals int
	for _, list := range m.proposals {
		for _, existing := range list {
			totalProposals++
			if !now.After(existing.CreatedAt) {
				now = existing.CreatedAt.Add(time.Microsecond)
			}
		}
	}
	for _, existing := range m.proposals[in.SessionID] {
		if existing.Status == data.ProposalPending {
			existing.Status = data.ProposalSuperseded
			existing.RespondedByID = in.ProposerID
			respondedAt := now
			existing.RespondedAt = &respondedAt
		}
	}
	m.sessionConfirmations[in.SessionID] = []string{in.ProposerID}
	delete(m.completionConfirmations, in.SessionID)

	proposalID := fmt.Sprintf("82000000-0000-0000-0000-%012d", totalProposals+1)
	proposal := &data.SessionProposal{
		ID:           proposalID,
		SessionID:    in.SessionID,
		Status:       data.ProposalPending,
		ProposedByID: in.ProposerID,
		StartsAt:     in.StartsAt.UTC(),
		EndsAt:       in.EndsAt.UTC(),
		Format:       in.Format,
		MeetingPlace: in.MeetingPlace,
		MeetingURL:   in.MeetingURL,
		CreatedAt:    now,
	}
	m.proposals[in.SessionID] = append(m.proposals[in.SessionID], proposal)

	s.Title = in.Title
	s.Description = in.Description
	if t, ok := m.users[in.TeacherID]; ok {
		s.Teacher = m.party(t)
	}
	if l, ok := m.users[in.LearnerID]; ok {
		s.Learner = m.party(l)
	}
	s.Status = data.SessionProposed
	starts := in.StartsAt.UTC()
	ends := in.EndsAt.UTC()
	s.StartsAt = &starts
	s.EndsAt = &ends
	s.Format = in.Format
	s.MeetingPlace = in.MeetingPlace
	s.MeetingURL = in.MeetingURL
	s.UpdatedAt = now
	if e, ok := m.exchanges[s.ExchangeID]; ok {
		e.UpdatedAt = now
	}
	return m.hydrateProposal(proposal), nil
}

func (m *Memory) AcceptSessionProposal(_ context.Context, sessionID, responderID string) (data.SessionProposal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.lessonSessions[sessionID]
	if !ok {
		return data.SessionProposal{}, data.ErrNotFound
	}
	var pending *data.SessionProposal
	for _, sp := range m.proposals[sessionID] {
		if sp.Status == data.ProposalPending {
			pending = sp
			break
		}
	}
	if pending == nil {
		return data.SessionProposal{}, data.ErrNotFound
	}
	now := time.Now().UTC()
	pending.Status = data.ProposalAccepted
	pending.RespondedByID = responderID
	pending.RespondedAt = &now
	if !slices.Contains(m.sessionConfirmations[sessionID], responderID) {
		m.sessionConfirmations[sessionID] = append(m.sessionConfirmations[sessionID], responderID)
	}
	s.Status = data.SessionScheduled
	s.UpdatedAt = now
	if e, ok := m.exchanges[s.ExchangeID]; ok {
		e.UpdatedAt = now
	}
	return m.hydrateProposal(pending), nil
}

func (m *Memory) DeclineSessionProposal(_ context.Context, sessionID, responderID string) (data.SessionProposal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.lessonSessions[sessionID]
	if !ok {
		return data.SessionProposal{}, data.ErrNotFound
	}
	var pending *data.SessionProposal
	for _, sp := range m.proposals[sessionID] {
		if sp.Status == data.ProposalPending {
			pending = sp
			break
		}
	}
	if pending == nil {
		return data.SessionProposal{}, data.ErrNotFound
	}
	now := time.Now().UTC()
	pending.Status = data.ProposalDeclined
	pending.RespondedByID = responderID
	pending.RespondedAt = &now
	delete(m.sessionConfirmations, sessionID)

	s.Status = data.SessionDraft
	s.StartsAt = nil
	s.EndsAt = nil
	if e, ok := m.exchanges[s.ExchangeID]; ok {
		s.Format = e.Format
		e.UpdatedAt = now
	}
	s.MeetingPlace = ""
	s.MeetingURL = ""
	s.UpdatedAt = now
	return m.hydrateProposal(pending), nil
}

func (m *Memory) CancelSession(_ context.Context, sessionID, cancelledByID, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.lessonSessions[sessionID]
	if !ok {
		return data.ErrNotFound
	}
	now := time.Now().UTC()
	for _, sp := range m.proposals[sessionID] {
		if sp.Status == data.ProposalPending {
			sp.Status = data.ProposalSuperseded
			sp.RespondedByID = cancelledByID
			sp.RespondedAt = &now
		}
	}
	delete(m.sessionConfirmations, sessionID)
	delete(m.completionConfirmations, sessionID)

	s.Status = data.SessionCancelled
	s.CancelledAt = &now
	s.CancellationReason = reason
	s.UpdatedAt = now

	maxOrder := 0
	nonCancelledForTeacher := 0
	for _, item := range m.lessonSessions {
		if item.ExchangeID == s.ExchangeID {
			if item.PlanOrder > maxOrder {
				maxOrder = item.PlanOrder
			}
			if item.Teacher.UserID == s.Teacher.UserID && item.Status != data.SessionCancelled && item.ID != s.ID {
				nonCancelledForTeacher++
			}
		}
	}
	exchangeFormat := s.Format
	skillName := s.SkillName
	if e, ok := m.exchanges[s.ExchangeID]; ok {
		exchangeFormat = e.Format
		e.UpdatedAt = now
		if s.Teacher.UserID == e.Requester.UserID {
			skillName = e.RequesterTeaches.Name
		} else if s.Teacher.UserID == e.Recipient.UserID {
			skillName = e.RecipientTeaches.Name
		}
	}
	if skillName == "" {
		skillName = s.Title
	}
	replacementID := fmt.Sprintf("81000000-0000-0000-0000-%012d", len(m.lessonSessions)+1)
	m.lessonSessions[replacementID] = &data.Session{
		ID:                     replacementID,
		ExchangeID:             s.ExchangeID,
		ExchangeStatus:         s.ExchangeStatus,
		ExchangeFormat:         exchangeFormat,
		PlanOrder:              maxOrder + 1,
		Title:                  fmt.Sprintf("%s — заняття %d", skillName, nonCancelledForTeacher+1),
		Status:                 data.SessionDraft,
		Format:                 exchangeFormat,
		SkillID:                s.SkillID,
		SkillCategoryID:        s.SkillCategoryID,
		SkillName:              skillName,
		PlannedDurationMinutes: s.PlannedDurationMinutes,
		Requester:              s.Requester,
		Recipient:              s.Recipient,
		Teacher:                s.Teacher,
		Learner:                s.Learner,
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	return nil
}

func (m *Memory) ConfirmSessionCompletion(_ context.Context, sessionID, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.lessonSessions[sessionID]
	if !ok {
		return data.ErrNotFound
	}
	if !slices.Contains(m.completionConfirmations[sessionID], userID) {
		m.completionConfirmations[sessionID] = append(m.completionConfirmations[sessionID], userID)
		m.touchExchange(s.ExchangeID)
	}
	return nil
}

func (m *Memory) CompleteSession(_ context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.lessonSessions[sessionID]
	if !ok || s.Status != data.SessionScheduled {
		return data.ErrNotFound
	}
	now := time.Now().UTC()
	s.Status = data.SessionCompleted
	s.CompletedAt = &now
	s.UpdatedAt = now
	m.touchExchange(s.ExchangeID)
	return nil
}

func (m *Memory) UserSchedule(_ context.Context, userID string, filter data.ScheduleFilter, limit, offset int) ([]data.Session, int, int, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.users[userID]; !ok {
		return nil, 0, 0, "", data.ErrNotFound
	}
	status := filter.Status
	if status == "" {
		status = data.SessionScheduled
	}
	now := filter.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	pendingCount := 0
	found := []data.Session{}
	for _, raw := range m.lessonSessions {
		if raw.Teacher.UserID != userID && raw.Learner.UserID != userID {
			continue
		}
		s := m.hydrateSession(raw)
		if s.ExchangeStatus == data.ExchangeActive && s.Status == data.SessionProposed &&
			s.ActiveProposal != nil && s.ActiveProposal.Status == data.ProposalPending &&
			s.ActiveProposal.ProposedByID != userID {
			pendingCount++
		}
		if status == data.ScheduleAll {
			if s.Status == data.SessionDraft {
				continue
			}
		} else if s.Status != status {
			continue
		}
		if filter.UpcomingOnly && (s.EndsAt == nil || s.EndsAt.Before(now)) {
			continue
		}
		found = append(found, s)
	}
	slices.SortFunc(found, func(a, b data.Session) int {
		switch {
		case a.StartsAt != nil && b.StartsAt != nil:
			if c := a.StartsAt.Compare(*b.StartsAt); c != 0 {
				return c
			}
		case a.StartsAt != nil:
			return -1
		case b.StartsAt != nil:
			return 1
		}
		return cmp.Or(cmp.Compare(a.PlanOrder, b.PlanOrder), cmp.Compare(a.ID, b.ID))
	})
	total := len(found)
	return append([]data.Session{}, found[min(offset, total):min(offset+limit, total)]...), total, pendingCount, "Europe/Kyiv", nil
}
