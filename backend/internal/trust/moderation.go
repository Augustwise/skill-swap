package trust

import (
	"context"
	"errors"
	"slices"
	"strings"

	"skillswap/backend/internal/data"
	"skillswap/backend/internal/profile"
	"skillswap/backend/internal/validate"
)

const (
	DecisionDismiss = "DISMISS"
	DecisionResolve = "RESOLVE"
	DecisionSuspend = "SUSPEND"

	// historyLength is the number of earlier decisions shown with a report.
	historyLength = 20
)

var reportStatuses = []string{data.ReportOpen, data.ReportResolved, data.ReportDismissed}

var (
	ErrUserNotFound   = errors.New("user was not found")
	ErrReportNotFound = errors.New("report was not found")
	ErrReportExists   = errors.New("an open report about this student already exists")
	ErrReportClosed   = errors.New("report has already been decided")
	ErrSelfModeration = errors.New("administrators cannot moderate their own account")
	ErrSkillNotFound  = errors.New("skill was not found")
	ErrSkillExists    = errors.New("a skill with this name or slug already exists")
)

// Reports lists the reports for administrators, newest first; an empty status means any status.
func (s *Service) Reports(ctx context.Context, status string, page int) (ReportPage, error) {
	if page < 1 || page > MaxPage {
		return ReportPage{}, ErrInvalidPage
	}
	if status != "" && !slices.Contains(reportStatuses, status) {
		return ReportPage{}, &profile.ValidationError{Fields: map[string]string{
			"status": "Status must be OPEN, RESOLVED or DISMISSED",
		}}
	}
	items, total, err := s.moderation.Reports(ctx, status, PageSize, (page-1)*PageSize)
	if err != nil {
		return ReportPage{}, err
	}
	return ReportPage{Items: items, Page: page, Total: total}, nil
}

type ReportDetails struct {
	Report data.Report
	// History holds the latest moderation actions about the reported user, newest first.
	History []data.ModerationAction
}

func (s *Service) ReportDetails(ctx context.Context, reportID string) (ReportDetails, error) {
	if !validate.UUID(reportID) {
		return ReportDetails{}, ErrReportNotFound
	}
	report, err := s.moderation.ReportByID(ctx, strings.ToLower(reportID))
	switch {
	case errors.Is(err, data.ErrNotFound):
		return ReportDetails{}, ErrReportNotFound
	case err != nil:
		return ReportDetails{}, err
	}
	history, _, err := s.moderation.ModerationActions(ctx, report.ReportedUser.ID, historyLength, 0)
	if err != nil {
		return ReportDetails{}, err
	}
	return ReportDetails{Report: report, History: history}, nil
}

type DecisionInput struct {
	Decision string
	Reason   string
}

var decisionResults = map[string]struct{ status, action string }{
	DecisionDismiss: {data.ReportDismissed, data.ActionDismissReport},
	DecisionResolve: {data.ReportResolved, data.ActionResolveReport},
	DecisionSuspend: {data.ReportResolved, data.ActionSuspendAccount},
}

// ResolveReport records the administrator's decision on an open report with its author, time
// and reason. SUSPEND also suspends the reported account.
func (s *Service) ResolveReport(ctx context.Context, moderatorID, reportID string, in DecisionInput) (data.Report, error) {
	if !validate.UUID(reportID) {
		return data.Report{}, ErrReportNotFound
	}
	fields := map[string]string{}
	result, known := decisionResults[in.Decision]
	if !known {
		fields["decision"] = "Decision must be DISMISS, RESOLVE or SUSPEND"
	}
	reason, ok := validReason(in.Reason)
	if !ok {
		fields["reason"] = reasonMessage
	}
	if len(fields) > 0 {
		return data.Report{}, &profile.ValidationError{Fields: fields}
	}

	var resolved data.Report
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		report, err := s.moderation.LockReport(ctx, strings.ToLower(reportID))
		switch {
		case errors.Is(err, data.ErrNotFound):
			return ErrReportNotFound
		case err != nil:
			return err
		case report.Status != data.ReportOpen:
			return ErrReportClosed
		}
		subjectID := report.ReportedUser.ID
		if in.Decision == DecisionSuspend {
			if _, err := s.suspend(ctx, moderatorID, subjectID); err != nil {
				return err
			}
		}
		if err := s.moderation.CloseReport(ctx, report.ID, result.status); err != nil {
			return err
		}
		if _, err := s.moderation.CreateModerationAction(ctx, data.NewModerationAction{
			ReportID: report.ID, ModeratorID: moderatorID, SubjectUserID: subjectID, Action: result.action, Reason: reason,
		}); err != nil {
			return err
		}
		resolved, err = s.moderation.ReportByID(ctx, report.ID)
		return err
	})
	return resolved, err
}

