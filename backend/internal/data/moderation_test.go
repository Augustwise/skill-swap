package data

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"

	"skillswap/backend/internal/expect"
)

const (
	demoAdmin         = "60000000-0000-0000-0000-000000000012"
	demoIrynaDmytro   = "80000000-0000-0000-0000-000000000002"
	demoOpenReport    = "a0000000-0000-0000-0000-000000000001"
	demoDecidedReport = "a0000000-0000-0000-0000-000000000002"
)

// inRollback runs fn in a transaction that is rolled back. A failed statement aborts the
// transaction, so a check that expects a database error must come last.
func inRollback(t *testing.T, fn func(ctx context.Context, store *Postgres)) {
	t.Helper()
	store := openTestStore(t)
	rollback := errors.New("roll back test data")
	err := store.WithinTx(context.Background(), func(ctx context.Context) error {
		fn(ctx, store)
		return rollback
	})
	expect.Error(t, "WithinTx", err, rollback)
}

// Skipped unless TEST_DATABASE_URL points to a migrated, demo-seeded database.
func TestBlocksAgainstPostgres(t *testing.T) {
	inRollback(t, func(ctx context.Context, store *Postgres) {
		for _, c := range []struct {
			name      string
			viewer    string
			student   string
			wantKnown bool
		}{
			{"a visible student", demoOlha, demoAndrii, true},
			{"a student Olha blocked", demoOlha, demoOleh, true},
			{"a hidden student without a shared request", demoOlha, demoNataliia, false},
			{"an unverified student", demoOlha, demoViktor, false},
			{"oneself", demoOlha, demoOlha, false},
		} {
			known, err := store.KnownStudent(ctx, c.viewer, c.student)
			expect.NoError(t, c.name, err)
			expect.Equal(t, c.name, known, c.wantKnown)
		}

		ids, err := store.ActiveExchangeIDs(ctx, demoIryna, "")
		expect.NoError(t, "Iryna's active exchanges", err)
		expect.Slice(t, "Iryna's active exchanges", ids, []string{demoIrynaMarkoExchange, demoIrynaDmytro})
		ids, err = store.ActiveExchangeIDs(ctx, demoMarko, demoIryna)
		expect.NoError(t, "Marko's exchanges with Iryna", err)
		expect.Slice(t, "Marko's exchanges with Iryna", ids, []string{demoIrynaMarkoExchange})
		requests, err := store.PendingRequests(ctx, demoOlha, demoTaras)
		expect.NoError(t, "Olha's requests from Taras", err)
		expect.Equal(t, "Olha's requests from Taras", len(requests), 1)
		expect.Equal(t, "requester", requests[0].RequesterID, demoTaras)
		requests, err = store.PendingRequests(ctx, demoOlha, demoAndrii)
		expect.NoError(t, "Olha's requests with Andrii", err)
		expect.Equal(t, "Olha's requests with Andrii", len(requests), 0)

		created, err := store.CreateBlock(ctx, demoIryna, demoMarko)
		expect.NoError(t, "block Marko", err)
		expect.Equal(t, "block is new", created, true)
		created, err = store.CreateBlock(ctx, demoIryna, demoMarko)
		expect.NoError(t, "block Marko again", err)
		expect.Equal(t, "repeated block is new", created, false)
		block, err := store.BlockByUsers(ctx, demoIryna, demoMarko)
		expect.NoError(t, "load the block", err)
		expect.Equal(t, "blocked user's name", block.User.FirstName, "Марко")
		blocks, total, err := store.UserBlocks(ctx, demoIryna, 20, 0)
		expect.NoError(t, "Iryna's blocks", err)
		expect.Equal(t, "Iryna's block count", total, 1)
		expect.Equal(t, "Iryna's blocked user", blocks[0].User.ID, demoMarko)

		convID, err := store.EnsureExchangeConversation(ctx, demoIrynaMarkoExchange)
		expect.NoError(t, "open the exchange chat", err)
		conv, err := store.ConversationByID(ctx, demoIryna, convID)
		expect.NoError(t, "load the chat as Iryna", err)
		expect.Equal(t, "Iryna blocked the partner", conv.BlockedByMe, true)
		expect.Equal(t, "Iryna can send", conv.CanSend, false)
		conv, err = store.ConversationByID(ctx, demoMarko, convID)
		expect.NoError(t, "load the chat as Marko", err)
		expect.Equal(t, "Marko blocked the partner", conv.BlockedByMe, false)

		// Changes made by the system have no author.
		expect.NoError(t, "cancel the exchange", store.CancelExchange(ctx, demoIrynaMarkoExchange, "", "Причина"))
		history, err := store.ExchangeHistory(ctx, demoIrynaMarkoExchange)
		expect.NoError(t, "exchange history", err)
		expect.Equal(t, "cancelled by", history[len(history)-1].ChangedByID, "")
		expect.NoError(t, "decline Taras's request", store.UpdateRequestStatus(ctx, demoTarasToOlha, RequestDeclined, ""))
		requestHistory, err := store.RequestHistory(ctx, demoTarasToOlha)
		expect.NoError(t, "request history", err)
		expect.Equal(t, "declined by", requestHistory[len(requestHistory)-1].ChangedByID, "")

		expect.NoError(t, "unblock Marko", store.DeleteBlock(ctx, demoIryna, demoMarko))
		_, err = store.BlockByUsers(ctx, demoIryna, demoMarko)
		expect.Error(t, "load the removed block", err, ErrNotFound)
	})
}

