package trust

import (
	"context"
	"errors"
	"strings"

	"skillswap/backend/internal/data"
	"skillswap/backend/internal/profile"
	"skillswap/backend/internal/validate"
)

const MaxReasonLength = 1000

type ReportInput struct {
	ReportedUserID string
	Reason         string
}

// CreateReport sends a report about a student to the administrators (FR-14). The student must be
// someone the user can see or has interacted with; one report per student stays open at a time.
func (s *Service) CreateReport(ctx context.Context, userID string, in ReportInput) (data.Report, error) {
	fields := map[string]string{}
	reportedID := strings.ToLower(in.ReportedUserID)
	switch {
	case !validate.UUID(reportedID):
		fields["reportedUserId"] = "Choose a student"
	case reportedID == userID:
		fields["reportedUserId"] = "You cannot report yourself"
	}
	reason, ok := validReason(in.Reason)
	if !ok {
		fields["reason"] = reasonMessage
	}
	if len(fields) > 0 {
		return data.Report{}, &profile.ValidationError{Fields: fields}
	}

	known, err := s.moderation.KnownStudent(ctx, userID, reportedID)
	switch {
	case err != nil:
		return data.Report{}, err
	case !known:
		return data.Report{}, ErrStudentNotFound
	}
	id, err := s.moderation.CreateReport(ctx, userID, reportedID, reason)
	switch {
	case errors.Is(err, data.ErrReportExists):
		return data.Report{}, ErrReportExists
	case err != nil:
		return data.Report{}, err
	}
	return s.moderation.ReportByID(ctx, id)
}

type ReportPage struct {
	Items []data.Report
	Page  int
	Total int
}

// MyReports lists the reports the user has sent, newest first.
func (s *Service) MyReports(ctx context.Context, userID string, page int) (ReportPage, error) {
	if page < 1 || page > MaxPage {
		return ReportPage{}, ErrInvalidPage
	}
	items, total, err := s.moderation.UserReports(ctx, userID, PageSize, (page-1)*PageSize)
	if err != nil {
		return ReportPage{}, err
	}
	return ReportPage{Items: items, Page: page, Total: total}, nil
}