type AccountResult struct {
	User data.User
	// Action is nil when the account already had the requested status.
	Action *data.ModerationAction
}

// SuspendAccount blocks sign-in, revokes the sessions and cancels the active exchanges of the
// account (FR-14). Suspending a suspended account changes nothing.
func (s *Service) SuspendAccount(ctx context.Context, moderatorID, userID, rawReason string) (AccountResult, error) {
	return s.changeAccount(ctx, moderatorID, userID, rawReason, data.ActionSuspendAccount, func(ctx context.Context, userID string) (bool, error) {
		return s.suspend(ctx, moderatorID, userID)
	})
}

// RestoreAccount lets a suspended user sign in again; cancelled exchanges stay cancelled.
func (s *Service) RestoreAccount(ctx context.Context, moderatorID, userID, rawReason string) (AccountResult, error) {
	return s.changeAccount(ctx, moderatorID, userID, rawReason, data.ActionRestoreAccount, func(ctx context.Context, userID string) (bool, error) {
		user, err := s.lockAccount(ctx, moderatorID, userID)
		if err != nil || user.Status == data.AccountActive {
			return false, err
		}
		return true, s.moderation.UpdateAccountStatus(ctx, userID, data.AccountActive)
	})
}

func (s *Service) changeAccount(ctx context.Context, moderatorID, userID, rawReason, action string,
	change func(ctx context.Context, userID string) (bool, error)) (AccountResult, error) {
	if !validate.UUID(userID) {
		return AccountResult{}, ErrUserNotFound
	}
	userID = strings.ToLower(userID)
	reason, ok := validReason(rawReason)
	if !ok {
		return AccountResult{}, &profile.ValidationError{Fields: map[string]string{"reason": reasonMessage}}
	}
	var result AccountResult
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		changed, err := change(ctx, userID)
		if err != nil {
			return err
		}
		if changed {
			id, err := s.moderation.CreateModerationAction(ctx, data.NewModerationAction{
				ModeratorID: moderatorID, SubjectUserID: userID, Action: action, Reason: reason,
			})
			if err != nil {
				return err
			}
			logged, err := s.moderation.ModerationActionByID(ctx, id)
			if err != nil {
				return err
			}
			result.Action = &logged
		}
		result.User, err = s.moderation.UserByID(ctx, userID)
		return err
	})
	return result, err
}

// suspend must run inside a transaction; it reports false when the account was already suspended.
func (s *Service) suspend(ctx context.Context, moderatorID, userID string) (bool, error) {
	user, err := s.lockAccount(ctx, moderatorID, userID)
	if err != nil || user.Status == data.AccountSuspended {
		return false, err
	}
	if err := s.moderation.UpdateAccountStatus(ctx, userID, data.AccountSuspended); err != nil {
		return false, err
	}
	if err := s.moderation.RevokeUserSessions(ctx, userID); err != nil {
		return false, err
	}
	if _, err := s.endInteractions(ctx, ending{userID: userID, reason: suspendReason}); err != nil {
		return false, err
	}
	return true, nil
}

// lockAccount holds the user row, as every operation that changes who may interact does.
func (s *Service) lockAccount(ctx context.Context, moderatorID, userID string) (data.User, error) {
	if userID == moderatorID {
		return data.User{}, ErrSelfModeration
	}
	// An unknown user has no row to lock; UserByID reports it below.
	if err := s.moderation.LockUsers(ctx, userID, userID); err != nil && !errors.Is(err, data.ErrNotFound) {
		return data.User{}, err
	}
	user, err := s.moderation.UserByID(ctx, userID)
	if errors.Is(err, data.ErrNotFound) {
		return data.User{}, ErrUserNotFound
	}
	return user, err
}

type ActionPage struct {
	Items []data.ModerationAction
	Page  int
	Total int
}

// ModerationLog lists the administrator actions, newest first; userID limits it to one account.
func (s *Service) ModerationLog(ctx context.Context, userID string, page int) (ActionPage, error) {
	if page < 1 || page > MaxPage {
		return ActionPage{}, ErrInvalidPage
	}
	if userID != "" && !validate.UUID(userID) {
		return ActionPage{}, &profile.ValidationError{Fields: map[string]string{"userId": "Choose a user"}}
	}
	items, total, err := s.moderation.ModerationActions(ctx, strings.ToLower(userID), PageSize, (page-1)*PageSize)
	if err != nil {
		return ActionPage{}, err
	}
	return ActionPage{Items: items, Page: page, Total: total}, nil
}

const reasonMessage = "Give a reason of 1 to 1000 characters"

// validReason checks a required reason of a report or a moderation decision.
func validReason(raw string) (string, bool) {
	reason, ok := validate.Text(raw, MaxReasonLength)
	return reason, ok && reason != ""
}
