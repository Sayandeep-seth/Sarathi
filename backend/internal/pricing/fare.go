// Package pricing computes fares for every booking type; rate cards and surge
// multipliers are cached in Redis.
package pricing

import (
	"math"

	"sarathi/internal/common/constants"
	"sarathi/internal/common/utils"
)

// RateCard is the per-class distance/time tariff.
type RateCard struct {
	Class      string  `json:"class"`
	Base       float64 `json:"base"`
	PerKm      float64 `json:"per_km"`
	PerMin     float64 `json:"per_min"`
	MinFare    float64 `json:"min_fare"`
	WaitPerMin float64 `json:"wait_per_min"`
}

// DefaultRates are used when Redis/Postgres have no card for a class (INR).
var DefaultRates = map[string]RateCard{
	constants.ClassCab:       {Class: constants.ClassCab, Base: 50, PerKm: 12, PerMin: 1.5, MinFare: 100, WaitPerMin: 2},
	constants.ClassSeat5:     {Class: constants.ClassSeat5, Base: 50, PerKm: 12, PerMin: 1.5, MinFare: 100, WaitPerMin: 2},
	constants.ClassSeat7:     {Class: constants.ClassSeat7, Base: 70, PerKm: 16, PerMin: 2, MinFare: 150, WaitPerMin: 2.5},
	constants.ClassShared:    {Class: constants.ClassShared, Base: 30, PerKm: 12, PerMin: 1, MinFare: 60, WaitPerMin: 0},
	constants.ClassTestDrive: {Class: constants.ClassTestDrive, Base: 60, PerKm: 13, PerMin: 1.5, MinFare: 120, WaitPerMin: 2},
}

// Breakdown is an itemised fare. Surgeable items are multiplied by surge.
type Breakdown struct {
	Items           map[string]float64 `json:"items"`
	SurgeMultiplier float64            `json:"surge_multiplier"`
	Discount        float64            `json:"discount"`
	Deposit         float64            `json:"deposit,omitempty"` // refundable, not in Total
	MinFare         float64            `json:"min_fare,omitempty"`
	Total           float64            `json:"total"`
	Currency        string             `json:"currency"`

	surgeable float64
	fixed     float64
}

func NewBreakdown() Breakdown {
	return Breakdown{Items: map[string]float64{}, SurgeMultiplier: 1, Currency: "INR"}
}

// AddSurgeable adds an item affected by surge (distance, time, base).
func (b *Breakdown) AddSurgeable(name string, amt float64) {
	b.Items[name] += utils.Round(amt, 2)
	b.surgeable += amt
}

// AddFixed adds an item not affected by surge (parking, tolls, food, allowances).
func (b *Breakdown) AddFixed(name string, amt float64) {
	b.Items[name] += utils.Round(amt, 2)
	b.fixed += amt
}

// Finalize computes Total (rounded to the rupee).
func (b *Breakdown) Finalize() {
	t := b.surgeable*b.SurgeMultiplier + b.fixed - b.Discount
	if t < b.MinFare {
		t = b.MinFare
	}
	b.Total = math.Round(t)
	b.Discount = utils.Round(b.Discount, 2)
}

// RideFare is distance+time pricing used by cab, shared, pickup-return and test-drive cabs.
func RideFare(rc RateCard, km, minutes, surge float64) Breakdown {
	b := NewBreakdown()
	b.AddSurgeable("base", rc.Base)
	b.AddSurgeable("distance", rc.PerKm*km)
	b.AddSurgeable("time", rc.PerMin*minutes)
	b.SurgeMultiplier = surge
	b.MinFare = rc.MinFare
	b.Items["parking"] = 0 // option 1: normal cab never charges parking
	b.Finalize()
	return b
}
