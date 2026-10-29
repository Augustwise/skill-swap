package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"skillswap/backend/internal/expect"
)

func TestE2EStartupAndSchemaContract(t *testing.T) {
	s := newStack(t)
	before := tableCounts(t, s.pool)
	s.command(suite.db, nil, "up")
	s.command(suite.db, nil, "seed-test")
	after := tableCounts(t, s.pool)
	for table, count := range before {
		expect.Equal(t, "repeat setup preserves "+table, after[table], count)
	}
	for _, test := range []struct {
		settings    map[string]string
		explanation string
	}{
		{map[string]string{"API_ADDR": "0.0.0.0:0"}, "loopback IP"},
		{map[string]string{"API_ADDR": "192.0.2.1:8080"}, "loopback IP"},
		{map[string]string{"DATABASE_URL": "postgres://student@db.example.test/skillswap?sslmode=disable"}, "sslmode=verify-full"},
	} {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		cmd := exec.CommandContext(ctx, suite.api)
		cmd.Dir = suite.dir
		cmd.Env = s.env(test.settings)
		output, err := cmd.CombinedOutput()
		cancel()
		expect.Equal(t, "unsafe configuration rejected before serving", err != nil, true)
		expect.Equal(t, "configuration failure was not a timeout", ctx.Err() == context.DeadlineExceeded, false)
		expect.Equal(t, "configuration rejection explains the rule", strings.Contains(string(output), test.explanation), true)
	}
	s.stop()
	s.sql(`UPDATE goose_db_version SET is_applied = false WHERE version_id = 8`)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, suite.api)
	cmd.Dir = suite.dir
	cmd.Env = s.env(nil)
	output, err := cmd.CombinedOutput()
	expect.Equal(t, "incomplete schema prevents startup", err != nil, true)
	expect.Equal(t, "startup explains missing migration", strings.Contains(string(output), "missing: [8]"), true)
	s.sql(`UPDATE goose_db_version SET is_applied = true WHERE version_id = 8`)
	s.start()
	s.request(nil, "GET", "/ready", nil, 200)
}

func TestE2ERestartPersistence(t *testing.T) {
	if suite.adminURL == "" {
		t.Skip("enable the E2E environment")
	}
	if suite.pgData == "" {
		t.Skip("E2E_PG_BIN is required: only the cluster owned by this suite may be restarted")
	}
	s := newStack(t)
	a := s.student("restart-a", guitar, photoshop)
	b := s.student("restart-b", photoshop, guitar)
	requestID, ex := s.exchange(a, b)
	chat := s.request(a, "GET", "/exchanges/"+ex+"/conversation", nil, 200).str("conversation", "id")
	path := "/conversations/" + chat + "/messages"
	s.request(a, "POST", path, map[string]string{"body": "Повідомлення до перезапуску"}, 201)
	s.request(a, "PATCH", "/me/profile", map[string]string{"bio": "Профіль до перезапуску"}, 200)
	s.stop()
	expect.NoError(t, "restart owned PostgreSQL cluster", pgTool("pg_ctl", "-D", suite.pgData, "-l", filepath.Join(suite.dir, "postgres.log"), "-m", "fast", "-w", "restart"))
	s.pool.Reset()
	s.start()
	expect.Equal(t, "same session survives API and database restart", s.request(a, "GET", "/auth/me", nil, 200).str("user", "id"), a.id)
	expect.Equal(t, "profile survives restart", s.request(a, "GET", "/me/profile", nil, 200).str("profile", "bio"), "Профіль до перезапуску")
	expect.Equal(t, "request survives restart", s.request(b, "GET", "/exchange-requests/"+requestID, nil, 200).str("request", "status"), "ACCEPTED")
	expect.Equal(t, "message survives restart", s.request(b, "GET", path, nil, 200).items("items")[0].str("body"), "Повідомлення до перезапуску")
	s.request(b, "POST", path, map[string]string{"body": "Продовжуємо після перезапуску"}, 201)
	s.finish(a, b, ex)
	s.request(a, "POST", "/exchanges/"+ex+"/reviews", map[string]int{"rating": 5}, 201)
}