func TestReportsAgainstPostgres(t *testing.T) {
	inRollback(t, func(ctx context.Context, store *Postgres) {
		decided, err := store.ReportByID(ctx, demoDecidedReport)
		expect.NoError(t, "load the decided report", err)
		expect.Equal(t, "decided report status", decided.Status, ReportDismissed)
		expect.Equal(t, "decision", decided.Decision.Action, ActionDismissReport)
		expect.Equal(t, "decision author", decided.Decision.Moderator.ID, demoAdmin)
		expect.Equal(t, "decision subject", decided.Decision.Subject.ID, demoMarko)

		open, _, err := store.Reports(ctx, ReportOpen, 20, 0)
		expect.NoError(t, "list open reports", err)
		expect.Equal(t, "seeded open report", slices.ContainsFunc(open, func(r Report) bool { return r.ID == demoOpenReport }), true)

		id, err := store.CreateReport(ctx, demoAndrii, demoOlha, "Не відповідає на повідомлення")
		expect.NoError(t, "report Olha", err)
		report, err := store.LockReport(ctx, id)
		expect.NoError(t, "lock the report", err)
		expect.Equal(t, "reporter", report.Reporter.FirstName, "Андрій")
		expect.Equal(t, "reported account", report.ReportedAccountStatus, AccountActive)
		expect.Equal(t, "open report decision", report.Decision == nil, true)
		mine, total, err := store.UserReports(ctx, demoAndrii, 20, 0)
		expect.NoError(t, "Andrii's reports", err)
		expect.Equal(t, "Andrii's newest report", mine[0].ID, id)
		expect.Equal(t, "Andrii's report count", total >= 2, true)

		expect.NoError(t, "resolve the report", store.CloseReport(ctx, id, ReportResolved))
		expect.Error(t, "resolve the report again", store.CloseReport(ctx, id, ReportDismissed), ErrNotFound)
		actionID, err := store.CreateModerationAction(ctx, NewModerationAction{
			ReportID: id, ModeratorID: demoAdmin, SubjectUserID: demoOlha, Action: ActionSuspendAccount, Reason: "Перевірено",
		})
		expect.NoError(t, "log the decision", err)
		action, err := store.ModerationActionByID(ctx, actionID)
		expect.NoError(t, "load the decision", err)
		expect.Equal(t, "logged report", action.ReportID, id)
		expect.Equal(t, "logged skill", action.Skill == nil, true)
		report, err = store.ReportByID(ctx, id)
		expect.NoError(t, "load the resolved report", err)
		expect.Equal(t, "resolved at", report.ResolvedAt != nil, true)
		expect.Equal(t, "decision", report.Decision.ID, actionID)
		actions, total, err := store.ModerationActions(ctx, demoOlha, 20, 0)
		expect.NoError(t, "Olha's moderation log", err)
		expect.Equal(t, "Olha's log entries", total, 1)
		expect.Equal(t, "Olha's newest entry", actions[0].ID, actionID)

		expect.NoError(t, "suspend Olha", store.UpdateAccountStatus(ctx, demoOlha, AccountSuspended))
		user, err := store.UserByID(ctx, demoOlha)
		expect.NoError(t, "load Olha", err)
		expect.Equal(t, "Olha's status", user.Status, AccountSuspended)

		_, err = store.CreateReport(ctx, demoOlha, demoOleh, "Ще одна скарга")
		expect.Error(t, "a second open report about Oleh", err, ErrReportExists)
	})
}

