package trust

import (
	"context"
	"strings"
	"testing"

	"skillswap/backend/internal/communication"
	"skillswap/backend/internal/data"
	"skillswap/backend/internal/data/datatest"
	"skillswap/backend/internal/exchange"
	"skillswap/backend/internal/expect"
)

const unknownUserID = "40000000-0000-0000-0000-000000000999"

// pendingRequest has Andrii ask Olha for a new exchange of the same skills.
func (f *fixture) pendingRequest(t *testing.T) string {
	t.Helper()
	request, err := f.exchanges.CreateRequest(context.Background(), f.andrii, exchange.NewRequest{
		RecipientID: f.olha, TeachSkillID: datatest.PhotoshopSkillID, LearnSkillID: datatest.GuitarSkillID,
		Format: data.FormatOnline, TeachSessions: 1, TeachDurationMinutes: 60, LearnSessions: 1, LearnDurationMinutes: 60,
	})
	expect.NoError(t, "create Andrii's request", err)
	return request.ID
}

func (f *fixture) admin(t *testing.T) string {
	t.Helper()
	id := f.student(t, "admin@students.example.test", datatest.GuitarSkillID, datatest.PhotoshopSkillID)
	f.store.SetAdmin(id)
	return id
}

func (f *fixture) exchangeDetails(t *testing.T, exchangeID string) exchange.ExchangeDetails {
	t.Helper()
	details, err := f.exchanges.ExchangeDetails(context.Background(), f.olha, exchangeID)
	expect.NoError(t, "load the exchange", err)
	return details
}

func (f *fixture) latestNotification(t *testing.T, userID string) data.Notification {
	t.Helper()
	page, err := f.comm.Notifications(context.Background(), userID, false, 1)
	expect.NoError(t, "load notifications", err)
	if len(page.Items) == 0 {
		t.Fatal("no notifications")
	}
	return page.Items[0]
}

func TestBlockEndsInteractionsBetweenThePair(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	exchangeID := f.exchangeWithStatus(t, data.ExchangeActive)
	requestID := f.pendingRequest(t)

	result, err := f.service.Block(ctx, f.olha, strings.ToUpper(f.andrii))
	expect.NoError(t, "block Andrii", err)
	expect.Equal(t, "blocked user", result.Block.User.ID, f.andrii)
	expect.Equal(t, "cancelled exchanges", result.CancelledExchanges, 1)
	expect.Equal(t, "closed requests", result.ClosedRequests, 1)

	details := f.exchangeDetails(t, exchangeID)
	expect.Equal(t, "exchange status", details.Exchange.Status, data.ExchangeCancelled)
	expect.Equal(t, "cancellation reason", details.Exchange.CancellationReason, blockReason)
	expect.Equal(t, "cancelled by", details.History[len(details.History)-1].ChangedByID, f.olha)
	request, err := f.exchanges.RequestDetails(ctx, f.olha, requestID)
	expect.NoError(t, "load Andrii's request", err)
	expect.Equal(t, "Andrii's request, declined by Olha", request.Request.Status, data.RequestDeclined)

	notification := f.latestNotification(t, f.andrii)
	expect.Equal(t, "Andrii's notification", notification.Title, "Обмін скасовано")
	expect.Equal(t, "notification exchange", notification.ExchangeID, exchangeID)

	// Andrii can no longer find Olha or send her a request.
	_, err = f.exchanges.CreateRequest(ctx, f.andrii, exchange.NewRequest{
		RecipientID: f.olha, TeachSkillID: datatest.PhotoshopSkillID, LearnSkillID: datatest.GuitarSkillID,
		Format: data.FormatOnline, TeachSessions: 1, TeachDurationMinutes: 60, LearnSessions: 1, LearnDurationMinutes: 60,
	})
	expect.Error(t, "request to a student who blocked you", err, exchange.ErrStudentNotFound)

	again, err := f.service.Block(ctx, f.olha, f.andrii)
	expect.NoError(t, "block Andrii again", err)
	expect.Equal(t, "cancelled exchanges on a repeated block", again.CancelledExchanges, 0)

	blocks, err := f.service.Blocks(ctx, f.olha, 1)
	expect.NoError(t, "list Olha's blocks", err)
	expect.Equal(t, "Olha's blocks", blocks.Total, 1)
	expect.Equal(t, "blocked user in the list", blocks.Items[0].User.ID, f.andrii)

	expect.NoError(t, "unblock Andrii", f.service.Unblock(ctx, f.olha, f.andrii))
	expect.NoError(t, "unblock Andrii again", f.service.Unblock(ctx, f.olha, f.andrii))
	blocks, err = f.service.Blocks(ctx, f.olha, 1)
	expect.NoError(t, "list Olha's blocks after unblocking", err)
	expect.Equal(t, "Olha's blocks after unblocking", blocks.Total, 0)
	expect.Equal(t, "exchange after unblocking", f.exchangeDetails(t, exchangeID).Exchange.Status, data.ExchangeCancelled)
}

