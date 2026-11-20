package map_service

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository reads road attributes (pre-ingested from OSM + field surveys) and
// stores route-search history.
type Repository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

// GetRoads loads attributes for the given road ids.
func (r *Repository) GetRoads(ctx context.Context, ids []string) (map[string]RoadAttr, error) {
	out := make(map[string]RoadAttr, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, name, road_class, width_m, one_way, lanes, max_height_m
		FROM roads WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var a RoadAttr
		if err := rows.Scan(&a.ID, &a.Name, &a.Class, &a.WidthM, &a.OneWay, &a.Lanes, &a.MaxHeightM); err != nil {
			return nil, err
		}
		a.Source = "measured"
		out[a.ID] = a
	}
	return out, rows.Err()
}

// UpsertRoad saves a road (ingestion / crowd-sourced width correction).
func (r *Repository) UpsertRoad(ctx context.Context, a RoadAttr) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO roads (id, name, road_class, width_m, one_way, lanes, max_height_m)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, road_class=EXCLUDED.road_class,
		  width_m=EXCLUDED.width_m, one_way=EXCLUDED.one_way, lanes=EXCLUDED.lanes, max_height_m=EXCLUDED.max_height_m`,
		a.ID, a.Name, a.Class, a.WidthM, a.OneWay, a.Lanes, a.MaxHeightM)
	return err
}

// SaveSearch records a route search for analytics/AI training.
func (r *Repository) SaveSearch(ctx context.Context, userID string, req RouteRequest, resp *RouteResponse) error {
	raw, _ := json.Marshal(resp.Routes)
	_, err := r.db.Exec(ctx, `
		INSERT INTO route_searches (user_id, car_model, origin_lat, origin_lng, dest_lat, dest_lng, routes)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		userID, resp.Car.Model, req.Origin.Lat, req.Origin.Lng, req.Destination.Lat, req.Destination.Lng, raw)
	return err
}