func TestE2EBackupRestore(t *testing.T) {
	if suite.adminURL == "" {
		t.Skip("enable the E2E environment")
	}
	for _, name := range []string{"pg_dump", "pg_restore"} {
		if err := pgTool(name, "--version"); err != nil {
			t.Fatalf("backup acceptance requires PostgreSQL client tools: %v", err)
		}
	}
	s := newStack(t)
	a := s.student("backup-a", guitar, photoshop)
	b := s.student("backup-b", photoshop, guitar)
	_, ex := s.exchange(a, b)
	chat := s.request(a, "GET", "/exchanges/"+ex+"/conversation", nil, 200).str("conversation", "id")
	path := "/conversations/" + chat + "/messages"
	s.request(a, "POST", path, map[string]string{"body": "Відновлена історія"}, 201)
	s.command(suite.db, nil, "seed-load")
	expect.Equal(t, "1000 backup test profiles", s.count(`SELECT count(*) FROM users WHERE email LIKE 'load.%@students.example.test'`), 1000)
	dir := t.TempDir()
	config := map[string]string{"BACKUP_DIR": dir}
	s.command(suite.db, config, "backup")
	files, err := filepath.Glob(filepath.Join(dir, "skillswap-*.dump"))
	expect.NoError(t, "find backup", err)
	expect.Equal(t, "one archive created", len(files), 1)
	archive, err := os.ReadFile(files[0])
	expect.NoError(t, "read archive", err)
	expect.Equal(t, "PostgreSQL custom archive", string(archive[:5]), "PGDMP")
	for day := 1; day <= 8; day++ {
		name := "skillswap-" + time.Date(2000, 1, day, 0, 0, 0, 0, time.UTC).Format("20060102-150405") + ".dump"
		expect.NoError(t, "create old archive", os.WriteFile(filepath.Join(dir, name), archive, 0600))
	}
	expect.NoError(t, "create unrelated file", os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("keep"), 0600))
	s.command(suite.db, config, "backup")
	files, err = filepath.Glob(filepath.Join(dir, "skillswap-*.dump"))
	expect.NoError(t, "find retained archives", err)
	expect.Equal(t, "seven archives retained", len(files), 7)
	_, err = os.Stat(filepath.Join(dir, "keep.txt"))
	expect.NoError(t, "unrelated file retained", err)
	_, err = os.Stat(filepath.Join(dir, "skillswap-20000101-000000.dump"))
	expect.Equal(t, "oldest archive removed", os.IsNotExist(err), true)
	sourceCounts := tableCounts(t, s.pool)
	target := createDatabase(t)
	s.stop()
	started := time.Now()
	output := s.command(suite.db, map[string]string{"RESTORE_DATABASE_URL": target}, "restore", files[len(files)-1])
	elapsed := time.Since(started)
	expect.Equal(t, "restore fits 15 minutes", elapsed <= 15*time.Minute, true)
	expect.Equal(t, "restore command verifies counts", strings.Contains(output, "All row counts match"), true)
	restored, err := pgxpool.New(context.Background(), target)
	expect.NoError(t, "open restored database", err)
	t.Cleanup(restored.Close)
	t.Cleanup(s.stop)
	restoredCounts := tableCounts(t, restored)
	expect.Equal(t, "same table count", len(restoredCounts), len(sourceCounts))
	for table, count := range sourceCounts {
		expect.Equal(t, "restored rows in "+table, restoredCounts[table], count)
	}
	s.databaseURL = target
	s.pool = restored
	s.start()
	expect.Equal(t, "restored session works", s.request(a, "GET", "/auth/me", nil, 200).str("user", "id"), a.id)
	expect.Equal(t, "restored conversation", s.request(b, "GET", path, nil, 200).items("items")[0].str("body"), "Відновлена історія")
	s.finish(a, b, ex)
	s.request(a, "POST", "/exchanges/"+ex+"/reviews", map[string]int{"rating": 5}, 201)
	t.Logf("1000 profiles restored and all table counts matched in %s; exchange completed through restored API", elapsed.Round(time.Millisecond))
}

func tableCounts(t *testing.T, pool *pgxpool.Pool) map[string]int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	rows, err := pool.Query(ctx, `SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AND table_type = 'BASE TABLE' ORDER BY table_name`)
	expect.NoError(t, "list tables", err)
	tables, err := pgx.CollectRows(rows, pgx.RowTo[string])
	expect.NoError(t, "collect table names", err)
	counts := map[string]int64{}
	for _, table := range tables {
		var count int64
		expect.NoError(t, "count "+table, pool.QueryRow(ctx, "SELECT count(*) FROM "+pgx.Identifier{"public", table}.Sanitize()).Scan(&count))
		counts[table] = count
	}
	return counts
}

func TestE2ELoad(t *testing.T) {
	if os.Getenv("E2E_LOAD") != "1" {
		t.Skip("set E2E_LOAD=1 for the full 10-minute, 50-user acceptance run")
	}
	s := newStack(t)
	s.command(suite.db, nil, "seed-load")
	expect.Equal(t, "1000 load profiles", s.count(`SELECT count(*) FROM users WHERE email LIKE 'load.%@students.example.test'`), 1000)
	t.Log("Starting the 10-minute load test with 50 concurrent users")
	output := s.command(suite.load, nil, "-base", s.url, "-users", "50", "-duration", "10m")
	t.Log(strings.TrimSpace(output))
	expect.Equal(t, "load thresholds passed", strings.Contains(output, "NFR-01 PASSED"), true)
}

func TestE2ESoak(t *testing.T) {
	if os.Getenv("E2E_SOAK") != "1" {
		t.Skip("set E2E_SOAK=1 and -timeout 75m for the full 60-minute demonstration")
	}
	s := newStack(t)
	a := s.student("soak-a", guitar, photoshop)
	b := s.student("soak-b", photoshop, guitar)
	_, ex := s.exchange(a, b)
	chat := s.request(a, "GET", "/exchanges/"+ex+"/conversation", nil, 200).str("conversation", "id")
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	deadline := time.NewTimer(time.Hour)
	defer deadline.Stop()
	iterations := 0
	for {
		select {
		case <-deadline.C:
			s.finish(a, b, ex)
			s.request(a, "POST", "/exchanges/"+ex+"/reviews", map[string]int{"rating": 5}, 201)
			t.Logf("60 minutes without API exits; %d polling cycles; exchange and review completed", iterations)
			return
		case <-ticker.C:
			s.request(a, "GET", "/students?q=Photoshop", nil, 200)
			s.request(b, "GET", "/me/matches", nil, 200)
			s.request(b, "GET", "/conversations/"+chat+"/messages", nil, 200)
			s.request(a, "GET", "/ready", nil, 200)
			if iterations%12 == 0 {
				s.request(a, "POST", "/conversations/"+chat+"/messages", map[string]string{"body": "Демонстрація триває"}, 201)
				t.Logf("Demonstration minute %d/60", iterations/12+1)
			}
			iterations++
		}
	}
}
