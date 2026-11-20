package booking

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperr "sarathi/internal/common/errors"
)

// Repository persists bookings (Postgres is the source of truth; Redis the hot copy).
type Repository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

func (r *Repository) Insert(ctx context.Context, b *Booking) error {
	data, _ := json.Marshal(b)
	_, err := r.db.Exec(ctx, `
		INSERT INTO bookings (id, user_id, type, state, class, fare_total, scheduled_at, data, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		b.ID, b.UserID, b.Type, b.State, b.Class, b.Fare.Total, b.ScheduledAt, data, b.CreatedAt)
	return err
}

func (r *Repository) Update(ctx context.Context, b *Booking) error {
	data, _ := json.Marshal(b)
	_, err := r.db.Exec(ctx, `UPDATE bookings SET state=$2, driver_id=nullif($3,''), fare_total=$4, data=$5, updated_at=now() WHERE id=$1`,
		b.ID, b.State, b.DriverID, b.Fare.Total, data)
	return err
}

func (r *Repository) Get(ctx context.Context, id string) (*Booking, error) {
	var raw []byte
	err := r.db.QueryRow(ctx, `SELECT data FROM bookings WHERE id=$1`, id).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("booking not found")
	}
	if err != nil {
		return nil, err
	}
	var b Booking
	return &b, json.Unmarshal(raw, &b)
}
