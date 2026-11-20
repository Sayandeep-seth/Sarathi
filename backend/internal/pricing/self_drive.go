package pricing

import "math"

// Band is a self-drive price band: every car whose ex-showroom price falls in
// [MinPrice, MaxPrice) has the same daily fare and deposit (e.g. 50L–60L).
type Band struct {
	ID        string  `json:"id"`
	MinPrice  float64 `json:"min_price"`
	MaxPrice  float64 `json:"max_price"`
	DailyRate float64 `json:"daily_rate"`
	Deposit   float64 `json:"deposit"`
	KmPerDay  float64 `json:"km_per_day"`
	ExtraKm   float64 `json:"extra_km"`
	Insurance float64 `json:"insurance_per_day"`
}

// SelfDriveFare prices a self-drive rental; the deposit is returned separately.
func SelfDriveFare(b Band, days int, estKm float64) Breakdown {
	if days < 1 {
		days = 1
	}
	out := NewBreakdown()
	out.AddFixed("rent", float64(days)*b.DailyRate)
	out.AddFixed("insurance", float64(days)*b.Insurance)
	if extra := estKm - float64(days)*b.KmPerDay; extra > 0 {
		out.AddFixed("extra_km", math.Ceil(extra)*b.ExtraKm)
	}
	out.Deposit = b.Deposit
	out.Finalize()
	return out
}
