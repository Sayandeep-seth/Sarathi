package pricing

import "math"

// DailyRate is the tourist-trip tariff for a vehicle class.
type DailyRate struct {
	Class           string  `json:"class"`
	PerDay          float64 `json:"per_day"`
	KmPerDay        float64 `json:"km_per_day"` // free km allowance per day
	ExtraKm         float64 `json:"extra_km"`
	DriverAllowance float64 `json:"driver_allowance"` // per day (food/stay of driver)
	NightCharge     float64 `json:"night_charge"`     // per night outstation
}

// DailyFare prices a tourist trip (charged per day + journey km beyond allowance).
func DailyFare(r DailyRate, days int, estKm float64, nights int) Breakdown {
	if days < 1 {
		days = 1
	}
	b := NewBreakdown()
	b.AddFixed("days", float64(days)*r.PerDay)
	if extra := estKm - float64(days)*r.KmPerDay; extra > 0 {
		b.AddFixed("extra_km", math.Ceil(extra)*r.ExtraKm)
	}
	b.AddFixed("driver_allowance", float64(days)*r.DriverAllowance)
	if nights > 0 {
		b.AddFixed("night_charge", float64(nights)*r.NightCharge)
	}
	b.Finalize()
	return b
}
