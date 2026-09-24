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

const demoUniversityID = "10000000-0000-0000-0000-000000000001"

// TestTransactionAgainstLocalPostgres verifies PostgreSQL transaction semantics:
// 1. Rollback on failure (including nested WithinTx calls and data visibility within the transaction).
// 2. Commit on success and data persistence across transactions.
// 3. Unique email constraint violation (ErrEmailTaken) handling.
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
	email := fmt.Sprintf("tx-%d@students.example.test", time.Now().UnixNano())
	failure := errors.New("fail before commit")

	// 1. Verify rollback behavior on error
	err = store.WithinTx(ctx, func(ctx context.Context) error {
		userID, err := store.CreateUser(ctx, NewUser{UniversityID: demoUniversityID, Email: email, FirstName: "A", LastName: "B"})
		if err != nil {
			return err
		}

		err = store.WithinTx(ctx, func(ctx context.Context) error {
			return store.CreateLocalIdentity(ctx, userID, "$2a$12$placeholder")
		})
		if err != nil {
			return err
		}
		if _, _, err := store.UserByEmail(ctx, email); err != nil {
			return fmt.Errorf("user is not visible inside the transaction: %w", err)
		}
		return failure
	})
	if !errors.Is(err, failure) {
		t.Fatalf("WithinTx err = %v", err)
	}
	if _, _, err := store.UserByEmail(ctx, email); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after rollback err = %v, want ErrNotFound", err)
	}

	// 2. Verify commit behavior on success
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

	// 3. Verify unique constraint enforcement
	if _, err := store.CreateUser(ctx, NewUser{UniversityID: demoUniversityID, Email: email, FirstName: "A", LastName: "B"}); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("duplicate err = %v, want ErrEmailTaken", err)
	}
}
