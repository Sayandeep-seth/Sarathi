package pricing

// HourlyFare prices premium & vintage bookings (charged per hour).
//
//	ratePerHour  model-specific (from the premium fleet hash in Redis)
//	minHours     minimum billable hours (premium 4, vintage 3)
//	extras       decoration, chauffeur uniform, red carpet, etc.
func HourlyFare(ratePerHour float64, hours, minHours int, extras map[string]float64) Breakdown {
	if hours < minHours {
		hours = minHours
	}
	b := NewBreakdown()
	b.AddFixed("hours", float64(hours)*ratePerHour)
	for name, amt := range extras {
		b.AddFixed(name, amt)
	}
	b.Finalize()
	return b
}
