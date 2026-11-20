package pricing

import "math"

// ParkingTariff is a hub's parking rule (airport, railway station, metro).
type ParkingTariff struct {
	HubID       string  `json:"hub_id"`
	Name        string  `json:"name"`
	FreeMinutes float64 `json:"free_minutes"` // e.g. 10-min free pickup lane
	BaseFee     float64 `json:"base_fee"`     // entry / first slab
	PerHour     float64 `json:"per_hour"`
	MaxFee      float64 `json:"max_fee"` // daily cap (0 = none)
}

// ParkingFee for a wait of waitMin minutes (option 3: pickup & return pays parking).
func ParkingFee(t ParkingTariff, waitMin float64) float64 {
	if waitMin <= t.FreeMinutes {
		return 0
	}
	hours := math.Ceil((waitMin - t.FreeMinutes) / 60)
	fee := t.BaseFee + math.Max(0, hours-1)*t.PerHour
	if t.MaxFee > 0 && fee > t.MaxFee {
		fee = t.MaxFee
	}
	return fee
}
