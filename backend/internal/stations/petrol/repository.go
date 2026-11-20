// Package petrol finds petrol/diesel/CNG stations along a journey and rates them
// from GPS crowd data (how many distinct cars are seen at each station).
package petrol

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperr "sarathi/internal/common/errors"
	"sarathi/internal/common/utils"
)

// Station is a fuel station.
type Station struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Brand    string      `json:"brand"`
	Address  string      `json:"address"`
	Location utils.Point `json:"location"`
	Fuels    []string    `json:"fuels"`
	Open24x7 bool        `json:"open_24x7"`

	// live / derived values (from Redis)
	Rating       float64 `json:"rating"`         // 1..5 crowd rating
	WeeklyVisits int64   `json:"weekly_visits"`  // distinct cars seen in 7 days
	CarsNow      int64   `json:"cars_now"`       // cars at the station in the last 15 min
	DistKm       float64 `json:"distance_km"`    // from the query point
	AlongRouteKm float64 `json:"along_route_km"` // for route searches
	Score        float64 `json:"score"`
	Reachable    bool    `json:"reachable"`
}

type Repository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

const cols = `id, name, brand, address, lat, lng, fuels, open_24x7`

func scan(row pgx.Row) (Station, error) {
	var s Station
	err := row.Scan(&s.ID, &s.Name, &s.Brand, &s.Address, &s.Location.Lat, &s.Location.Lng, &s.Fuels, &s.Open24x7)
	return s, err
}

func (r *Repository) All(ctx context.Context) ([]Station, error) {
	rows, err := r.db.Query(ctx, `SELECT `+cols+` FROM petrol_stations WHERE active`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Station
	for rows.Next() {
		s, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *Repository) Get(ctx context.Context, id string) (Station, error) {
	s, err := scan(r.db.QueryRow(ctx, `SELECT `+cols+` FROM petrol_stations WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return s, apperr.NotFound("station not found")
	}
	return s, err
}

func (r *Repository) Upsert(ctx context.Context, s Station) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO petrol_stations (id, name, brand, address, lat, lng, fuels, open_24x7, active)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,true)
		ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, brand=EXCLUDED.brand, address=EXCLUDED.address,
		  lat=EXCLUDED.lat, lng=EXCLUDED.lng, fuels=EXCLUDED.fuels, open_24x7=EXCLUDED.open_24x7`,
		s.ID, s.Name, s.Brand, s.Address, s.Location.Lat, s.Location.Lng, s.Fuels, s.Open24x7)
	return err
}

// SaveDailyRating persists the computed rating history.
func (r *Repository) SaveDailyRating(ctx context.Context, id, date string, weekly int64, rating float64) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO station_ratings (station_id, day, weekly_visits, rating) VALUES ($1, to_date($2,'YYYYMMDD'), $3, $4)
		ON CONFLICT (station_id, day) DO UPDATE SET weekly_visits = EXCLUDED.weekly_visits, rating = EXCLUDED.rating`,
		id, date, weekly, rating)
	return err
}
