// Package expect stops a test with a readable message when a checked value is wrong.
package expect

import (
	"errors"
	"slices"
	"testing"
)

// Equal stops the test when got differs from want; name says what was checked.
func Equal[T comparable](t testing.TB, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}

// Slice stops the test when got and want differ in length, order or values.
func Slice[T comparable](t testing.TB, name string, got, want []T) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}

// Error stops the test when err does not match want with errors.Is.
func Error(t testing.TB, action string, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("%s: err = %v, want %v", action, err, want)
	}
}

// NoError stops the test when err is not nil.
func NoError(t testing.TB, action string, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", action, err)
	}
}
