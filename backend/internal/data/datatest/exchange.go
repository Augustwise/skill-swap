package datatest

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	"skillswap/backend/internal/data"
)

var _ data.IExchangeData = (*Memory)(nil)

// requestSettings keep the zero value for the defaults: visible and open to anyone.
type requestSettings struct {
	hidden             bool
	closed             bool
	sameUniversityOnly bool
}

// SetRequestSettings changes who may send requests to the user.
func (m *Memory) SetRequestSettings(userID string, acceptsRequests, sameUniversityOnly bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.settings[userID]
	s.closed, s.sameUniversityOnly = !acceptsRequests, sameUniversityOnly
	m.settings[userID] = s
}

// SetRequestStatus changes the status of a stored request, e.g. to free its skill pair.
func (m *Memory) SetRequestStatus(requestID, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.requests[requestID]; ok {
		r.Status = status
	}
}

func (m *Memory) RequestRecipient(ctx context.Context, senderID, recipientID string) (data.RequestRecipient, error) {
	m.mu.Lock()
	c, ok := m.users[recipientID]
	shown := ok && m.visible(senderID, recipientID, c)
	settings := m.settings[recipientID]
	m.mu.Unlock()
	if !shown {
		return data.RequestRecipient{}, data.ErrNotFound
	}
	profile, err := m.ProfileByUserID(ctx, recipientID)
	if err != nil {
		return data.RequestRecipient{}, err
	}
	return data.RequestRecipient{
		Profile: profile, AcceptsRequests: !settings.closed, SameUniversityOnly: settings.sameUniversityOnly,
	}, nil
}

func (m *Memory) CreateRequest(_ context.Context, r data.NewRequest) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	requester, recipient := m.users[r.RequesterID], m.users[r.RecipientID]
	if requester == nil || recipient == nil {
		return "", data.ErrNotFound
	}
	teacherLevel, ok1 := requester.skills[data.TeachingList][r.TeachSkillID]
	learnerLevel, ok2 := recipient.skills[data.LearningList][r.TeachSkillID]
	recipientLevel, ok3 := recipient.skills[data.TeachingList][r.LearnSkillID]
	requesterLevel, ok4 := requester.skills[data.LearningList][r.LearnSkillID]
	if !ok1 || !ok2 || !ok3 || !ok4 {
		return "", data.ErrNotFound
	}
	pair := pendingPair(r.RequesterID, r.TeachSkillID, r.RecipientID, r.LearnSkillID)
	for _, existing := range m.requests {
		if existing.Status == data.RequestPending && pendingPair(existing.Requester.UserID, existing.RequesterTeaches.SkillID,
			existing.Recipient.UserID, existing.RecipientTeaches.SkillID) == pair {
			return "", data.ErrDuplicateRequest
		}
	}
	id := fmt.Sprintf("70000000-0000-0000-0000-%012d", len(m.requests)+1)
	now := time.Now().UTC()
	m.requests[id] = &data.ExchangeRequest{
		ID: id, Status: data.RequestPending, Format: r.Format, TotalSessions: r.TeachSessions + r.LearnSessions,
		Message:   r.Message,
		Requester: m.party(requester), Recipient: m.party(recipient),
		RequesterTeaches: m.terms(r.TeachSkillID, teacherLevel, learnerLevel, r.TeachSessions, r.TeachDurationMinutes),
		RecipientTeaches: m.terms(r.LearnSkillID, recipientLevel, requesterLevel, r.LearnSessions, r.LearnDurationMinutes),
		CreatedAt:        now,
	}
	m.history[id] = []data.RequestStatusChange{{Status: data.RequestPending, ChangedByID: requester.user.ID,
		ChangedByFirstName: requester.profile.FirstName, ChangedByLastName: requester.profile.LastName, CreatedAt: now}}
	return id, nil
}

// pendingPair names both teaching rows in a fixed order, as the PostgreSQL index does.
func pendingPair(requesterID, teachSkillID, recipientID, learnSkillID string) [2]string {
	a, b := requesterID+"/"+teachSkillID, recipientID+"/"+learnSkillID
	return [2]string{min(a, b), max(a, b)}
}

func (m *Memory) party(a *account) data.RequestParty {
	return data.RequestParty{
		UserID: a.user.ID, FirstName: a.profile.FirstName, LastName: a.profile.LastName,
		UniversityID: a.user.UniversityID, UniversityName: "Демонстраційний університет", City: a.profile.City,
	}
}

