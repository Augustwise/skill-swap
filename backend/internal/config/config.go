package config

import (
	"errors"
	"net"
	"net/mail"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	APIAddr     string
	DatabaseURL string
	FrontendURL string
	SMTP        SMTPConfig

	AllowedEmailDomains []string
}

type SMTPConfig struct {
	Addr     string
	Username string
	Password string
	From     string
}

func Load() (Config, error) {
	if err := loadEnvFile(); err != nil {
		return Config{}, err
	}
	smtp, err := smtpFromEnvironment()
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		APIAddr:     value("API_ADDR", "127.0.0.1:8080"),
		DatabaseURL: strings.TrimSpace(os.Getenv("DATABASE_URL")),
		FrontendURL: value("FRONTEND_ORIGIN", "http://localhost:3000"),
		SMTP:        smtp,
	}
	cfg.AllowedEmailDomains, err = emailDomains(value("ALLOWED_EMAIL_DOMAINS", "students.example.test"))
	if err != nil {
		return Config{}, err
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required: set a complete PostgreSQL URL in .env")
	}
	parsed, err := url.Parse(cfg.DatabaseURL)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || parsed.Hostname() == "" {
		return Config{}, errors.New("DATABASE_URL must be a complete PostgreSQL URL (postgres://USER:PASSWORD@HOST:5432/DATABASE), not just a hostname")
	}
	dbHost := parsed.Hostname()
	dbIP := net.ParseIP(dbHost)
	localDB := strings.EqualFold(dbHost, "localhost") || (dbIP != nil && dbIP.IsLoopback())
	if !localDB && (parsed.Query().Get("sslmode") != "verify-full" || parsed.Query().Get("sslrootcert") == "") {
		return Config{}, errors.New("remote DATABASE_URL requires sslmode=verify-full and sslrootcert")
	}
	host, _, err := net.SplitHostPort(cfg.APIAddr)
	if err != nil {
		return Config{}, errors.New("API_ADDR must be a host:port pair")
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return Config{}, errors.New("API_ADDR must use a loopback IP address")
	}
	if cfg.FrontendURL != "http://localhost:3000" && cfg.FrontendURL != "http://127.0.0.1:3000" {
		return Config{}, errors.New("FRONTEND_ORIGIN must be a local frontend origin")
	}
	return cfg, nil
}

func LoadSMTP() (SMTPConfig, error) {
	if err := loadEnvFile(); err != nil {
		return SMTPConfig{}, err
	}
	return smtpFromEnvironment()
}

func smtpFromEnvironment() (SMTPConfig, error) {
	cfg := SMTPConfig{
		Addr:     value("SMTP_ADDR", "127.0.0.1:1025"),
		Username: strings.TrimSpace(os.Getenv("SMTP_USERNAME")),
		Password: os.Getenv("SMTP_PASSWORD"),
		From:     value("MAIL_FROM", "no-reply@students.example.test"),
	}
	host, portText, err := net.SplitHostPort(cfg.Addr)
	if err != nil || strings.TrimSpace(host) == "" {
		return SMTPConfig{}, errors.New("SMTP_ADDR must be a host:port pair")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return SMTPConfig{}, errors.New("SMTP_ADDR must contain a valid port")
	}
	if (cfg.Username == "") != (cfg.Password == "") {
		return SMTPConfig{}, errors.New("SMTP_USERNAME and SMTP_PASSWORD must be set together")
	}
	address, err := mail.ParseAddress(cfg.From)
	if err != nil || address.Address == "" {
		return SMTPConfig{}, errors.New("MAIL_FROM must be a valid email address")
	}
	return cfg, nil
}

func emailDomains(raw string) ([]string, error) {
	domains := make([]string, 0)
	for _, part := range strings.Split(raw, ",") {
		domain := strings.ToLower(strings.TrimSpace(part))
		if domain == "" {
			continue
		}
		if strings.ContainsAny(domain, "@ /\\") || !strings.Contains(domain, ".") {
			return nil, errors.New("ALLOWED_EMAIL_DOMAINS must be a comma-separated list of domains such as students.example.test")
		}
		domains = append(domains, domain)
	}
	if len(domains) == 0 {
		return nil, errors.New("ALLOWED_EMAIL_DOMAINS must contain at least one domain")
	}
	return domains, nil
}

func loadEnvFile() error {
	if err := godotenv.Load(".env"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func value(key, fallback string) string {
	if text := strings.TrimSpace(os.Getenv(key)); text != "" {
		return text
	}
	return fallback
}
