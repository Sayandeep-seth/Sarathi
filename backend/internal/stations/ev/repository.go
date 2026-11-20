// Package ev finds EV chargers along a journey based on the battery left.
package ev

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"sarathi/internal/common/utils"
)

// Charger is an EV charging station.
type Charger struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Operator   string      `json:"operator"`
	Location   utils.Point `json:"location"`
	Connectors []string    `json:"connectors"` // CCS2, Type2, CHAdeMO, GB/T, Bharat DC-001
	PowerKW    float64     `json:"power_kw"`
	TotalPorts int         `json:"total_ports"`

	FreePorts     int     `json:"free_ports"` // live, from operator feed (Redis)
	DistKm        float64 `json:"distance_km"`
	AlongRouteKm  float64 `json:"along_route_km"`
	Reachable     bool    `json:"reachable"`
	Compatible    bool    `json:"compatible"`
	ChargeMinutes float64 `json:"charge_to_80_min,omitempty"`
}

type Repository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

func (r *Repository) All(ctx context.Context) ([]Charger, error) {
	rows, err := r.db.Query(ctx, `SELECT id, name, operator, lat, lng, connectors, power_kw, total_ports FROM ev_stations WHERE active`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Charger
	for rows.Next() {
		var c Charger
		if err := rows.Scan(&c.ID, &c.Name, &c.Operator, &c.Location.Lat, &c.Location.Lng, &c.Connectors, &c.PowerKW, &c.TotalPorts); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repository) Upsert(ctx context.Context, c Charger) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO ev_stations (id, name, operator, lat, lng, connectors, power_kw, total_ports, active)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,true)
		ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, operator=EXCLUDED.operator, lat=EXCLUDED.lat,
		  lng=EXCLUDED.lng, connectors=EXCLUDED.connectors, power_kw=EXCLUDED.power_kw, total_ports=EXCLUDED.total_ports`,
		c.ID, c.Name, c.Operator, c.Location.Lat, c.Location.Lng, c.Connectors, c.PowerKW, c.TotalPorts)
	return err
}