func (m *Memory) terms(skillID, teacherLevel, learnerLevel string, sessions, duration int) data.RequestTerms {
	skill := m.skills[skillID].skill
	return data.RequestTerms{
		SkillID: skillID, CategoryID: skill.CategoryID, Name: skill.Name, TeacherLevel: teacherLevel,
		LearnerLevel: learnerLevel, Sessions: sessions, DurationMinutes: duration,
	}
}

func (m *Memory) RequestByID(_ context.Context, requestID string) (data.ExchangeRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.requests[requestID]
	if !ok {
		return data.ExchangeRequest{}, data.ErrNotFound
	}
	return *r, nil
}

func (m *Memory) UserRequests(_ context.Context, userID string, filter data.RequestFilter, limit, offset int) ([]data.ExchangeRequest, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var found []data.ExchangeRequest
	for _, r := range m.requests {
		owner := r.Requester.UserID
		if filter.Direction == data.Incoming {
			owner = r.Recipient.UserID
		}
		if owner == userID && (filter.Status == "" || r.Status == filter.Status) {
			found = append(found, *r)
		}
	}
	slices.SortFunc(found, func(a, b data.ExchangeRequest) int {
		return cmp.Or(b.CreatedAt.Compare(a.CreatedAt), cmp.Compare(b.ID, a.ID))
	})
	total := len(found)
	found = found[min(offset, total):min(offset+limit, total)]
	return found, total, nil
}

func (m *Memory) RequestHistory(_ context.Context, requestID string) ([]data.RequestStatusChange, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.history[requestID]), nil
}

func (m *Memory) LockRequest(_ context.Context, requestID string) (data.RequestState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.requests[requestID]
	if !ok {
		return data.RequestState{}, data.ErrNotFound
	}
	return data.RequestState{ID: r.ID, RequesterID: r.Requester.UserID, RecipientID: r.Recipient.UserID, Status: r.Status}, nil
}

func (m *Memory) UpdateRequestStatus(_ context.Context, requestID, status, changedByID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.requests[requestID]
	if !ok {
		return data.ErrNotFound
	}
	now := time.Now().UTC()
	r.Status, r.RespondedAt = status, &now
	change := data.RequestStatusChange{Status: status, ChangedByID: changedByID, CreatedAt: now}
	if a, ok := m.users[changedByID]; ok {
		change.ChangedByFirstName, change.ChangedByLastName = a.profile.FirstName, a.profile.LastName
	}
	m.history[requestID] = append(m.history[requestID], change)
	return nil
}

func (m *Memory) RequestIsCurrent(_ context.Context, requestID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.requests[requestID]
	if !ok {
		return false, nil
	}
	requester, recipient := m.users[r.Requester.UserID], m.users[r.Recipient.UserID]
	if !requester.active() || !recipient.active() || m.blocked(r.Requester.UserID, r.Recipient.UserID) {
		return false, nil
	}
	_, ok1 := requester.skills[data.TeachingList][r.RequesterTeaches.SkillID]
	_, ok2 := recipient.skills[data.LearningList][r.RequesterTeaches.SkillID]
	_, ok3 := recipient.skills[data.TeachingList][r.RecipientTeaches.SkillID]
	_, ok4 := requester.skills[data.LearningList][r.RecipientTeaches.SkillID]
	return ok1 && ok2 && ok3 && ok4, nil
}

// SetExchangeStatus changes the status of a stored exchange, e.g. to COMPLETED or CANCELLED.
func (m *Memory) SetExchangeStatus(exchangeID, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.exchanges[exchangeID]; ok {
		e.Status = status
		now := time.Now().UTC()
		e.UpdatedAt = now
		switch status {
		case data.ExchangeCompleted:
			e.CompletedAt = &now
		case data.ExchangeCancelled:
			e.CancelledAt = &now
		}
	}
}

