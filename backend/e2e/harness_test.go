package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/quotedprintable"
	"net"
	"net/http"
	"net/mail"
	"net/textproto"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"skillswap/backend/internal/data/datatest"
	"skillswap/backend/internal/expect"
)

const (
	origin     = "http://localhost:3000"
	password   = "E2E password 2026!"
	guitar     = datatest.GuitarSkillID
	photoshop  = datatest.PhotoshopSkillID
	university = datatest.DemoUniversityID
)

var suite struct {
	root, dir, adminURL, pgBin, pgData string
	api, db, load                      string
}

func TestMain(m *testing.M) {
	if os.Getenv("E2E_PG_BIN") == "" && os.Getenv("E2E_DATABASE_URL") == "" {
		os.Exit(m.Run())
	}
	code := func() int {
		var err error
		suite.root, err = filepath.Abs("..")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		suite.dir, err = os.MkdirTemp("", "skillswap-e2e-")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		defer os.RemoveAll(suite.dir)
		suite.pgBin = os.Getenv("E2E_PG_BIN")
		if suite.pgBin == "" {
			suite.pgBin = os.Getenv("PG_BIN")
		}
		if os.Getenv("E2E_PG_BIN") != "" {
			suite.pgData = filepath.Join(suite.dir, "postgres")
			if err := pgTool("initdb", "-D", suite.pgData, "-U", "postgres", "-A", "trust", "--encoding=UTF8", "--locale-provider=icu", "--icu-locale=uk-UA"); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			addr := freeAddress()
			_, port, _ := net.SplitHostPort(addr)
			options := "listen_addresses='127.0.0.1'\nport=" + port + "\nmax_connections=120\n"
			if err := os.WriteFile(filepath.Join(suite.pgData, "postgresql.auto.conf"), []byte(options), 0600); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			if err := pgTool("pg_ctl", "-D", suite.pgData, "-l", filepath.Join(suite.dir, "postgres.log"), "-w", "start"); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			defer func() {
				if err := pgTool("pg_ctl", "-D", suite.pgData, "-m", "fast", "-w", "stop"); err != nil {
					fmt.Fprintln(os.Stderr, err)
				}
			}()
			suite.adminURL = "postgres://postgres@" + addr + "/postgres?sslmode=disable"
		} else {
			suite.adminURL = os.Getenv("E2E_DATABASE_URL")
		}
		u, err := url.Parse(suite.adminURL)
		if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || !loopback(u.Hostname()) {
			fmt.Fprintln(os.Stderr, "E2E_DATABASE_URL must be a local PostgreSQL URL; remote hosts are refused")
			return 1
		}
		for _, key := range []string{"host", "hostaddr", "port", "dbname", "database", "service", "servicefile"} {
			if u.Query().Has(key) {
				fmt.Fprintln(os.Stderr, "E2E_DATABASE_URL must set its host, port and database in the URL, without connection overrides")
				return 1
			}
		}
		for name, target := range map[string]*string{"api": &suite.api, "db": &suite.db, "loadtest": &suite.load} {
			*target = filepath.Join(suite.dir, name)
			if runtime.GOOS == "windows" {
				*target += ".exe"
			}
			cmd := exec.Command("go", "build", "-o", *target, "./cmd/"+name)
			cmd.Dir = suite.root
			if output, err := cmd.CombinedOutput(); err != nil {
				fmt.Fprintf(os.Stderr, "build %s: %v\n%s", name, err, output)
				return 1
			}
		}
		return m.Run()
	}()
	os.Exit(code)
}

func loopback(host string) bool {
	ip := net.ParseIP(host)
	return strings.EqualFold(host, "localhost") || (ip != nil && ip.IsLoopback())
}

func freeAddress() string {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()
	return address
}

func pgTool(name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	command := name
	if suite.pgBin != "" {
		command = filepath.Join(suite.pgBin, name)
	}
	output, err := os.CreateTemp(suite.dir, "pg-tool-*.log")
	if err != nil {
		return err
	}
	defer output.Close()
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdout, cmd.Stderr = output, output
	if err := cmd.Run(); err != nil {
		data, _ := os.ReadFile(output.Name())
		return fmt.Errorf("%s: %w\n%s", name, err, data)
	}
	return nil
}

