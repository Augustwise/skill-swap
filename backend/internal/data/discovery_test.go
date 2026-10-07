package data

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Demo students
const (
	demoOlha     = "60000000-0000-0000-0000-000000000001"
	demoAndrii   = "60000000-0000-0000-0000-000000000002"
	demoMarko    = "60000000-0000-0000-0000-000000000003"
	demoIryna    = "60000000-0000-0000-0000-000000000004"
	demoDmytro   = "60000000-0000-0000-0000-000000000005"
	demoSofiia   = "60000000-0000-0000-0000-000000000006"
	demoTaras    = "60000000-0000-0000-0000-000000000007"
	demoOleh     = "60000000-0000-0000-0000-000000000010"
	demoKateryna = "60000000-0000-0000-0000-000000000011"
	design       = "20000000-0000-0000-0000-000000000002"
	music        = "20000000-0000-0000-0000-000000000001"
	typography   = "30000000-0000-0000-0000-000000000011"
	branding     = "30000000-0000-0000-0000-000000000012"
	guitarSkill  = "30000000-0000-0000-0000-000000000001"
	photoshop    = "30000000-0000-0000-0000-000000000002"
)

// openTestStore connects to TEST_DATABASE_URL or skips the test.
func openTestStore(t *testing.T) *Postgres {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL for the local PostgreSQL integration test")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return NewPostgres(pool)
}

// matchIDs returns the user IDs of the matches in order.
// demoOnly keeps the demo students, so other users in a shared database do not
// change the expected lists.
func demoOnly(ids []string) []string {
	return slices.DeleteFunc(ids, func(id string) bool { return !strings.HasPrefix(id, "60000000-") })
}

func matchIDs(matches []Match) []string {
	ids := []string{}
	for _, m := range matches {
		ids = append(ids, m.UserID)
	}
	return ids
}

// skillIDs returns the skill IDs of a matched skill list in order.
func skillIDs(skills []UserSkill) []string {
	ids := []string{}
	for _, s := range skills {
		ids = append(ids, s.SkillID)
	}
	return ids
}

