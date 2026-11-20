package gps

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// LocationRepository persists GPS history in Postgres (fed from a Redis stream
// in batches, so the hot path never waits on Postgres).
type LocationRepository struct{ db *pgxpool.Pool }

func NewLocationRepository(db *pgxpool.Pool) *LocationRepository { return &LocationRepository{db: db} }

// InsertBatch writes many pings with one statement (UNNEST of arrays).
func (r *LocationRepository) InsertBatch(ctx context.Context, pings []Ping) error {
	if len(pings) == 0 {
		return nil
	}
	n := len(pings)
	vehicle, trip := make([]string, n), make([]string, n)
	lat, lng, speed := make([]float64, n), make([]float64, n), make([]float64, n)
	at := make([]time.Time, n)
	for i, p := range pings {
		vehicle[i], trip[i] = p.VehicleID, p.TripID
		lat[i], lng[i], speed[i], at[i] = p.Location.Lat, p.Location.Lng, p.SpeedKmh, p.At
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO vehicle_locations (vehicle_id, trip_id, lat, lng, speed_kmh, recorded_at)
		SELECT * FROM UNNEST($1::text[], $2::text[], $3::float8[], $4::float8[], $5::float8[], $6::timestamptz[])`,
		vehicle, trip, lat, lng, speed, at)
	return err
}