func TestCatalogAgainstPostgres(t *testing.T) {
	const design = "20000000-0000-0000-0000-000000000002"
	inRollback(t, func(ctx context.Context, store *Postgres) {
		id, err := store.CreateSkill(ctx, SkillInput{CategoryID: design, Name: "Canva", Slug: "canva", IsActive: true})
		expect.NoError(t, "create Canva", err)
		skill, err := store.CatalogSkillByID(ctx, id)
		expect.NoError(t, "load Canva", err)
		expect.Equal(t, "category name", skill.CategoryName, "Дизайн")
		expect.Equal(t, "no description", skill.Description, "")

		expect.NoError(t, "hide Canva", store.UpdateSkill(ctx, id, SkillInput{
			CategoryID: design, Name: "Canva", Slug: "canva", Description: "Онлайн-редактор", IsActive: false,
		}))
		public, err := store.Skills(ctx, "canva", 10)
		expect.NoError(t, "search the public catalog", err)
		expect.Equal(t, "hidden skill in the public catalog", len(public), 0)
		all, total, err := store.CatalogSkills(ctx, "canva", 50, 0)
		expect.NoError(t, "search the administrator catalog", err)
		expect.Equal(t, "hidden skill in the administrator catalog", total, 1)
		expect.Equal(t, "description", all[0].Description, "Онлайн-редактор")

		actionID, err := store.CreateModerationAction(ctx, NewModerationAction{
			ModeratorID: demoAdmin, SkillID: id, Action: ActionUpdateSkill, Reason: "Навичку «Canva» приховано з каталогу",
		})
		expect.NoError(t, "log the catalog change", err)
		action, err := store.ModerationActionByID(ctx, actionID)
		expect.NoError(t, "load the catalog change", err)
		expect.Equal(t, "logged skill", action.Skill.Name, "Canva")
		expect.Equal(t, "logged subject", action.Subject == nil, true)
		expect.Error(t, "update an unknown skill", store.UpdateSkill(ctx, demoAdmin, SkillInput{
			CategoryID: design, Name: "X", Slug: "x",
		}), ErrNotFound)

		_, err = store.CreateSkill(ctx, SkillInput{CategoryID: design, Name: "Ще один Photoshop", Slug: "photoshop", IsActive: true})
		expect.Error(t, "a duplicate slug", err, ErrSkillExists)
	})
	inRollback(t, func(ctx context.Context, store *Postgres) {
		_, err := store.CreateSkill(ctx, SkillInput{CategoryID: demoAdmin, Name: "Canva", Slug: "canva", IsActive: true})
		expect.Error(t, "an unknown category", err, ErrCategoryNotFound)
	})
}

func TestPrivacyAndAnonymizationAgainstPostgres(t *testing.T) {
	inRollback(t, func(ctx context.Context, store *Postgres) {
		settings, err := store.PrivacySettings(ctx, demoNataliia)
		expect.NoError(t, "Nataliia's settings", err)
		expect.Equal(t, "Nataliia is hidden", settings.Discoverable, false)
		settings, err = store.PrivacySettings(ctx, demoOlha)
		expect.NoError(t, "Olha's settings", err)
		expect.Equal(t, "Olha's default scope", settings.RequestScope, ScopeAnyone)
		expect.NoError(t, "hide Olha", store.UpdatePrivacySettings(ctx, demoOlha, PrivacySettings{
			Discoverable: false, AcceptsRequests: true, RequestScope: ScopeSameUniversity,
		}))
		settings, err = store.PrivacySettings(ctx, demoOlha)
		expect.NoError(t, "Olha's new settings", err)
		expect.Equal(t, "Olha's scope", settings.RequestScope, ScopeSameUniversity)
		visible, err := store.StudentVisible(ctx, demoAndrii, demoOlha)
		expect.NoError(t, "Olha's visibility", err)
		expect.Equal(t, "hidden Olha is visible", visible, false)

		expect.NoError(t, "delete Taras's account", store.AnonymizeUser(ctx, demoTaras))
		_, err = store.UserByID(ctx, demoTaras)
		expect.Error(t, "load the deleted user", err, ErrNotFound)
		_, err = store.ProfileByUserID(ctx, demoTaras)
		expect.Error(t, "load the deleted profile", err, ErrNotFound)
		var email, city string
		var bio *string
		expect.NoError(t, "read the users row", store.conn(ctx).QueryRow(ctx,
			`SELECT email, COALESCE(city, ''), bio FROM users WHERE id = $1`, demoTaras).Scan(&email, &city, &bio))
		expect.Equal(t, "email", email, "deleted-"+demoTaras+"@deleted.invalid")
		expect.Equal(t, "city", city, "")
		expect.Equal(t, "bio", bio == nil, true)

		request, err := store.RequestByID(ctx, demoTarasToOlha)
		expect.NoError(t, "load Taras's request", err)
		expect.Equal(t, "requester name", request.Requester.FirstName+" "+request.Requester.LastName, "Видалений користувач")
		expect.Equal(t, "request message", request.Message, "")
		rows, err := store.conn(ctx).Query(ctx, `SELECT body FROM messages WHERE author_id = $1`, demoTaras)
		expect.NoError(t, "read Taras's messages", err)
		bodies, err := pgx.CollectRows(rows, pgx.RowTo[string])
		expect.NoError(t, "scan Taras's messages", err)
		expect.Equal(t, "Taras's seeded messages", len(bodies) > 0, true)
		for _, body := range bodies {
			expect.Equal(t, "Taras's message", body, DeletedMessageBody)
		}

		expect.Error(t, "delete the account twice", store.AnonymizeUser(ctx, demoTaras), ErrNotFound)
	})
}
