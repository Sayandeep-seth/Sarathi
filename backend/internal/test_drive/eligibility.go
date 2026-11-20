package test_drive

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
	apperr "sarathi/internal/common/errors"
)

// Eligibility enforces "one test drive per customer per model per showroom".
//
//	sr:td:used:<showroom>:<model>  SET customer ids (SADD returns 0 for a repeat)
//
// Postgres (unique index) is the durable backstop if Redis is ever flushed.
type Eligibility struct {
	rdb *redis.Client
	db  *pgxpool.Pool
}

func NewEligibility(rdb *redis.Client, db *pgxpool.Pool) *Eligibility {
	return &Eligibility{rdb: rdb, db: db}
}

// Claim reserves the customer's single test drive of model at showroom.
func (e *Eligibility) Claim(ctx context.Context, showroomID, model, customerID string) error {
	added, err := e.rdb.SAdd(ctx, cache.KeyTDUsed(showroomID, model), customerID).Result()
	if err != nil {
		return err
	}
	if added == 0 {
		return apperr.Conflict("you have already test driven this model at this showroom")
	}
	var exists bool
	_ = e.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM test_drives WHERE showroom_id=$1 AND model=$2 AND customer_id=$3 AND status <> 'cancelled')`,
		showroomID, model, customerID).Scan(&exists)
	if exists {
		return apperr.Conflict("you have already test driven this model at this showroom")
	}
	return nil
}

// Unclaim gives the chance back (booking cancelled / payment failed).
func (e *Eligibility) Unclaim(ctx context.Context, showroomID, model, customerID string) error {
	return e.rdb.SRem(ctx, cache.KeyTDUsed(showroomID, model), customerID).Err()
}

// Eligible checks without claiming (to grey out the button in the app).
func (e *Eligibility) Eligible(ctx context.Context, showroomID, model, customerID string) (bool, error) {
	used, err := e.rdb.SIsMember(ctx, cache.KeyTDUsed(showroomID, model), customerID).Result()
	return !used, err
}
