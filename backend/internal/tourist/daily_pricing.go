package tourist

import (
	"context"

	"sarathi/internal/cache"
	"sarathi/internal/pricing"
)

// default per-day tariffs (INR) by class
var defaultRates = map[string]pricing.DailyRate{
	"sedan":           {Class: "sedan", PerDay: 2800, KmPerDay: 250, ExtraKm: 12, DriverAllowance: 400, NightCharge: 300},
	"suv":             {Class: "suv", PerDay: 3800, KmPerDay: 250, ExtraKm: 16, DriverAllowance: 400, NightCharge: 300},
	"tempo_traveller": {Class: "tempo_traveller", PerDay: 5500, KmPerDay: 250, ExtraKm: 24, DriverAllowance: 500, NightCharge: 400},
}

// Rates returns the class tariff from Redis (admin editable) or the default.
func (c *Cache) Rates(ctx context.Context, class string) pricing.DailyRate {
	m, err := c.rdb.HGetAll(ctx, cache.KeyTouristRates(class)).Result()
	if err == nil && len(m) > 0 {
		return pricing.DailyRate{Class: class, PerDay: cache.HGetFloat(m, "per_day"), KmPerDay: cache.HGetFloat(m, "km_per_day"),
			ExtraKm: cache.HGetFloat(m, "extra_km"), DriverAllowance: cache.HGetFloat(m, "driver_allowance"), NightCharge: cache.HGetFloat(m, "night_charge")}
	}
	return defaultRates[class]
}

func (c *Cache) SetRates(ctx context.Context, r pricing.DailyRate) error {
	return c.rdb.HSet(ctx, cache.KeyTouristRates(r.Class), "per_day", r.PerDay, "km_per_day", r.KmPerDay,
		"extra_km", r.ExtraKm, "driver_allowance", r.DriverAllowance, "night_charge", r.NightCharge).Err()
}

// QuoteDaily = per day × days + extra km + driver allowance + night halts.
func (c *Cache) QuoteDaily(ctx context.Context, class string, days int, km float64) pricing.Breakdown {
	return pricing.DailyFare(c.Rates(ctx, class), days, km, max(days-1, 0))
}
