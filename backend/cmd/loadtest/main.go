// Command loadtest measures the API under load: virtual students search and open mutual
// matches against a running API, and the run passes when 95% of the requests answer within
// 2 seconds and less than 1% fail. The students come from seeds/load.sql.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/signal"
	"slices"
	"sync"
	"time"
)

const (
	maxP95       = 2 * time.Second
	maxErrorRate = 0.01
	loadStudents = 1000
)

// requests is the mix every virtual student repeats: search by skill name, by name,
// with filters, and the first two pages of mutual matches.
var requests = []struct {
	group string
	path  string
}{
	{"search", "/students?q=" + url.QueryEscape("Photoshop")},
	{"search", "/students?q=" + url.QueryEscape("Гітара") + "&format=ONLINE"},
	{"search", "/students?q=" + url.QueryEscape("Шевченко")},
	{"search", "/students?mutual=true&page=2"},
	{"search", "/students?level=ADVANCED&format=OFFLINE"},
	{"matches", "/me/matches"},
	{"matches", "/me/matches?page=2"},
}

type options struct {
	baseURL  string
	origin   string
	password string
	users    int
	duration time.Duration
	pause    time.Duration
	timeout  time.Duration
}

type result struct {
	group   string
	elapsed time.Duration
	failed  bool
}

func main() {
	var opts options
	flag.StringVar(&opts.baseURL, "base", "http://127.0.0.1:8080/api/v1", "API base URL")
	flag.StringVar(&opts.origin, "origin", "http://localhost:3000", "Origin header for sign-in")
	flag.StringVar(&opts.password, "password", "SkillSwapDemo2026", "password of the load.NNNN students")
	flag.IntVar(&opts.users, "users", 50, "number of concurrent virtual students")
	flag.DurationVar(&opts.duration, "duration", 10*time.Minute, "test duration")
	flag.DurationVar(&opts.pause, "pause", time.Second, "pause between two requests of one student")
	flag.DurationVar(&opts.timeout, "timeout", 10*time.Second, "timeout of one request")
	flag.Parse()
	if opts.users < 1 || opts.users > loadStudents {
		fmt.Fprintf(os.Stderr, "-users must be from 1 to %d\n", loadStudents)
		os.Exit(2)
	}
	ok, err := run(opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if !ok {
		os.Exit(1)
	}
}

func run(opts options) (bool, error) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	fmt.Printf("Signing in %d students...\n", opts.users)
	clients := make([]*http.Client, opts.users)
	errs := make([]error, opts.users)
	var wg sync.WaitGroup
	for i := range clients {
		wg.Go(func() { clients[i], errs[i] = signIn(ctx, opts, i+1) })
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return false, err
	}

	fmt.Printf("Running for %s...\n", opts.duration)
	ctx, cancel := context.WithTimeout(ctx, opts.duration)
	defer cancel()
	var mu sync.Mutex
	var results []result
	for i, client := range clients {
		wg.Go(func() {
			own := student(ctx, opts, client, i)
			mu.Lock()
			results = append(results, own...)
			mu.Unlock()
		})
	}
	wg.Wait()
	return report(results), nil
}

// signIn logs in as load.NNNN and returns a client that keeps the session cookie.
func signIn(ctx context.Context, opts options, n int) (*http.Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Jar: jar, Timeout: opts.timeout}
	body, _ := json.Marshal(map[string]string{
		"email":    fmt.Sprintf("load.%04d@students.example.test", n),
		"password": opts.password,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, opts.baseURL+"/auth/login", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", opts.origin)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sign-in of load.%04d: %w", n, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		text, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sign-in of load.%04d: HTTP %d %s (run go run ./cmd/db seed-load first)", n, resp.StatusCode, text)
	}
	return client, nil
}

// student sends the request mix in a loop until the context ends.
func student(ctx context.Context, opts options, client *http.Client, offset int) []result {
	var results []result
	for i := offset; ctx.Err() == nil; i++ {
		next := requests[i%len(requests)]
		start := time.Now()
		failed := get(ctx, client, opts.baseURL+next.path) != nil
		if ctx.Err() != nil {
			break
		}
		results = append(results, result{group: next.group, elapsed: time.Since(start), failed: failed})
		select {
		case <-ctx.Done():
		case <-time.After(opts.pause):
		}
	}
	return results
}

func get(ctx context.Context, client *http.Client, target string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

// report prints the latency per request group and overall, and says whether the limits hold.
func report(results []result) bool {
	if len(results) == 0 {
		fmt.Println("No requests were completed")
		return false
	}
	fmt.Printf("%-8s %8s %8s %10s %10s %10s\n", "group", "requests", "errors", "p50", "p95", "max")
	for _, group := range []string{"search", "matches", "all"} {
		s := summarize(results, group)
		if len(s.times) > 0 {
			fmt.Printf("%-8s %8d %8d %10s %10s %10s\n", group, len(s.times), s.failed,
				percentile(s.times, 0.50), percentile(s.times, 0.95), percentile(s.times, 1))
		}
	}
	all := summarize(results, "all")
	p95 := percentile(all.times, 0.95)
	errorRate := float64(all.failed) / float64(len(all.times))
	ok := p95 <= maxP95 && errorRate < maxErrorRate
	verdict := "PASSED"
	if !ok {
		verdict = "FAILED"
	}
	fmt.Printf("NFR-01 %s: p95 = %s (limit %s), errors = %.2f%% (limit below %.0f%%)\n",
		verdict, p95, maxP95, errorRate*100, maxErrorRate*100)
	return ok
}

type summary struct {
	times  []time.Duration
	failed int
}

// summarize collects the sorted response times and the failures of one group, or of all requests.
func summarize(results []result, group string) summary {
	var s summary
	for _, r := range results {
		if group == "all" || r.group == group {
			s.times = append(s.times, r.elapsed)
			if r.failed {
				s.failed++
			}
		}
	}
	slices.Sort(s.times)
	return s
}

// percentile returns the value below which the given share of the sorted times falls.
func percentile(sorted []time.Duration, share float64) time.Duration {
	index := int(math.Ceil(share*float64(len(sorted)))) - 1
	return sorted[max(index, 0)]
}
