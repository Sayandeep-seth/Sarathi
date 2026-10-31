package database

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"
	"testing/fstest"
	"time"
)

// These tests need a real PostgreSQL. Set SARATHI_TEST_DATABASE_URL to a throwaway
// database; they are skipped otherwise (and reported as skipped, not passed).
func TestMigrateAppliesOnceAndIsIdempotent(t *testing.T) {
	url := os.Getenv("SARATHI_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("SARATHI_TEST_DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := Open(url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	_, _ = db.ExecContext(ctx, `DROP TABLE IF EXISTS schema_migrations, mig_probe`)

	fsys := fstest.MapFS{
		"0001_a.sql": {Data: []byte(`CREATE TABLE mig_probe (id int PRIMARY KEY);`)},
		"0002_b.sql": {Data: []byte(`INSERT INTO mig_probe VALUES (1);`)},
		"notes.txt":  {Data: []byte(`ignored`)},
	}
	applied, err := Migrate(ctx, db, fsys, log)
	if err != nil || len(applied) != 2 {
		t.Fatalf("first run: applied=%v err=%v", applied, err)
	}
	applied, err = Migrate(ctx, db, fsys, log)
	if err != nil || len(applied) != 0 {
		t.Fatalf("second run must be a no-op: applied=%v err=%v", applied, err)
	}
	var n int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM mig_probe`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("count=%d err=%v", n, err)
	}
	_, _ = db.ExecContext(ctx, `DROP TABLE schema_migrations, mig_probe`)
}

func TestMigrateRollsBackFailedFile(t *testing.T) {
	url := os.Getenv("SARATHI_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("SARATHI_TEST_DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := Open(url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	_, _ = db.ExecContext(ctx, `DROP TABLE IF EXISTS schema_migrations, mig_half`)

	fsys := fstest.MapFS{
		"0001_bad.sql": {Data: []byte(`CREATE TABLE mig_half (id int); SELECT * FROM does_not_exist;`)},
	}
	if _, err := Migrate(ctx, db, fsys, log); err == nil {
		t.Fatal("expected failure")
	}
	var exists bool
	_ = db.QueryRowContext(ctx, `SELECT to_regclass('mig_half') IS NOT NULL`).Scan(&exists)
	if exists {
		t.Fatal("failed migration must roll back partial DDL")
	}
	var recorded int
	_ = db.QueryRowContext(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&recorded)
	if recorded != 0 {
		t.Fatal("failed migration must not be recorded")
	}
	_, _ = db.ExecContext(ctx, `DROP TABLE schema_migrations`)
}

func TestConcurrentMigratorsSerialize(t *testing.T) {
	url := os.Getenv("SARATHI_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("SARATHI_TEST_DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := Open(url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	_, _ = db.ExecContext(ctx, `DROP TABLE IF EXISTS schema_migrations, mig_conc`)

	fsys := fstest.MapFS{"0001_c.sql": {Data: []byte(`CREATE TABLE mig_conc (id int);`)}}
	var wg sync.WaitGroup
	var mu sync.Mutex
	total := 0
	errs := 0
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a, err := Migrate(ctx, db, fsys, log)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs++
			}
			total += len(a)
		}()
	}
	wg.Wait()
	if errs != 0 || total != 1 {
		t.Fatalf("exactly one migrator should apply the file: applied=%d errs=%d", total, errs)
	}
	_, _ = db.ExecContext(ctx, `DROP TABLE schema_migrations, mig_conc`)
}
