package validate

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxNameLength = 100

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func Name(raw string) (string, bool) {
	name, ok := Line(raw, MaxNameLength)
	return name, ok && name != ""
}

func Line(raw string, maxLength int) (string, bool) {
	return trimmed(raw, maxLength, false)
}

func Text(raw string, maxLength int) (string, bool) {
	return trimmed(raw, maxLength, true)
}

func UUID(value string) bool {
	return uuidPattern.MatchString(value)
}

func trimmed(raw string, maxLength int, multiline bool) (string, bool) {
	value := strings.TrimSpace(raw)
	if utf8.RuneCountInString(value) > maxLength {
		return "", false
	}
	for _, r := range value {
		if multiline && (r == '\n' || r == '\r' || r == '\t') {
			continue
		}
		if unicode.IsControl(r) {
			return "", false
		}
	}
	return value, true
}
