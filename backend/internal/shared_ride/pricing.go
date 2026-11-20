package shared_ride

import (
	"sarathi/internal/pricing"
)

// Discount by number of riders in the pool (incl. this one).
func Discount(riders int) float64 {
	switch {
	case riders >= 3:
		return 0.40
	case riders == 2:
		return 0.35
	default:
		return 0.25 // even alone in a pool you pay less than a private cab
	}
}

// SharedFare: rider pays the solo fare minus the pool discount, per seat.
func SharedFare(solo pricing.Breakdown, riders, seats int) pricing.Breakdown {
	b := pricing.NewBreakdown()
	for k, v := range solo.Items {
		if k == "parking" {
			continue
		}
		b.AddSurgeable(k, v*float64(seats))
	}
	b.SurgeMultiplier = solo.SurgeMultiplier
	b.Discount = solo.Total * float64(seats) * Discount(riders)
	b.MinFare = solo.MinFare * 0.6
	b.Finalize()
	return b
}

// DriverEarning: drivers keep 85 % (vs 80 % on solo rides) of the pooled fares,
// so a full shared cab earns clearly more than one private ride on the same route.
func DriverEarning(riderFares []float64) float64 {
	sum := 0.0
	for _, f := range riderFares {
		sum += f
	}
	return sum * 0.85
}