func TestBlockRejectsInvalidTargets(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	_, err := f.service.Block(ctx, f.olha, f.olha)
	expect.Equal(t, "blocking yourself", fieldErrors(t, err)["userId"], "You cannot block yourself")
	_, err = f.service.Block(ctx, f.olha, "not-a-uuid")
	expect.Error(t, "an invalid ID", err, ErrStudentNotFound)
	_, err = f.service.Block(ctx, f.olha, unknownUserID)
	expect.Error(t, "an unknown student", err, ErrStudentNotFound)
	_, err = f.service.Blocks(ctx, f.olha, 0)
	expect.Error(t, "page 0", err, ErrInvalidPage)
}

func TestCreateReport(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	_, err := f.service.CreateReport(ctx, f.olha, ReportInput{ReportedUserID: f.olha, Reason: "   "})
	fields := fieldErrors(t, err)
	expect.Equal(t, "reporting yourself", fields["reportedUserId"], "You cannot report yourself")
	expect.Equal(t, "blank reason", fields["reason"], reasonMessage)
	_, err = f.service.CreateReport(ctx, f.olha, ReportInput{ReportedUserID: f.andrii, Reason: strings.Repeat("a", 1001)})
	expect.Equal(t, "long reason", fieldErrors(t, err)["reason"], reasonMessage)
	_, err = f.service.CreateReport(ctx, f.olha, ReportInput{ReportedUserID: unknownUserID, Reason: "Спам"})
	expect.Error(t, "an unknown student", err, ErrStudentNotFound)

	// A blocked student can still be reported by the one who blocked them.
	_, err = f.service.Block(ctx, f.olha, f.andrii)
	expect.NoError(t, "block Andrii", err)
	report, err := f.service.CreateReport(ctx, f.olha, ReportInput{ReportedUserID: f.andrii, Reason: "  Надсилає спам  "})
	expect.NoError(t, "report Andrii", err)
	expect.Equal(t, "report status", report.Status, data.ReportOpen)
	expect.Equal(t, "trimmed reason", report.Reason, "Надсилає спам")
	expect.Equal(t, "reported user", report.ReportedUser.ID, f.andrii)
	expect.Equal(t, "report decision", report.Decision == nil, true)

	_, err = f.service.CreateReport(ctx, f.olha, ReportInput{ReportedUserID: f.andrii, Reason: "Ще раз"})
	expect.Error(t, "a second open report", err, ErrReportExists)
	// Andrii no longer sees Olha, so he cannot report her.
	_, err = f.service.CreateReport(ctx, f.andrii, ReportInput{ReportedUserID: f.olha, Reason: "Помста"})
	expect.Error(t, "reporting a student who blocked you", err, ErrStudentNotFound)

	mine, err := f.service.MyReports(ctx, f.olha, 1)
	expect.NoError(t, "list Olha's reports", err)
	expect.Equal(t, "Olha's reports", mine.Total, 1)
	expect.Equal(t, "Olha's report", mine.Items[0].ID, report.ID)
}

