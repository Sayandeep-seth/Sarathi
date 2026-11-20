package driver

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperr "sarathi/internal/common/errors"
)

// Driver is the registered driver + vehicle profile.
type Driver struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Phone        string   `json:"phone"`
	VehicleID    string   `json:"vehicle_id"`
	VehicleModel string   `json:"vehicle_model"`
	Seats        int      `json:"seats"`
	Classes      []string `json:"classes"` // cab, seat5/seat7, shared, premium, vintage, tourist, testdrive
	Rating       float64  `json:"rating"`
	DealerID     string   `json:"dealer_id,omitempty"` // test-drive cars belong to a dealer
}

type Repository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

func (r *Repository) Get(ctx context.Context, id string) (*Driver, error) {
	var d Driver
	err := r.db.QueryRow(ctx, `
		SELECT d.id, d.name, d.phone, v.id, v.model, v.seats, v.classes, d.rating, coalesce(v.dealer_id,'')
		FROM drivers d JOIN fleet_vehicles v ON v.id = d.vehicle_id WHERE d.id = $1 AND d.active`, id).
		Scan(&d.ID, &d.Name, &d.Phone, &d.VehicleID, &d.VehicleModel, &d.Seats, &d.Classes, &d.Rating, &d.DealerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("driver not found")
	}
	return &d, err
}

// AddEarning records a completed-trip payout.
func (r *Repository) AddEarning(ctx context.Context, driverID, bookingID string, amount float64) error {
	_, err := r.db.Exec(ctx, `INSERT INTO driver_earnings (driver_id, booking_id, amount) VALUES ($1,$2,$3)`, driverID, bookingID, amount)
	return err
}
