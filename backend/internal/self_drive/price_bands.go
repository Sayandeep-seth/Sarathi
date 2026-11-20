// Package self_drive rents cars the passenger drives himself. Cars are grouped
// into price bands by ex-showroom price: every car in a band (e.g. ₹50L–60L)
// has the same daily fare and the same refundable deposit.
package self_drive

import (
	"context"
	"sort"

	"sarathi/internal/cache"
	apperr "sarathi/internal/common/errors"
	"sarathi/internal/pricing"
)

const lakh = 100_000.0

// DefaultBands (INR). Admins can override via PUT /api/v1/self-drive/bands.
var DefaultBands = []pricing.Band{
	{ID: "b0_10L", MinPrice: 0, MaxPrice: 10 * lakh, DailyRate: 1800, Deposit: 5000, KmPerDay: 250, ExtraKm: 8, Insurance: 150},
	{ID: "b10_20L", MinPrice: 10 * lakh, MaxPrice: 20 * lakh, DailyRate: 2800, Deposit: 10000, KmPerDay: 250, ExtraKm: 10, Insurance: 250},
	{ID: "b20_35L", MinPrice: 20 * lakh, MaxPrice: 35 * lakh, DailyRate: 4500, Deposit: 20000, KmPerDay: 250, ExtraKm: 14, Insurance: 400},
	{ID: "b35_50L", MinPrice: 35 * lakh, MaxPrice: 50 * lakh, DailyRate: 7000, Deposit: 35000, KmPerDay: 200, ExtraKm: 20, Insurance: 600},
	{ID: "b50_60L", MinPrice: 50 * lakh, MaxPrice: 60 * lakh, DailyRate: 9500, Deposit: 50000, KmPerDay: 200, ExtraKm: 25, Insurance: 800},
	{ID: "b60_80L", MinPrice: 60 * lakh, MaxPrice: 80 * lakh, DailyRate: 12500, Deposit: 75000, KmPerDay: 200, ExtraKm: 30, Insurance: 1000},
	{ID: "b80L_1Cr", MinPrice: 80 * lakh, MaxPrice: 100 * lakh, DailyRate: 16000, Deposit: 100000, KmPerDay: 150, ExtraKm: 40, Insurance: 1400},
	{ID: "b1Cr_plus", MinPrice: 100 * lakh, MaxPrice: 1e12, DailyRate: 25000, Deposit: 200000, KmPerDay: 150, ExtraKm: 60, Insurance: 2500},
}

// Bands returns the configured bands (sr:selfdrive:bands JSON, no expiry) or the defaults.
func (c *Cache) Bands(ctx context.Context) []pricing.Band {
	var bands []pricing.Band
	if found, err := c.cs.GetJSON(ctx, cache.KeySelfDriveBands(), &bands); err == nil && found && len(bands) > 0 {
		return bands
	}
	return DefaultBands
}

// SetBands replaces the band table; ranges must not overlap.
func (c *Cache) SetBands(ctx context.Context, bands []pricing.Band) error {
	sort.Slice(bands, func(i, j int) bool { return bands[i].MinPrice < bands[j].MinPrice })
	for i := 1; i < len(bands); i++ {
		if bands[i].MinPrice < bands[i-1].MaxPrice {
			return apperr.BadRequest("price bands overlap: " + bands[i-1].ID + " / " + bands[i].ID)
		}
	}
	return c.cs.SetJSON(ctx, cache.KeySelfDriveBands(), bands, 0)
}

// BandFor returns the band of an ex-showroom price.
func BandFor(bands []pricing.Band, price float64) (pricing.Band, error) {
	for _, b := range bands {
		if price >= b.MinPrice && price < b.MaxPrice {
			return b, nil
		}
	}
	return pricing.Band{}, apperr.BadRequest("no price band for this car")
}