func environment(values map[string]string) []string {
	var result []string
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if _, replaced := values[key]; !replaced {
			result = append(result, entry)
		}
	}
	for key, value := range values {
		result = append(result, key+"="+value)
	}
	return result
}

type lockedBuffer struct {
	sync.Mutex
	bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.Lock()
	defer b.Unlock()
	return b.Buffer.Write(p)
}
func (b *lockedBuffer) text() string { b.Lock(); defer b.Unlock(); return b.Buffer.String() }

type stack struct {
	t                *testing.T
	url, databaseURL string
	pool             *pgxpool.Pool
	mail             *smtpInbox
	process          *exec.Cmd
	done             chan error
	logs             *lockedBuffer
	secrets          []string
}

func newStack(t *testing.T) *stack {
	t.Helper()
	if suite.adminURL == "" {
		t.Skip("set E2E_PG_BIN for an isolated cluster or E2E_DATABASE_URL for a local role with CREATEDB")
	}
	s := &stack{t: t, url: "http://" + freeAddress() + "/api/v1", logs: &lockedBuffer{}}
	s.databaseURL = createDatabase(t)
	s.mail = newInbox(t)
	s.command(suite.db, nil, "up")
	s.command(suite.db, nil, "seed-test")
	var err error
	s.pool, err = pgxpool.New(context.Background(), s.databaseURL)
	expect.NoError(t, "open test database", err)
	t.Cleanup(s.pool.Close)
	t.Cleanup(func() {
		s.stop()
		for _, secret := range s.secrets {
			expect.Equal(t, "secret absent from API logs", strings.Contains(s.logs.text(), secret), false)
		}
	})
	s.start()
	return s
}

func createDatabase(t *testing.T) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, suite.adminURL)
	expect.NoError(t, "connect to local database administrator", err)
	defer admin.Close(ctx)
	name := fmt.Sprintf("skillswap_e2e_%d", time.Now().UnixNano())
	_, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize())
	expect.NoError(t, "create isolated database", err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		admin, err := pgx.Connect(ctx, suite.adminURL)
		if err != nil {
			t.Errorf("connect for database cleanup: %v", err)
			return
		}
		defer admin.Close(ctx)
		_, err = admin.Exec(ctx, "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
		if err != nil {
			t.Errorf("drop isolated database: %v", err)
		}
	})
	u, _ := url.Parse(suite.adminURL)
	u.Path = "/" + name
	return u.String()
}

func (s *stack) env(extra map[string]string) []string {
	u, _ := url.Parse(s.url)
	values := map[string]string{
		"DATABASE_URL": s.databaseURL, "API_ADDR": u.Host, "FRONTEND_ORIGIN": origin,
		"SMTP_ADDR": s.mail.address, "SMTP_USERNAME": "", "SMTP_PASSWORD": "",
		"MAIL_FROM": "e2e@students.example.test", "ALLOWED_EMAIL_DOMAINS": "students.example.test", "PG_BIN": suite.pgBin,
	}
	for key, value := range extra {
		values[key] = value
	}
	return environment(values)
}

func (s *stack) command(binary string, env map[string]string, args ...string) string {
	s.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 16*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = suite.root
	cmd.Env = s.env(env)
	output, err := cmd.CombinedOutput()
	if err != nil {
		s.t.Fatalf("%s %s: %v\n%s", filepath.Base(binary), strings.Join(args, " "), err, output)
	}
	return string(output)
}

func (s *stack) start() {
	s.t.Helper()
	s.process = exec.Command(suite.api)
	s.process.Dir = suite.dir // No application .env is loaded by the child process.
	s.process.Env = s.env(nil)
	s.process.Stdout, s.process.Stderr = s.logs, s.logs
	expect.NoError(s.t, "start API process", s.process.Start())
	s.done = make(chan error, 1)
	go func() { s.done <- s.process.Wait() }()
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-s.done:
			s.process = nil
			s.t.Fatalf("API exited before readiness: %v\n%s", err, s.logs.text())
		default:
		}
		response, err := client.Get(s.url + "/ready")
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(30 * time.Millisecond)
	}
	s.t.Fatal("API did not become ready within 15 seconds")
}