func (m *Memory) CreateExchange(_ context.Context, requestID, _ string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.requests[requestID]
	if !ok {
		return "", data.ErrNotFound
	}
	if r.ExchangeID != "" {
		return "", fmt.Errorf("request %s already has an exchange", requestID)
	}
	id := fmt.Sprintf("80000000-0000-0000-0000-%012d", len(m.exchanges)+1)
	now := time.Now().UTC()
	for _, existing := range m.exchanges {
		if !now.After(existing.UpdatedAt) {
			now = existing.UpdatedAt.Add(time.Microsecond)
		}
	}
	m.exchanges[id] = &data.Exchange{
		ID: id, RequestID: r.ID, Status: data.ExchangeActive, Format: r.Format, TotalSessions: r.TotalSessions,
		Requester: r.Requester, Recipient: r.Recipient,
		RequesterTeaches: r.RequesterTeaches, RecipientTeaches: r.RecipientTeaches,
		StartedAt: &now, CreatedAt: now, UpdatedAt: now,
	}
	r.ExchangeID = id
	m.exchangeHistory[id] = []data.ExchangeStatusChange{m.statusChange(data.ExchangeActive, r.Recipient.UserID, "", now)}

	order := 1
	addDrafts := func(teacher, learner data.RequestParty, terms data.RequestTerms) {
		for i := 1; i <= terms.Sessions; i++ {
			sessionID := fmt.Sprintf("81000000-0000-0000-0000-%012d", len(m.lessonSessions)+1)
			m.lessonSessions[sessionID] = &data.Session{
				ID:                     sessionID,
				ExchangeID:             id,
				ExchangeStatus:         data.ExchangeActive,
				ExchangeFormat:         r.Format,
				PlanOrder:              order,
				Title:                  fmt.Sprintf("%s — заняття %d", terms.Name, i),
				Status:                 data.SessionDraft,
				Format:                 r.Format,
				SkillID:                terms.SkillID,
				SkillCategoryID:        terms.CategoryID,
				SkillName:              terms.Name,
				PlannedDurationMinutes: terms.DurationMinutes,
				Requester:              r.Requester,
				Recipient:              r.Recipient,
				Teacher:                teacher,
				Learner:                learner,
				CreatedAt:              now,
				UpdatedAt:              now,
			}
			order++
		}
	}
	addDrafts(r.Requester, r.Recipient, r.RequesterTeaches)
	addDrafts(r.Recipient, r.Requester, r.RecipientTeaches)
	return id, nil
}

func (m *Memory) hydrateExchange(e *data.Exchange) data.Exchange {
	out := *e
	if a, ok := m.users[out.Requester.UserID]; ok {
		out.Requester = m.party(a)
	}
	if b, ok := m.users[out.Recipient.UserID]; ok {
		out.Recipient = m.party(b)
	}
	ua, ub := m.users[out.Requester.UserID], m.users[out.Recipient.UserID]
	out.CanInteract = ua.active() && ub.active() && !m.blocked(out.Requester.UserID, out.Recipient.UserID)
	out.ConfirmedUserIDs = append([]string{}, m.exchangeConfirmations[out.ID]...)
	out.ReviewAuthorIDs = []string{}
	for _, r := range m.exchangeReviews(out.ID) {
		out.ReviewAuthorIDs = append(out.ReviewAuthorIDs, r.AuthorID)
	}
	progress := data.ExchangeProgress{
		TotalSessions:            out.TotalSessions,
		RequesterPlannedSessions: out.RequesterTeaches.Sessions,
		RecipientPlannedSessions: out.RecipientTeaches.Sessions,
	}
	now := time.Now().UTC()
	var next *data.NextSessionSummary
	for _, s := range m.lessonSessions {
		if s.ExchangeID != out.ID {
			continue
		}
		switch s.Status {
		case data.SessionCompleted:
			progress.CompletedSessions++
			if s.Teacher.UserID == out.Requester.UserID {
				progress.RequesterCompletedSessions++
			} else if s.Teacher.UserID == out.Recipient.UserID {
				progress.RecipientCompletedSessions++
			}
		case data.SessionScheduled:
			progress.ScheduledSessions++
			if s.EndsAt != nil && !s.EndsAt.After(now) {
				progress.AwaitingConfirmationSessions++
			}
			if s.StartsAt != nil && s.EndsAt != nil && !s.EndsAt.Before(now) {
				candidate := &data.NextSessionSummary{
					ID:           s.ID,
					PlanOrder:    s.PlanOrder,
					Title:        s.Title,
					Status:       s.Status,
					Format:       s.Format,
					StartsAt:     *s.StartsAt,
					EndsAt:       *s.EndsAt,
					Teacher:      s.Teacher,
					Learner:      s.Learner,
					MeetingPlace: s.MeetingPlace,
					MeetingURL:   s.MeetingURL,
				}
				if next == nil || candidate.StartsAt.Before(next.StartsAt) ||
					(candidate.StartsAt.Equal(next.StartsAt) && (candidate.PlanOrder < next.PlanOrder ||
						(candidate.PlanOrder == next.PlanOrder && candidate.ID < next.ID))) {
					next = candidate
				}
			}
		case data.SessionProposed:
			progress.ProposedSessions++
		case data.SessionDraft:
			progress.DraftSessions++
		case data.SessionCancelled:
			progress.CancelledSessions++
		}
	}
	if progress.TotalSessions > 0 {
		progress.Percent = min(100, (progress.CompletedSessions*100)/progress.TotalSessions)
	}
	out.Progress = progress
	out.NextSession = next
	return out
}

