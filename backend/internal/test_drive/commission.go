package test_drive

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
	"sarathi/internal/common/utils"
)

const (
	TestDriveFee   = 199.0 // paid by the customer when booking a test drive
	DealerFeeShare = 0.90  // dealer keeps 90 % of the test-drive fee
	DealerCabShare = 0.75  // dealer keeps 75 % of every cab fare earned by a demo car
)

// Commission credits dealer income.
//
//	sr:td:earn:<dealer>:<yyyymm>  HASH test_drive, test_drive_count, cab, cab_count
//	sr:stream:td:ledger           STREAM every credit (for payouts / audit)
type Commission struct{ rdb *redis.Client }

func NewCommission(rdb *redis.Client) *Commission { return &Commission{rdb: rdb} }

func (c *Commission) credit(ctx context.Context, dealerID, kind, ref string, amount float64) error {
	k := cache.KeyTDEarnings(dealerID, utils.MonthKey(time.Now()))
	pipe := c.rdb.TxPipeline()
	pipe.HIncrByFloat(ctx, k, kind, amount)
	pipe.HIncrBy(ctx, k, kind+"_count", 1)
	pipe.Expire(ctx, k, 400*24*time.Hour)
	pipe.XAdd(ctx, &redis.XAddArgs{Stream: cache.StreamTDLedger, MaxLen: 1_000_000, Approx: true,
		Values: map[string]any{"dealer": dealerID, "kind": kind, "ref": ref, "amount": amount, "at": time.Now().Unix()}})
	_, err := pipe.Exec(ctx)
	return err
}

// TestDrive credits the dealer's share of a paid test drive.
func (c *Commission) TestDrive(ctx context.Context, dealerID, bookingID string) error {
	return c.credit(ctx, dealerID, "test_drive", bookingID, TestDriveFee*DealerFeeShare)
}

// Cab credits the dealer's share of a completed cab ride.
func (c *Commission) Cab(ctx context.Context, dealerID, bookingID string, fare float64) error {
	return c.credit(ctx, dealerID, "cab", bookingID, utils.Round(fare*DealerCabShare, 2))
}

// Earnings returns a dealer's month summary.
func (c *Commission) Earnings(ctx context.Context, dealerID, month string) (map[string]string, error) {
	return c.rdb.HGetAll(ctx, cache.KeyTDEarnings(dealerID, month)).Result()
}
