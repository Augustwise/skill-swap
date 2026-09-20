package config

import "testing"

func TestLoadRequiresDatabaseURL(t *testing.T) {
	setValidSMTPEnv(t)
	t.Setenv("DATABASE_URL", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected missing database URL to fail")
	}
}

func TestLoadRejectsNonLoopbackAddress(t *testing.T) {
	setValidSMTPEnv(t)
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("API_ADDR", "0.0.0.0:8080")
	if _, err := Load(); err == nil {
		t.Fatal("expected non-loopback listener to fail")
	}
}

func TestLoadAcceptsLocalAddress(t *testing.T) {
	setValidSMTPEnv(t)
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("API_ADDR", "127.0.0.1:8080")
	t.Setenv("FRONTEND_ORIGIN", "http://localhost:3000")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SMTP.Addr != "127.0.0.1:1025" || cfg.SMTP.From != "no-reply@students.example.test" {
		t.Fatalf("unexpected SMTP config: %+v", cfg.SMTP)
	}
}

func TestLoadRequiresTLSForRemoteDatabase(t *testing.T) {
	setValidSMTPEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@example.rds.amazonaws.com/skillswap?sslmode=disable")
	if _, err := Load(); err == nil {
		t.Fatal("expected remote database without verified TLS to fail")
	}
}

func TestLoadSMTPDefaults(t *testing.T) {
	t.Setenv("SMTP_ADDR", "")
	t.Setenv("SMTP_USERNAME", "")
	t.Setenv("SMTP_PASSWORD", "")
	t.Setenv("MAIL_FROM", "")
	cfg, err := LoadSMTP()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Addr != "127.0.0.1:1025" || cfg.From != "no-reply@students.example.test" {
		t.Fatalf("unexpected SMTP defaults: %+v", cfg)
	}
}

func TestLoadSMTPRejectsInvalidAddress(t *testing.T) {
	setValidSMTPEnv(t)
	t.Setenv("SMTP_ADDR", "mail.example.test")
	if _, err := LoadSMTP(); err == nil {
		t.Fatal("expected invalid SMTP address to fail")
	}
}

func TestLoadSMTPRequiresCompleteCredentials(t *testing.T) {
	setValidSMTPEnv(t)
	t.Setenv("SMTP_USERNAME", "mailer")
	t.Setenv("SMTP_PASSWORD", "")
	if _, err := LoadSMTP(); err == nil {
		t.Fatal("expected incomplete SMTP credentials to fail")
	}
}

func TestLoadSMTPRejectsInvalidFrom(t *testing.T) {
	setValidSMTPEnv(t)
	t.Setenv("MAIL_FROM", "not-an-email")
	if _, err := LoadSMTP(); err == nil {
		t.Fatal("expected invalid sender to fail")
	}
}

func setValidSMTPEnv(t *testing.T) {
	t.Helper()
	t.Setenv("SMTP_ADDR", "127.0.0.1:1025")
	t.Setenv("SMTP_USERNAME", "")
	t.Setenv("SMTP_PASSWORD", "")
	t.Setenv("MAIL_FROM", "no-reply@students.example.test")
}
