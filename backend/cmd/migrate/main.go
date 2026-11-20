// Command migrate applies SQL files from migrations/ in order. A Redis lock makes
// sure only one deploy job migrates at a time.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"sarathi/app/bootstrap"
	"sarathi/app/config"
	"sarathi/internal/cache"
	"sarathi/internal/common/logger"
)

func main() {
	ctx := context.Background()
	cfg := config.Load()
	log := logger.New(cfg.Env)

	db, err := bootstrap.NewPostgres(ctx, cfg.Postgres)
	must(err)
	defer db.Close()
	rdb, err := bootstrap.NewRedis(ctx, cfg.Redis)
	must(err)
	defer rdb.Close()

	lk, err := cache.NewLocker(rdb).AcquireWait(ctx, "migrate", 10*time.Minute, 2*time.Minute)
	must(err)
	defer lk.Release(ctx)

	_, err = db.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`)
	must(err)

	files, err := filepath.Glob(filepath.Join(cfg.Postgres.MigrationsDir, "*.sql"))
	must(err)
	sort.Strings(files)
	for _, f := range files {
		version := filepath.Base(f)
		var done bool
		must(db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version=$1)`, version).Scan(&done))
		if done {
			continue
		}
		sql, err := os.ReadFile(f)
		must(err)
		tx, err := db.Begin(ctx)
		must(err)
		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			_ = tx.Rollback(ctx)
			must(fmt.Errorf("%s: %w", version, err))
		}
		_, err = tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, version)
		must(err)
		must(tx.Commit(ctx))
		log.Info("migration applied", "version", version)
	}
	log.Info("migrations up to date", "count", len(files))
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}
