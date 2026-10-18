package validate

import (
	"fmt"
	"strings"
	"testing"

	"skillswap/backend/internal/expect"
)

func TestName(t *testing.T) {
	tests := []struct {
		raw  string
		want string
		ok   bool
	}{
		{"  Олена ", "Олена", true},
		{strings.Repeat("я", 100), strings.Repeat("я", 100), true},
		{strings.Repeat("я", 101), "", false},
		{"   ", "", false},
		{"Оле\nна", "", false},
	}
	for _, test := range tests {
		got, ok := Name(test.raw)
		expect.Equal(t, fmt.Sprintf("Name(%q) result", test.raw), got, test.want)
		expect.Equal(t, fmt.Sprintf("Name(%q) valid", test.raw), ok, test.ok)
	}
}

func TestLineAndText(t *testing.T) {
	got, ok := Line("", 10)
	expect.Equal(t, "empty line valid", ok, true)
	expect.Equal(t, "empty line", got, "")
	if _, ok := Line("Київ\tЦентр", 100); ok {
		t.Fatal("a single line must reject tabs and line breaks")
	}
	got, ok = Text(" Перший рядок\nДругий\t рядок ", 100)
	expect.Equal(t, "text valid", ok, true)
	expect.Equal(t, "text", got, "Перший рядок\nДругий\t рядок")
	if _, ok := Text("a\x00b", 100); ok {
		t.Fatal("text must reject NUL")
	}
	if _, ok := Text(strings.Repeat("ї", 601), 600); ok {
		t.Fatal("601 characters must exceed a 600-character limit")
	}
}

func TestUUID(t *testing.T) {
	for value, want := range map[string]bool{
		"30000000-0000-0000-0000-000000000001": true,
		"A0EEBC99-9C0B-4EF8-BB6D-6BB9BD380A11": true,
		"30000000000000000000000000000001":     false,
		"30000000-0000-0000-0000-00000000000z": false,
		"":                                     false,
		"'; DROP TABLE skills; --":             false,
	} {
		if got := UUID(value); got != want {
			t.Errorf("UUID(%q) = %t, want %t", value, got, want)
		}
	}
}
