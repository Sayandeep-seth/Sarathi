package self_drive

import (
	"context"
	"time"

	"sarathi/internal/cache"
	apperr "sarathi/internal/common/errors"
	"sarathi/internal/payments"
)

// Deposit tracks the refundable security deposit collected with the rent.
//
//	sr:selfdrive:deposit:<booking>  HASH amount,status(held|released),deduction,reason,at
type Deposit struct {
	cache    *Cache
	payments *payments.Service
}

func NewDeposit(c *Cache, p *payments.Service) *Deposit { return &Deposit{cache: c, payments: p} }

// Hold records the deposit once the payment (rent + deposit) succeeded.
func (d *Deposit) Hold(ctx context.Context, bookingID string, amount float64) error {
	return d.cache.rdb.HSet(ctx, cache.KeySelfDriveDeposit(bookingID),
		"amount", amount, "status", "held", "at", time.Now().Unix()).Err()
}

// Release refunds the deposit minus deductions (damage, fuel, extra km, late return).
// HSETNX on "released_at" guarantees a deposit is released only once.
func (d *Deposit) Release(ctx context.Context, bookingID string, deduction float64, reason string) (float64, error) {
	k := cache.KeySelfDriveDeposit(bookingID)
	m, err := d.cache.rdb.HGetAll(ctx, k).Result()
	if err != nil {
		return 0, err
	}
	if m["status"] != "held" {
		return 0, apperr.Conflict("no held deposit for this booking")
	}
	first, err := d.cache.rdb.HSetNX(ctx, k, "released_at", time.Now().Unix()).Result()
	if err != nil || !first {
		return 0, apperr.Conflict("deposit already being released")
	}
	amount := cache.HGetFloat(m, "amount")
	refund := amount - deduction
	if refund < 0 {
		refund = 0
	}
	if refund > 0 {
		if _, err := d.payments.RefundByRef(ctx, "booking", bookingID, refund, "deposit release: "+reason); err != nil {
			d.cache.rdb.HDel(ctx, k, "released_at")
			return 0, err
		}
	}
	d.cache.rdb.HSet(ctx, k, "status", "released", "deduction", deduction, "reason", reason)
	return refund, nil
}