func (s *stack) stop() {
	if s.process == nil {
		return
	}
	select {
	case err := <-s.done:
		s.t.Errorf("API exited unexpectedly: %v", err)
	default:
		_ = s.process.Process.Kill()
		<-s.done
	}
	s.process = nil
}

func (s *stack) sql(query string, args ...any) {
	s.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := s.pool.Exec(ctx, query, args...)
	expect.NoError(s.t, "fixture SQL", err)
}

func (s *stack) count(query string, args ...any) int {
	s.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var count int
	expect.NoError(s.t, "query persisted count", s.pool.QueryRow(ctx, query, args...).Scan(&count))
	return count
}

type browser struct {
	id, email, csrf string
	cookie          *http.Cookie
}
type reply struct {
	t       *testing.T
	status  int
	header  http.Header
	cookies []*http.Cookie
	body    map[string]any
}

func encode(value any) string {
	body, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(body)
}

func (s *stack) request(b *browser, method, path string, body any, status int) reply {
	s.t.Helper()
	var text string
	if body != nil {
		text = encode(body)
	}
	r, err := s.raw(b, method, path, text, nil)
	expect.NoError(s.t, "HTTP request", err)
	expect.Equal(s.t, method+" "+path+" status", r.status, status)
	return r
}

func (s *stack) raw(b *browser, method, path, body string, headers map[string]string) (reply, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, s.url+path, strings.NewReader(body))
	if err != nil {
		return reply{}, err
	}
	req.Header.Set("Origin", origin)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if b != nil {
		req.AddCookie(b.cookie)
		req.Header.Set("X-CSRF-Token", b.csrf)
	}
	for name, value := range headers {
		if value == "" {
			req.Header.Del(name)
		} else {
			req.Header.Set(name, value)
		}
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return reply{}, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return reply{}, err
	}
	r := reply{t: s.t, status: response.StatusCode, header: response.Header, cookies: response.Cookies()}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &r.body); err != nil {
			return reply{}, fmt.Errorf("invalid JSON response: %w", err)
		}
	}
	return r, nil
}

func (r reply) value(path ...string) any {
	r.t.Helper()
	var current any = r.body
	for _, key := range path {
		object, ok := current.(map[string]any)
		if !ok {
			r.t.Fatalf("response has no object at %v", path)
		}
		current, ok = object[key]
		if !ok {
			r.t.Fatalf("response is missing field %v", path)
		}
	}
	return current
}

func (r reply) str(path ...string) string {
	r.t.Helper()
	value, ok := r.value(path...).(string)
	if !ok {
		r.t.Fatalf("%v is not a string", path)
	}
	return value
}
func (r reply) num(path ...string) float64 {
	r.t.Helper()
	value, ok := r.value(path...).(float64)
	if !ok {
		r.t.Fatalf("%v is not a number", path)
	}
	return value
}
func (r reply) flag(path ...string) bool {
	r.t.Helper()
	value, ok := r.value(path...).(bool)
	if !ok {
		r.t.Fatalf("%v is not a boolean", path)
	}
	return value
}
func (r reply) items(path ...string) []reply {
	r.t.Helper()
	values, ok := r.value(path...).([]any)
	if !ok {
		r.t.Fatalf("%v is not an array", path)
	}
	var items []reply
	for _, value := range values {
		object, ok := value.(map[string]any)
		if !ok {
			r.t.Fatal("array item is not an object")
		}
		items = append(items, reply{t: r.t, body: object})
	}
	return items
}
func (r reply) problem(code string) {
	r.t.Helper()
	expect.Equal(r.t, "error code", r.str("error", "code"), code)
	expect.Equal(r.t, "error message exists", r.str("error", "message") != "", true)
}
func (r reply) invalid(field string) {
	r.t.Helper()
	r.problem("validation_failed")
	expect.Equal(r.t, "field explanation exists", r.str("error", "fields", field) != "", true)
}

