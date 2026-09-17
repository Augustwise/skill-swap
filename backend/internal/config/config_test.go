package config

import "testing"

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected missing database URL to fail")
	}
}

func TestLoadRejectsNonLoopbackAddress(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("API_ADDR", "0.0.0.0:8080")
	if _, err := Load(); err == nil {
		t.Fatal("expected non-loopback listener to fail")
	}
}

func TestLoadAcceptsLocalAddress(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("API_ADDR", "127.0.0.1:8080")
	t.Setenv("FRONTEND_ORIGIN", "http://localhost:3000")
	if _, err := Load(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRequiresTLSForRemoteDatabase(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@example.rds.amazonaws.com/skillswap?sslmode=disable")
	if _, err := Load(); err == nil {
		t.Fatal("expected remote database without verified TLS to fail")
	}
}