func TestResolveReportWithSuspension(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	admin := f.admin(t)
	exchangeID := f.exchangeWithStatus(t, data.ExchangeActive)
	report, err := f.service.CreateReport(ctx, f.olha, ReportInput{ReportedUserID: f.andrii, Reason: "Образливі повідомлення"})
	expect.NoError(t, "report Andrii", err)

	_, err = f.service.ResolveReport(ctx, admin, report.ID, DecisionInput{Decision: "BAN", Reason: ""})
	fields := fieldErrors(t, err)
	expect.Equal(t, "unknown decision", fields["decision"], "Decision must be DISMISS, RESOLVE or SUSPEND")
	expect.Equal(t, "missing reason", fields["reason"], reasonMessage)
	_, err = f.service.ResolveReport(ctx, admin, unknownUserID, DecisionInput{Decision: DecisionDismiss, Reason: "Немає порушення"})
	expect.Error(t, "an unknown report", err, ErrReportNotFound)

	open, err := f.service.Reports(ctx, data.ReportOpen, 1)
	expect.NoError(t, "list open reports", err)
	expect.Equal(t, "open reports", open.Total, 1)
	_, err = f.service.Reports(ctx, "CLOSED", 1)
	expect.Equal(t, "unknown report status", fieldErrors(t, err)["status"], "Status must be OPEN, RESOLVED or DISMISSED")

	resolved, err := f.service.ResolveReport(ctx, admin, report.ID, DecisionInput{Decision: DecisionSuspend, Reason: "Підтверджено"})
	expect.NoError(t, "suspend Andrii", err)
	expect.Equal(t, "report status", resolved.Status, data.ReportResolved)
	expect.Equal(t, "reported account status", resolved.ReportedAccountStatus, data.AccountSuspended)
	expect.Equal(t, "decision", resolved.Decision.Action, data.ActionSuspendAccount)
	expect.Equal(t, "decision author", resolved.Decision.Moderator.ID, admin)
	expect.Equal(t, "decision reason", resolved.Decision.Reason, "Підтверджено")

	details := f.exchangeDetails(t, exchangeID)
	expect.Equal(t, "exchange status", details.Exchange.Status, data.ExchangeCancelled)
	expect.Equal(t, "cancellation reason", details.Exchange.CancellationReason, suspendReason)
	expect.Equal(t, "cancelled by the system", details.History[len(details.History)-1].ChangedByID, "")
	expect.Equal(t, "Olha's notification", f.latestNotification(t, f.olha).ExchangeID, exchangeID)

	_, err = f.service.ResolveReport(ctx, admin, report.ID, DecisionInput{Decision: DecisionDismiss, Reason: "Ще раз"})
	expect.Error(t, "deciding a report twice", err, ErrReportClosed)

	reportDetails, err := f.service.ReportDetails(ctx, report.ID)
	expect.NoError(t, "load the report", err)
	expect.Equal(t, "Andrii's moderation history", len(reportDetails.History), 1)
}

func TestSuspendAndRestoreAccount(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	admin := f.admin(t)

	_, err := f.service.SuspendAccount(ctx, admin, f.andrii, "")
	expect.Equal(t, "missing reason", fieldErrors(t, err)["reason"], reasonMessage)
	_, err = f.service.SuspendAccount(ctx, admin, admin, "Тест")
	expect.Error(t, "suspending yourself", err, ErrSelfModeration)
	_, err = f.service.SuspendAccount(ctx, admin, unknownUserID, "Тест")
	expect.Error(t, "an unknown user", err, ErrUserNotFound)

	suspended, err := f.service.SuspendAccount(ctx, admin, f.andrii, "Спам у чатах")
	expect.NoError(t, "suspend Andrii", err)
	expect.Equal(t, "Andrii's status", suspended.User.Status, data.AccountSuspended)
	expect.Equal(t, "logged action", suspended.Action.Action, data.ActionSuspendAccount)
	expect.Equal(t, "logged reason", suspended.Action.Reason, "Спам у чатах")
	expect.Equal(t, "logged subject", suspended.Action.Subject.ID, f.andrii)

	again, err := f.service.SuspendAccount(ctx, admin, f.andrii, "Ще раз")
	expect.NoError(t, "suspend Andrii again", err)
	expect.Equal(t, "action for a repeated suspension", again.Action == nil, true)

	restored, err := f.service.RestoreAccount(ctx, admin, f.andrii, "Апеляцію прийнято")
	expect.NoError(t, "restore Andrii", err)
	expect.Equal(t, "Andrii's status after restoring", restored.User.Status, data.AccountActive)
	expect.Equal(t, "logged restore", restored.Action.Action, data.ActionRestoreAccount)

	log, err := f.service.ModerationLog(ctx, "", 1)
	expect.NoError(t, "load the moderation log", err)
	expect.Equal(t, "log entries", log.Total, 2)
	expect.Equal(t, "newest entry", log.Items[0].Action, data.ActionRestoreAccount)
	olhasLog, err := f.service.ModerationLog(ctx, f.olha, 1)
	expect.NoError(t, "load Olha's moderation log", err)
	expect.Equal(t, "Olha's log entries", olhasLog.Total, 0)
	_, err = f.service.ModerationLog(ctx, "nobody", 1)
	expect.Equal(t, "invalid user filter", fieldErrors(t, err)["userId"], "Choose a user")
}