func (s *stack) login(email, pass string) *browser {
	s.t.Helper()
	r := s.request(nil, "POST", "/auth/login", map[string]any{"email": email, "password": pass, "rememberMe": true}, 200)
	b := &browser{id: r.str("user", "id"), email: email, csrf: r.str("csrfToken")}
	for _, cookie := range r.cookies {
		if cookie.Name == "skillswap_session" {
			b.cookie = cookie
		}
	}
	expect.Equal(s.t, "session cookie exists", b.cookie != nil, true)
	s.secrets = append(s.secrets, pass, b.csrf, b.cookie.Value)
	return b
}

func (s *stack) student(name, teach, learn string) *browser {
	s.t.Helper()
	email := name + "@students.example.test"
	s.request(nil, "POST", "/auth/register", map[string]any{"email": email, "password": password, "firstName": "Олена", "lastName": "Тестова"}, 201)
	token := s.mail.token(s.t, email)
	s.secrets = append(s.secrets, token)
	s.request(nil, "POST", "/auth/verify-email", map[string]string{"token": token}, 200)
	b := s.login(email, password)
	s.request(b, "PATCH", "/me/profile", map[string]any{"city": "Київ", "formats": []string{"ONLINE", "OFFLINE"}}, 200)
	for list, skill := range map[string]string{"teaching": teach, "learning": learn} {
		if skill != "" {
			s.request(b, "POST", "/me/"+list+"-skills", map[string]string{"skillId": skill, "level": "ADVANCED"}, 201)
		}
	}
	return b
}

// The SMTP sink accepts real net/smtp delivery and never forwards mail externally.
type smtpInbox struct {
	sync.Mutex
	address  string
	messages map[string]string
}

func newInbox(t *testing.T) *smtpInbox {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	expect.NoError(t, "listen for SMTP", err)
	inbox := &smtpInbox{address: listener.Addr().String(), messages: map[string]string{}}
	var workers sync.WaitGroup
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			workers.Add(1)
			go func() { defer workers.Done(); inbox.receive(conn) }()
		}
	}()
	t.Cleanup(func() { listener.Close(); <-finished; workers.Wait() })
	return inbox
}

func (inbox *smtpInbox) receive(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	wire := textproto.NewConn(conn)
	_ = wire.PrintfLine("220 localhost ESMTP test inbox")
	var recipient string
	for {
		line, err := wire.ReadLine()
		if err != nil {
			return
		}
		switch {
		case strings.HasPrefix(line, "EHLO"), strings.HasPrefix(line, "HELO"), strings.HasPrefix(line, "MAIL FROM:"), line == "RSET":
			_ = wire.PrintfLine("250 OK")
		case strings.HasPrefix(line, "RCPT TO:"):
			recipient = strings.Trim(strings.TrimPrefix(line, "RCPT TO:"), "<>")
			_ = wire.PrintfLine("250 OK")
		case line == "DATA":
			_ = wire.PrintfLine("354 Send message")
			body, err := wire.ReadDotBytes()
			if err != nil {
				return
			}
			inbox.Lock()
			inbox.messages[recipient] = string(body)
			inbox.Unlock()
			_ = wire.PrintfLine("250 queued")
		case line == "QUIT":
			_ = wire.PrintfLine("221 bye")
			return
		default:
			_ = wire.PrintfLine("500 unsupported")
		}
	}
}

var tokenPattern = regexp.MustCompile(`token=([A-Za-z0-9_-]{43})`)

func (inbox *smtpInbox) token(t *testing.T, email string) string {
	t.Helper()
	inbox.Lock()
	body := inbox.messages[email]
	inbox.Unlock()
	message, err := mail.ReadMessage(strings.NewReader(body))
	expect.NoError(t, "parse SMTP message", err)
	decoded, err := io.ReadAll(quotedprintable.NewReader(message.Body))
	expect.NoError(t, "decode SMTP message", err)
	match := tokenPattern.FindStringSubmatch(string(decoded))
	if match == nil {
		t.Fatal("SMTP did not receive a one-time link for the student")
	}
	return match[1]
}
