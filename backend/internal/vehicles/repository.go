package vehicles

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperr "sarathi/internal/common/errors"
)

// Repository stores car specs in Postgres (source of truth).
type Repository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

func (r *Repository) Get(ctx context.Context, model string) (*Spec, error) {
	var raw []byte
	err := r.db.QueryRow(ctx, `SELECT data FROM car_specs WHERE model = $1`, model).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("car model not found: " + model)
	}
	if err != nil {
		return nil, err
	}
	var s Spec
	return &s, json.Unmarshal(raw, &s)
}

func (r *Repository) Upsert(ctx context.Context, s *Spec) error {
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, `
		INSERT INTO car_specs (model, display_name, data, updated_at) VALUES ($1, $2, $3, now())
		ON CONFLICT (model) DO UPDATE SET display_name = EXCLUDED.display_name, data = EXCLUDED.data, updated_at = now()`,
		s.Model, s.DisplayName, raw)
	return err
}

// AllModels lists every known model (used to warm the autocomplete index).
func (r *Repository) AllModels(ctx context.Context) ([]string, error) {
	rows, err := r.db.Query(ctx, `SELECT model FROM car_specs`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var m string
		if err := rows.Scan(&m); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
