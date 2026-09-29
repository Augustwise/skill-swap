package data

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// demoUniversityID is the university created by the demo seed data.
const demoUniversityID = "10000000-0000-0000-0000-000000000001"

// Checks transactions on a real PostgreSQL database: an error rolls everything
// back, success commits, nested calls join the outer transaction, and a
// duplicate email is reported as ErrEmailTaken.
// Skipped unless TEST_DATABASE_URL is set.
func TestTransactionAgainstLocalPostgres(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL for the local PostgreSQL integration test")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	store := NewPostgres(pool)
	ctx := context.Background()
	// A unique email per run avoids clashes with users from earlier runs.
	email := fmt.Sprintf("tx-%d@students.example.test", time.Now().UnixNano())
	failure := errors.New("fail before commit")

	// Part 1: create a user, then return an error so the transaction rolls back.
	err = store.WithinTx(ctx, func(ctx context.Context) error {
		userID, err := store.CreateUser(ctx, NewUser{UniversityID: demoUniversityID, Email: email, FirstName: "A", LastName: "B"})
		if err != nil {
			return err
		}

		// A nested WithinTx must reuse the outer transaction, not start a new one.
		err = store.WithinTx(ctx, func(ctx context.Context) error {
			return store.CreateLocalIdentity(ctx, userID, "$2a$12$placeholder")
		})
		if err != nil {
			return err
		}
		// The uncommitted user must be visible inside the same transaction.
		if _, _, err := store.UserByEmail(ctx, email); err != nil {
			return fmt.Errorf("user is not visible inside the transaction: %w", err)
		}
		return failure
	})
	if !errors.Is(err, failure) {
		t.Fatalf("WithinTx err = %v", err)
	}
	// After the rollback the user must not exist.
	if _, _, err := store.UserByEmail(ctx, email); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after rollback err = %v, want ErrNotFound", err)
	}

	// Part 2: the same steps without an error must be committed.
	err = store.WithinTx(ctx, func(ctx context.Context) error {
		userID, err := store.CreateUser(ctx, NewUser{UniversityID: demoUniversityID, Email: email, FirstName: "A", LastName: "B"})
		if err != nil {
			return err
		}
		return store.CreateLocalIdentity(ctx, userID, "$2a$12$placeholder")
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.UserByEmail(ctx, email); err != nil {
		t.Fatalf("after commit err = %v", err)
	}

	// Part 3: creating a user with the same email again must fail.
	if _, err := store.CreateUser(ctx, NewUser{UniversityID: demoUniversityID, Email: email, FirstName: "A", LastName: "B"}); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("duplicate err = %v, want ErrEmailTaken", err)
	}
}

// Checks teaching and learning skill lists on a real PostgreSQL database:
// duplicates are rejected per list, one user cannot change another user's
// skills, and a removed skill can be added again.
// Skipped unless TEST_DATABASE_URL is set.
func TestSkillListsAgainstLocalPostgres(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL for the local PostgreSQL integration test")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	store := NewPostgres(pool)
	ctx := context.Background()
	// Guitar is a skill from the demo seed data.
	const guitarSkillID = "30000000-0000-0000-0000-000000000001"
	// Two users with unique emails: the owner of the skills and another user.
	owner, err := store.CreateUser(ctx, NewUser{UniversityID: demoUniversityID,
		Email: fmt.Sprintf("owner-%d@students.example.test", time.Now().UnixNano()), FirstName: "A", LastName: "B"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := store.CreateUser(ctx, NewUser{UniversityID: demoUniversityID,
		Email: fmt.Sprintf("other-%d@students.example.test", time.Now().UnixNano()), FirstName: "C", LastName: "D"})
	if err != nil {
		t.Fatal(err)
	}

	// Adding the same skill twice to one list fails, but the other list is independent.
	if err := store.AddUserSkill(ctx, TeachingList, owner, guitarSkillID, "ADVANCED"); err != nil {
		t.Fatal(err)
	}
	if err := store.AddUserSkill(ctx, TeachingList, owner, guitarSkillID, "BEGINNER"); !errors.Is(err, ErrSkillAlreadyAdded) {
		t.Fatalf("duplicate err = %v, want ErrSkillAlreadyAdded", err)
	}
	if err := store.AddUserSkill(ctx, LearningList, owner, guitarSkillID, "BEGINNER"); err != nil {
		t.Fatalf("the same skill in the other list: %v", err)
	}
	// Another user has no such skill in their list, so changing or removing it gives ErrNotFound.
	if err := store.UpdateUserSkillLevel(ctx, TeachingList, other, guitarSkillID, "BEGINNER"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other user's level change err = %v, want ErrNotFound", err)
	}
	if err := store.RemoveUserSkill(ctx, TeachingList, other, guitarSkillID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other user's removal err = %v, want ErrNotFound", err)
	}

	// Removing from the teaching list must leave the learning list unchanged.
	if err := store.RemoveUserSkill(ctx, TeachingList, owner, guitarSkillID); err != nil {
		t.Fatal(err)
	}
	profile, err := store.ProfileByUserID(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	if len(profile.TeachingSkills) != 0 || len(profile.LearningSkills) != 1 {
		t.Fatalf("teaching = %v, learning = %v", profile.TeachingSkills, profile.LearningSkills)
	}
	// The removed skill can be added again.
	if err := store.AddUserSkill(ctx, TeachingList, owner, guitarSkillID, "INTERMEDIATE"); err != nil {
		t.Fatalf("adding a removed skill again: %v", err)
	}
}