func TestCatalogSkills(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	admin := f.admin(t)
	design := "20000000-0000-0000-0000-000000000002"
	name, slug := "Figma", "figma"

	_, err := f.service.CreateSkill(ctx, admin, SkillPatch{Name: new(string), Slug: new(string)})
	fields := fieldErrors(t, err)
	expect.Equal(t, "missing category", fields["categoryId"], "Choose a category from the list")
	expect.Equal(t, "missing name", fields["name"], "Name is required and must be at most 150 characters")
	badSlug := "Web Design"
	_, err = f.service.CreateSkill(ctx, admin, SkillPatch{CategoryID: &design, Name: &name, Slug: &badSlug})
	expect.Equal(t, "slug with spaces", fieldErrors(t, err)["slug"] != "", true)
	unknownCategory := "20000000-0000-0000-0000-000000000099"
	_, err = f.service.CreateSkill(ctx, admin, SkillPatch{CategoryID: &unknownCategory, Name: &name, Slug: &slug})
	expect.Equal(t, "unknown category", fieldErrors(t, err)["categoryId"], "Choose a category from the list")

	created, err := f.service.CreateSkill(ctx, admin, SkillPatch{CategoryID: &design, Name: &name, Slug: &slug})
	expect.NoError(t, "create Figma", err)
	expect.Equal(t, "new skill is active", created.IsActive, true)
	expect.Equal(t, "category name", created.CategoryName, "Дизайн")
	_, err = f.service.CreateSkill(ctx, admin, SkillPatch{CategoryID: &design, Name: &name, Slug: &slug})
	expect.Error(t, "a duplicate skill", err, ErrSkillExists)

	hidden := false
	updated, err := f.service.UpdateSkill(ctx, admin, created.ID, SkillPatch{IsActive: &hidden})
	expect.NoError(t, "hide Figma", err)
	expect.Equal(t, "hidden skill", updated.IsActive, false)
	expect.Equal(t, "name is kept", updated.Name, "Figma")
	active, err := f.store.ActiveSkillExists(ctx, created.ID)
	expect.NoError(t, "check the public catalog", err)
	expect.Equal(t, "hidden skill in the public catalog", active, false)
	_, err = f.service.UpdateSkill(ctx, admin, unknownUserID, SkillPatch{IsActive: &hidden})
	expect.Error(t, "an unknown skill", err, ErrSkillNotFound)

	catalog, err := f.service.CatalogSkills(ctx, "fig", 1)
	expect.NoError(t, "search the catalog", err)
	expect.Equal(t, "found skills", catalog.Total, 1)
	expect.Equal(t, "hidden skill is listed for administrators", catalog.Items[0].ID, created.ID)

	log, err := f.service.ModerationLog(ctx, "", 1)
	expect.NoError(t, "load the moderation log", err)
	expect.Equal(t, "catalog log entries", log.Total, 2)
	expect.Equal(t, "hiding reason", log.Items[0].Reason, "Навичку «Figma» приховано з каталогу")
	expect.Equal(t, "logged skill", log.Items[0].Skill.ID, created.ID)
}

func TestDeleteAccountAnonymizesTheUser(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	exchangeID := f.exchangeWithStatus(t, data.ExchangeActive)
	conversation, err := f.comm.ExchangeConversation(ctx, f.andrii, exchangeID)
	expect.NoError(t, "open the exchange chat", err)
	_, err = f.comm.SendMessage(ctx, f.andrii, conversation.ID, "Мій номер +380 00 000 0000")
	expect.NoError(t, "send Andrii's message", err)

	expect.NoError(t, "delete Andrii's account", f.service.DeleteAccount(ctx, f.andrii))
	expect.Equal(t, "account is deleted", f.store.IsDeleted(f.andrii), true)
	_, err = f.store.UserByID(ctx, f.andrii)
	expect.Error(t, "load the deleted user", err, data.ErrNotFound)

	details := f.exchangeDetails(t, exchangeID)
	expect.Equal(t, "exchange status", details.Exchange.Status, data.ExchangeCancelled)
	expect.Equal(t, "cancellation reason", details.Exchange.CancellationReason, deleteReason)
	expect.Equal(t, "partner name", details.Exchange.Recipient.FirstName+" "+details.Exchange.Recipient.LastName, "Видалений користувач")

	messages, err := f.comm.Messages(ctx, f.olha, conversation.ID, communication.MessageQuery{Page: 1})
	expect.NoError(t, "load the chat as Olha", err)
	expect.Equal(t, "replaced message", messages.Items[0].Body, data.DeletedMessageBody)
	expect.Equal(t, "message author", messages.Items[0].AuthorFirstName, data.DeletedFirstName)
	expect.Equal(t, "Olha's notification", f.latestNotification(t, f.olha).ExchangeID, exchangeID)

	expect.Error(t, "delete the account twice", f.service.DeleteAccount(ctx, f.andrii), ErrUserNotFound)
}
