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

type requestSettings struct {
	closed             bool
	sameUniversityOnly bool
}

// SetRequestSettings changes who may send requests to the user.
func (m *Memory) SetRequestSettings(userID string, acceptsRequests, sameUniversityOnly bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.settings[userID] = requestSettings{closed: !acceptsRequests, sameUniversityOnly: sameUniversityOnly}
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
	shown := ok && visible(senderID, recipientID, c)
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
