package luxury_bus

import (
	"context"
	"time"

	"sarathi/internal/cache"
	"sarathi/internal/common/utils"
)

// OwnerFoodCommission is the fixed share of every food order paid to the bus owner
// (passive income for letting passengers order on board).
const OwnerFoodCommission = 0.10

// CreditOwner adds the owner's commission for a paid food order.
//
//	sr:lbus:commission:<owner>:<yyyymm>  HASH food, food_count, restaurant_payable
func (s *Store) CreditOwner(ctx context.Context, ownerID string, orderTotal float64) error {
	k := cache.KeyLuxCommission(ownerID, utils.MonthKey(time.Now()))
	pipe := s.rdb.TxPipeline()
	pipe.HIncrByFloat(ctx, k, "food", utils.Round(orderTotal*OwnerFoodCommission, 2))
	pipe.HIncrBy(ctx, k, "food_count", 1)
	pipe.HIncrByFloat(ctx, k, "restaurant_payable", utils.Round(orderTotal*(1-OwnerFoodCommission), 2))
	pipe.Expire(ctx, k, 400*24*time.Hour)
	_, err := pipe.Exec(ctx)
	return err
}

// OwnerEarnings returns a month summary for the bus owner dashboard.
func (s *Store) OwnerEarnings(ctx context.Context, ownerID, month string) (map[string]string, error) {
	return s.rdb.HGetAll(ctx, cache.KeyLuxCommission(ownerID, month)).Result()
}
