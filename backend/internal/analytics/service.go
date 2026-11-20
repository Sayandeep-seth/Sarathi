// Package analytics keeps real-time counters, unique counts and leaderboards in
// Redis (INCR, HyperLogLog, sorted sets) and flushes daily snapshots to Postgres.
package analytics

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
	"sarathi/internal/common/response"
	"sarathi/internal/common/utils"
)

// Metric names.
const (
	MetricRouteSearch   = "route_search"
	MetricBookings      = "bookings"
	MetricCancellations = "cancellations"
	MetricQRScans       = "qr_scans"
	MetricActiveUsers   = "active_users" // unique (HLL)
)

var counterMetrics = []string{MetricRouteSearch, MetricBookings, MetricCancellations, MetricQRScans}

type Service struct {
	rdb *redis.Client
	db  *pgxpool.Pool
	log *slog.Logger
}

func NewService(rdb *redis.Client, db *pgxpool.Pool, log *slog.Logger) *Service {
	return &Service{rdb: rdb, db: db, log: log}
}

// Count increments a daily counter.
func (s *Service) Count(ctx context.Context, metric string, n int64) {
	k := cache.KeyStatCounter(metric, utils.DateKey(time.Now()))
	pipe := s.rdb.Pipeline()
	pipe.IncrBy(ctx, k, n)
	pipe.Expire(ctx, k, cache.TTLStats)
	_, _ = pipe.Exec(ctx)
}

// Unique adds a member to a daily HyperLogLog (≈12 KB per day regardless of users).
func (s *Service) Unique(ctx context.Context, metric, member string) {
	k := cache.KeyStatUnique(metric, utils.DateKey(time.Now()))
	pipe := s.rdb.Pipeline()
	pipe.PFAdd(ctx, k, member)
	pipe.Expire(ctx, k, cache.TTLStats)
	_, _ = pipe.Exec(ctx)
}

// Summary returns all counters for a day.
func (s *Service) Summary(ctx context.Context, date string) (map[string]any, error) {
	pipe := s.rdb.Pipeline()
	counters := map[string]*redis.StringCmd{}
	for _, m := range counterMetrics {
		counters[m] = pipe.Get(ctx, cache.KeyStatCounter(m, date))
	}
	uniq := pipe.PFCount(ctx, cache.KeyStatUnique(MetricActiveUsers, date))
	rides := pipe.HGetAll(ctx, cache.KeyStatRides(date))
	revenue := pipe.HGetAll(ctx, cache.KeyStatRevenue(date))
	if _, err := pipe.Exec(ctx); err != nil && !cache.IsNil(err) {
		return nil, err
	}
	out := map[string]any{"date": date, "active_users": uniq.Val(), "rides": rides.Val(), "revenue": revenue.Val()}
	for m, c := range counters {
		n, _ := c.Int64()
		out[m] = n
	}
	return out, nil
}

// FlushDay persists a day's Redis numbers into Postgres (worker, after midnight).
func (s *Service) FlushDay(ctx context.Context, date string) error {
	sum, err := s.Summary(ctx, date)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(ctx, `
		INSERT INTO daily_stats (day, data) VALUES (to_date($1,'YYYYMMDD'), $2)
		ON CONFLICT (day) DO UPDATE SET data = EXCLUDED.data`, date, sum)
	return err
}

// Register exposes admin dashboard endpoints.
func (s *Service) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/admin/analytics/summary", func(w http.ResponseWriter, r *http.Request) {
		date := r.URL.Query().Get("date")
		if date == "" {
			date = utils.DateKey(time.Now())
		}
		out, err := s.Summary(r.Context(), date)
		if err != nil {
			response.Error(w, err)
			return
		}
		response.OK(w, out)
	})
	mux.HandleFunc("GET /api/v1/admin/analytics/heatmap", func(w http.ResponseWriter, r *http.Request) {
		out, err := s.Heatmap(r.Context(), utils.DateKey(time.Now()), 100)
		if err != nil {
			response.Error(w, err)
			return
		}
		response.OK(w, out)
	})
	mux.HandleFunc("GET /api/v1/admin/analytics/stations/top", func(w http.ResponseWriter, r *http.Request) {
		out, err := s.TopStationsWeek(r.Context(), "petrol", 20)
		if err != nil {
			response.Error(w, err)
			return
		}
		response.OK(w, out)
	})
	mux.HandleFunc("GET /api/v1/admin/analytics/drivers/top", func(w http.ResponseWriter, r *http.Request) {
		out, err := s.TopEarners(r.Context(), utils.MonthKey(time.Now()), 20)
		if err != nil {
			response.Error(w, err)
			return
		}
		response.OK(w, out)
	})
}