// Checks LR1 3.2 on the demo students: who is a mutual match, in which order,
// with which skills and formats, and how pages work.
func TestMutualMatchesOnDemoData(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	for _, test := range []struct {
		name   string
		viewer string
		want   []string
	}{
		// Andrii (online) and Taras (offline, same city) form "guitar <-> Photoshop" with Olha.
		// Marko is one-sided, Kateryna has no learning skills, Nataliia is hidden,
		// Viktor is unverified and Oleh is blocked by Olha.
		{"Olha", demoOlha, []string{demoAndrii, demoTaras}},
		{"Andrii", demoAndrii, []string{demoOlha}},
		// Dmytro has three skill pairs with Iryna, Marko two, so Dmytro comes first.
		{"Iryna", demoIryna, []string{demoDmytro, demoMarko}},
		// Sofiia matches Andrii and Taras by skills, but offline in another city.
		{"Sofiia", demoSofiia, []string{}},
		// A block hides both directions: Olha blocked Oleh, so he does not see her either.
		{"Oleh", demoOleh, []string{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			matches, total, err := store.MutualMatches(ctx, test.viewer, 20, 0)
			if err != nil {
				t.Fatal(err)
			}
			if got := demoOnly(matchIDs(matches)); !slices.Equal(got, test.want) || total != len(matches) {
				t.Fatalf("matches = %v (total %d), want %v", got, total, test.want)
			}
		})
	}

	// The explanation names the exact skills and the usable formats.
	matches, _, err := store.MutualMatches(ctx, demoOlha, 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	byID := func(id string) Match {
		t.Helper()
		i := slices.IndexFunc(matches, func(m Match) bool { return m.UserID == id })
		if i < 0 {
			t.Fatalf("%s is not in %v", id, matchIDs(matches))
		}
		return matches[i]
	}
	andrii, taras := byID(demoAndrii), byID(demoTaras)
	if !slices.Equal(skillIDs(andrii.CanTeach), []string{photoshop}) || andrii.CanTeach[0].Level != "ADVANCED" {
		t.Fatalf("Andrii can teach %+v", andrii.CanTeach)
	}
	if !slices.Equal(skillIDs(andrii.WantsToLearn), []string{guitarSkill}) || andrii.WantsToLearn[0].Level != "BEGINNER" {
		t.Fatalf("Andrii wants to learn %+v", andrii.WantsToLearn)
	}
	if !slices.Equal(andrii.CommonFormats, []string{"ONLINE", "OFFLINE"}) || !slices.Equal(taras.CommonFormats, []string{"OFFLINE"}) {
		t.Fatalf("common formats: Andrii %v, Taras %v", andrii.CommonFormats, taras.CommonFormats)
	}
	if andrii.FirstName != "Андрій" || andrii.UniversityName == "" || andrii.City != "Київ" {
		t.Fatalf("Andrii = %+v", andrii)
	}

	// A page past the end is empty but still reports the total.
	page, total, err := store.MutualMatches(ctx, demoOlha, 1, 1)
	if err != nil || !slices.Equal(matchIDs(page), matchIDs(matches[1:2])) || total != len(matches) {
		t.Fatalf("second page = %v, total %d, err %v", matchIDs(page), total, err)
	}
	page, total, err = store.MutualMatches(ctx, demoOlha, 20, 400)
	if err != nil || len(page) != 0 || total != len(matches) {
		t.Fatalf("page past the end = %v, total %d, err %v", matchIDs(page), total, err)
	}
}

// Checks that the list follows the current data (FR-05): a match appears once both
// sides fit, and disappears after a skill is removed or the profile is hidden.
func TestMutualMatchesFollowCurrentData(t *testing.T) {
	store := openTestStore(t)
	rollback := errors.New("roll back test data")
	err := store.WithinTx(context.Background(), func(ctx context.Context) error {
		// Typography and branding are not used by the demo students, so only these two can match.
		newStudent := func(name string, teach, learn string) string {
			t.Helper()
			email := fmt.Sprintf("%s-%d@students.example.test", name, time.Now().UnixNano())
			id, err := store.CreateUser(ctx, NewUser{UniversityID: demoUniversityID, Email: email, FirstName: name, LastName: "Test"})
			if err != nil {
				t.Fatal(err)
			}
			if err := store.MarkEmailVerified(ctx, id); err != nil {
				t.Fatal(err)
			}
			if err := store.UpdateProfile(ctx, id, ProfileUpdate{FirstName: name, LastName: "Test", City: "Київ",
				Formats: []string{FormatOnline}}); err != nil {
				t.Fatal(err)
			}
			if err := store.AddUserSkill(ctx, TeachingList, id, teach, "ADVANCED"); err != nil {
				t.Fatal(err)
			}
			if err := store.AddUserSkill(ctx, LearningList, id, learn, "BEGINNER"); err != nil {
				t.Fatal(err)
			}
			return id
		}
		expect := func(step, viewer string, want ...string) {
			t.Helper()
			matches, _, err := store.MutualMatches(ctx, viewer, 20, 0)
			if err != nil {
				t.Fatal(err)
			}
			if got := matchIDs(matches); !slices.Equal(got, append([]string{}, want...)) {
				t.Fatalf("%s: matches = %v, want %v", step, got, want)
			}
		}

		a := newStudent("a", typography, branding)
		b := newStudent("b", branding, typography)
		expect("both sides fit", a, b)
		expect("both sides fit, other side", b, a)

		// One-sided: B no longer teaches what A wants.
		if err := store.RemoveUserSkill(ctx, TeachingList, b, branding); err != nil {
			t.Fatal(err)
		}
		expect("after removing a skill", a)
		if err := store.AddUserSkill(ctx, TeachingList, b, branding, "INTERMEDIATE"); err != nil {
			t.Fatal(err)
		}
		expect("after adding it back", a, b)

		// No shared format: B switches to offline only.
		if err := store.UpdateProfile(ctx, b, ProfileUpdate{FirstName: "b", LastName: "Test", City: "Київ",
			Formats: []string{FormatOffline}}); err != nil {
			t.Fatal(err)
		}
		expect("without a shared format", a)
		if err := store.UpdateProfile(ctx, b, ProfileUpdate{FirstName: "b", LastName: "Test", City: "Київ",
			Formats: []string{FormatOnline}}); err != nil {
			t.Fatal(err)
		}

		// A hidden profile disappears from the other side's list.
		if _, err := store.conn(ctx).Exec(ctx, `INSERT INTO user_profile_settings (user_id, is_discoverable)
			VALUES ($1, false)`, b); err != nil {
			t.Fatal(err)
		}
		expect("after hiding the profile", a)
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("WithinTx err = %v", err)
	}
}

func cardIDs(cards []StudentCard) []string {
	ids := []string{}
	for _, c := range cards {
		ids = append(ids, c.UserID)
	}
	return ids
}

// Skipped unless TEST_DATABASE_URL points to a migrated, demo-seeded database.
func TestSearchStudentsOnDemoData(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	for _, test := range []struct {
		name   string
		viewer string
		filter StudentFilter
		want   []string
	}{
		// Mutual matches come first. Iryna and Sofiia only want Photoshop; Nataliia is
		// hidden, Viktor unverified and Oleh blocked by Olha.
		{"skill name", demoOlha, StudentFilter{Query: "Photoshop"}, []string{demoAndrii, demoTaras, demoKateryna, demoMarko}},
		{"part of a skill name", demoOlha, StudentFilter{Query: "photo"}, []string{demoAndrii, demoTaras, demoKateryna, demoMarko}},
		// Andrii teaches Photoshop at ADVANCED and Illustrator at INTERMEDIATE, so he does not fit.
		{"skill and level on the same skill", demoOlha, StudentFilter{Query: "Photoshop", Level: "INTERMEDIATE"}, []string{demoMarko}},
		{"category and level", demoOlha, StudentFilter{CategoryID: design, Level: "INTERMEDIATE"}, []string{demoAndrii, demoIryna, demoMarko}},
		{"person's name", demoOlha, StudentFilter{Query: "коваль"}, []string{demoAndrii}},
		{"name and another category", demoOlha, StudentFilter{Query: "Коваль", CategoryID: music}, []string{}},
		{"format", demoOlha, StudentFilter{Format: FormatOffline}, []string{demoAndrii, demoTaras, demoSofiia}},
		{"mutual only", demoOlha, StudentFilter{MutualOnly: true}, []string{demoAndrii, demoTaras}},
		{"all filters together", demoOlha, StudentFilter{Query: "Photoshop", Format: FormatOffline, MutualOnly: true}, []string{demoAndrii, demoTaras}},
		{"wildcards are plain text", demoOlha, StudentFilter{Query: "%"}, []string{}},
		{"nothing found", demoOlha, StudentFilter{Query: "Скрипка"}, []string{}},
		// Olha blocked Oleh, so he does not find her either.
		{"block hides both ways", demoOleh, StudentFilter{Query: "Гітара"}, []string{demoSofiia}},
	} {
		t.Run(test.name, func(t *testing.T) {
			cards, total, err := store.SearchStudents(ctx, test.viewer, test.filter, 20, 0)
			if err != nil {
				t.Fatal(err)
			}
			if got := demoOnly(cardIDs(cards)); !slices.Equal(got, test.want) || total != len(cards) {
				t.Fatalf("students = %v (total %d), want %v", got, total, test.want)
			}
		})
	}

	cards, _, err := store.SearchStudents(ctx, demoOlha, StudentFilter{Query: "Photoshop"}, 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	byID := func(id string) StudentCard {
		t.Helper()
		i := slices.IndexFunc(cards, func(c StudentCard) bool { return c.UserID == id })
		if i < 0 {
			t.Fatalf("%s is not in %v", id, cardIDs(cards))
		}
		return cards[i]
	}
	andrii, marko := byID(demoAndrii), byID(demoMarko)
	if !andrii.Mutual || marko.Mutual {
		t.Fatalf("mutual: Andrii %v, Marko %v", andrii.Mutual, marko.Mutual)
	}
	if got := skillIDs(andrii.TeachingSkills); len(got) != 2 || andrii.TeachingSkills[0].Name != "Illustrator" {
		t.Fatalf("Andrii teaches %+v", andrii.TeachingSkills)
	}
	if !slices.Equal(andrii.Formats, []string{"ONLINE", "OFFLINE"}) || andrii.City != "Київ" || andrii.UniversityName == "" {
		t.Fatalf("Andrii = %+v", andrii)
	}

	page, total, err := store.SearchStudents(ctx, demoOlha, StudentFilter{Query: "Photoshop"}, 2, 2)
	if err != nil || !slices.Equal(cardIDs(page), cardIDs(cards[2:4])) || total != len(cards) {
		t.Fatalf("second page = %v, total %d, err %v", cardIDs(page), total, err)
	}
	page, total, err = store.SearchStudents(ctx, demoOlha, StudentFilter{Query: "Photoshop"}, 20, 400)
	if err != nil || len(page) != 0 || total != len(cards) {
		t.Fatalf("page past the end = %v, total %d, err %v", cardIDs(page), total, err)
	}
}
