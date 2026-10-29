package main

import (
	"testing"
	"time"

	"skillswap/backend/internal/expect"
)

func TestReportAcceptanceThresholds(t *testing.T) {
	for _, test := range []struct {
		name    string
		latency time.Duration
		errors  int
		want    bool
	}{
		{"at latency limit", 2 * time.Second, 0, true},
		{"just over latency limit", 2*time.Second + time.Nanosecond, 0, false},
		{"under one percent errors", time.Millisecond, 1, true},
		{"exactly one percent errors", time.Millisecond, 2, false},
		{"over one percent errors", time.Millisecond, 3, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			results := make([]result, 200)
			for i := range results {
				group := "search"
				if i%2 != 0 {
					group = "matches"
				}
				results[i] = result{group: group, elapsed: test.latency, failed: i < test.errors}
			}
			expect.Equal(t, "acceptance verdict", report(results), test.want)
		})
	}
	expect.Equal(t, "no measurements cannot pass", report(nil), false)
}

func TestReportUsesNinetyFifthPercentile(t *testing.T) {
	results := make([]result, 100)
	for i := range results {
		results[i] = result{group: "search", elapsed: time.Millisecond}
	}
	for i := 0; i < 5; i++ {
		results[i].elapsed = 3 * time.Second
	}
	expect.Equal(t, "five percent slow requests allowed", report(results), true)
	results[5].elapsed = 3 * time.Second
	expect.Equal(t, "six percent slow requests rejected", report(results), false)
}