func (m *Memory) ExchangeByID(_ context.Context, exchangeID string) (data.Exchange, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.exchanges[exchangeID]
	if !ok {
		return data.Exchange{}, data.ErrNotFound
	}
	return m.hydrateExchange(e), nil
}

func (m *Memory) UserExchanges(_ context.Context, userID string, filter data.ExchangeFilter, limit, offset int) ([]data.Exchange, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	found := []data.Exchange{}
	for _, e := range m.exchanges {
		if (e.Requester.UserID == userID || e.Recipient.UserID == userID) &&
			(filter.Status == "" || e.Status == filter.Status) {
			found = append(found, m.hydrateExchange(e))
		}
	}
	slices.SortFunc(found, func(a, b data.Exchange) int {
		activeOrder := func(s string) int {
			if s == data.ExchangeActive {
				return 0
			}
			return 1
		}
		return cmp.Or(
			cmp.Compare(activeOrder(a.Status), activeOrder(b.Status)),
			b.UpdatedAt.Compare(a.UpdatedAt),
			b.CreatedAt.Compare(a.CreatedAt),
			cmp.Compare(b.ID, a.ID),
		)
	})
	total := len(found)
	return append([]data.Exchange{}, found[min(offset, total):min(offset+limit, total)]...), total, nil
}

// touchExchange must be called with m.mu held.
func (m *Memory) touchExchange(exchangeID string) {
	if e, ok := m.exchanges[exchangeID]; ok {
		e.UpdatedAt = time.Now().UTC()
	}
}

func (m *Memory) statusChange(status, userID, reason string, at time.Time) data.ExchangeStatusChange {
	change := data.ExchangeStatusChange{Status: status, ChangedByID: userID, Reason: reason, CreatedAt: at}
	if a, ok := m.users[userID]; ok {
		change.ChangedByFirstName, change.ChangedByLastName = a.profile.FirstName, a.profile.LastName
	}
	return change
}

func (m *Memory) ExchangeHistory(_ context.Context, exchangeID string) ([]data.ExchangeStatusChange, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.exchangeHistory[exchangeID]), nil
}

func (m *Memory) LockExchange(ctx context.Context, exchangeID string) (data.Exchange, error) {
	return m.ExchangeByID(ctx, exchangeID)
}

func (m *Memory) ConfirmExchangeCompletion(_ context.Context, exchangeID, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.exchanges[exchangeID]; !ok {
		return data.ErrNotFound
	}
	if !slices.Contains(m.exchangeConfirmations[exchangeID], userID) {
		m.exchangeConfirmations[exchangeID] = append(m.exchangeConfirmations[exchangeID], userID)
		m.touchExchange(exchangeID)
	}
	return nil
}

func (m *Memory) CompleteExchange(_ context.Context, exchangeID, completedByID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.exchanges[exchangeID]
	if !ok || e.Status != data.ExchangeActive {
		return data.ErrNotFound
	}
	now := time.Now().UTC()
	e.Status = data.ExchangeCompleted
	e.CompletedAt = &now
	e.UpdatedAt = now
	m.exchangeHistory[exchangeID] = append(m.exchangeHistory[exchangeID], m.statusChange(data.ExchangeCompleted, completedByID, "", now))
	return nil
}

func (m *Memory) CancelExchange(_ context.Context, exchangeID, cancelledByID, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.exchanges[exchangeID]
	if !ok || e.Status != data.ExchangeActive {
		return data.ErrNotFound
	}
	now := time.Now().UTC()
	e.Status = data.ExchangeCancelled
	e.CancelledAt = &now
	e.CancellationReason = reason
	e.UpdatedAt = now
	m.exchangeHistory[exchangeID] = append(m.exchangeHistory[exchangeID], m.statusChange(data.ExchangeCancelled, cancelledByID, reason, now))
	for _, s := range m.lessonSessions {
		if s.ExchangeID != exchangeID || s.Status == data.SessionCompleted || s.Status == data.SessionCancelled {
			continue
		}
		for _, sp := range m.proposals[s.ID] {
			if sp.Status == data.ProposalPending {
				sp.Status = data.ProposalSuperseded
				sp.RespondedByID = cancelledByID
				sp.RespondedAt = &now
			}
		}
		delete(m.sessionConfirmations, s.ID)
		delete(m.completionConfirmations, s.ID)
		s.Status = data.SessionCancelled
		s.CancelledAt = &now
		s.CancellationReason = reason
		s.UpdatedAt = now
	}
	return nil
}
