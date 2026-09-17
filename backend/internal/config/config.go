package config

import (
	"errors"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	APIAddr     string
	DatabaseURL string
	FrontendURL string
}

func Load() (Config, error) {
	if err := loadEnvFile(); err != nil {
		return Config{}, err
	}
	cfg := Config{
		APIAddr:     value("API_ADDR", "127.0.0.1:8080"),
		DatabaseURL: strings.TrimSpace(os.Getenv("DATABASE_URL")),
		FrontendURL: value("FRONTEND_ORIGIN", "http://localhost:3000"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	parsed, err := url.Parse(cfg.DatabaseURL)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || parsed.Hostname() == "" {
		return Config{}, errors.New("DATABASE_URL must be a PostgreSQL URL")
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

func loadEnvFile() error {
	for _, path := range []string{".env.local", ".env"} {
		_, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		return godotenv.Load(path)
	}
	return nil
}

func value(key, fallback string) string {
	if text := strings.TrimSpace(os.Getenv(key)); text != "" {
		return text
	}
	return fallback
}
