// Package database opens PostgreSQL connections and applies SQL migrations.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
)

// Open creates a pooled handle. The pool is lazy: use Ping/Check to verify connectivity.
func Open(url string) (*sql.DB, error) {
	db, err := sql.Open("pgx", url)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	return db, nil
}

// Checker implements httpapi.Checker for PostgreSQL.
type Checker struct{ DB *sql.DB }

// Name implements httpapi.Checker.
func (Checker) Name() string { return "postgres" }

// Check pings the database within the caller's deadline.
func (c Checker) Check(ctx context.Context) error { return c.DB.PingContext(ctx) }

// migrationLockID is an arbitrary, fixed advisory-lock key so concurrent migrators serialize.
const migrationLockID = 7243001

// Migrate applies every pending "NNNN_name.sql" file from fsys in lexical order.
// Each file runs in its own transaction and is recorded in schema_migrations.
// Applied files are never re-run. Checksum verification of already-applied files is
// not implemented yet (hardening item): never edit an applied migration, add a new one.
func Migrate(ctx context.Context, db *sql.DB, fsys fs.FS, log *slog.Logger) (applied []string, err error) {
	// Use a dedicated connection so the advisory lock and the migrations share a session.
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Close()

	if _, err = conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, migrationLockID); err != nil {
		return nil, fmt.Errorf("advisory lock: %w", err)
	}
	defer func() {
		// Use a fresh context: ctx may already be cancelled when we unwind.
		uctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(uctx, `SELECT pg_advisory_unlock($1)`, migrationLockID)
	}()

	if _, err = conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    text PRIMARY KEY,
		applied_at timestamptz NOT NULL DEFAULT now()
	)`); err != nil {
		return nil, fmt.Errorf("create schema_migrations: %w", err)
	}

	done := map[string]bool{}
	rows, err := conn.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v string
		if err = rows.Scan(&v); err != nil {
			rows.Close()
			return nil, err
		}
		done[v] = true
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}

	entries, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return nil, err
	}
	sort.Strings(entries)

	for _, name := range entries {
		version := strings.TrimSuffix(name, ".sql")
		if done[version] {
			continue
		}
		body, rerr := fs.ReadFile(fsys, name)
		if rerr != nil {
			return applied, rerr
		}
		tx, terr := conn.BeginTx(ctx, nil)
		if terr != nil {
			return applied, terr
		}
		if _, eerr := tx.ExecContext(ctx, string(body)); eerr != nil {
			_ = tx.Rollback()
			return applied, fmt.Errorf("migration %s: %w", name, eerr)
		}
		if _, eerr := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version) VALUES($1)`, version); eerr != nil {
			_ = tx.Rollback()
			return applied, fmt.Errorf("record %s: %w", name, eerr)
		}
		if eerr := tx.Commit(); eerr != nil {
			return applied, fmt.Errorf("commit %s: %w", name, eerr)
		}
		log.Info("migration_applied", slog.String("migration", version))
		applied = append(applied, version)
	}
	return applied, nil
}
