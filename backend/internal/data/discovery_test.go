package data

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Demo students
const (
	demoOlha    = "60000000-0000-0000-0000-000000000001"
	demoAndrii  = "60000000-0000-0000-0000-000000000002"
	demoMarko   = "60000000-0000-0000-0000-000000000003"
	demoIryna   = "60000000-0000-0000-0000-000000000004"
	demoDmytro  = "60000000-0000-0000-0000-000000000005"
	demoSofiia  = "60000000-0000-0000-0000-000000000006"
	demoTaras   = "60000000-0000-0000-0000-000000000007"
	demoOleh    = "60000000-0000-0000-0000-000000000010"
	typography  = "30000000-0000-0000-0000-000000000011"
	branding    = "30000000-0000-0000-0000-000000000012"
	guitarSkill = "30000000-0000-0000-0000-000000000001"
	photoshop   = "30000000-0000-0000-0000-000000000002"
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
			if got := matchIDs(matches); !slices.Equal(got, test.want) || total != len(test.want) {
				t.Fatalf("matches = %v (total %d), want %v", got, total, test.want)
			}
		})
	}

	// The explanation names the exact skills and the usable formats.
	matches, _, err := store.MutualMatches(ctx, demoOlha, 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	andrii, taras := matches[0], matches[1]
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

	// The second page of one-item pages holds Taras; a page past the end is empty
	// but still reports the total.
	page, total, err := store.MutualMatches(ctx, demoOlha, 1, 1)
	if err != nil || !slices.Equal(matchIDs(page), []string{demoTaras}) || total != 2 {
		t.Fatalf("second page = %v, total %d, err %v", matchIDs(page), total, err)
	}
	page, total, err = store.MutualMatches(ctx, demoOlha, 20, 40)
	if err != nil || len(page) != 0 || total != 2 {
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
