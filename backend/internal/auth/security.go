package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"net/mail"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

const (
	bcryptCost       = 12
	minPasswordRunes = 12
	maxPasswordBytes = 72
	maxEmailLength   = 320
	maxNameLength    = 100
)

var dummyPasswordHash, _ = bcrypt.GenerateFromPassword([]byte("skillswap-timing-placeholder"), bcryptCost)

func normalizeEmail(raw string) (string, bool) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if email == "" || len(email) > maxEmailLength {
		return "", false
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || strings.Count(email, "@") != 1 {
		return "", false
	}
	return email, true
}

func emailDomain(email string) string {
	return email[strings.LastIndex(email, "@")+1:]
}

func validatePassword(password string) string {
	if !utf8.ValidString(password) {
		return "Password must be valid UTF-8 text"
	}
	if utf8.RuneCountInString(password) < minPasswordRunes {
		return "Password must contain at least 12 characters"
	}
	if len(password) > maxPasswordBytes {
		return "Password must not exceed 72 bytes"
	}
	return ""
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func passwordMatches(hash, password string) bool {
	if hash == "" {
		_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(password))
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func newToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func validToken(token string) bool {
	if len(token) != 43 {
		return false
	}
	_, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil
}

func CSRFToken(sessionToken string) string {
	sum := sha256.Sum256([]byte("csrf:" + sessionToken))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func ValidCSRF(sessionToken, header string) bool {
	expected := CSRFToken(sessionToken)
	return header != "" && subtle.ConstantTimeCompare([]byte(expected), []byte(header)) == 1
}

func cleanName(raw string) (string, bool) {
	name := strings.TrimSpace(raw)
	if name == "" || utf8.RuneCountInString(name) > maxNameLength {
		return "", false
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return "", false
		}
	}
	return name, true
}
