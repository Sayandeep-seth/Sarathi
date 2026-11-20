package daily_bus

import (
	"time"

	"sarathi/internal/common/utils"
)

// Ticket kinds.
const (
	KindDay     = "day"
	KindMonthly = "monthly"
)

// Ticket is a single-day ride or a monthly pass (same seat every working day).
type Ticket struct {
	ID        string   `json:"id"`
	UserID    string   `json:"user_id"`
	RouteID   string   `json:"route_id"`
	FromStop  string   `json:"from_stop"`
	ToStop    string   `json:"to_stop"`
	Seat      int      `json:"seat"`
	Dates     []string `json:"dates"` // yyyymmdd
	Fare      float64  `json:"fare"`
	Kind      string   `json:"kind"`
	Status    string   `json:"status"` // pending_payment | confirmed | cancelled
	PaymentID string   `json:"payment_intent_id,omitempty"`
}

// WorkingDays returns Mon–Sat dates from `from` until the end of that month.
func WorkingDays(from time.Time) []string {
	from = from.In(utils.IST)
	var out []string
	for d := from; d.Month() == from.Month(); d = d.AddDate(0, 0, 1) {
		if d.Weekday() != time.Sunday {
			out = append(out, utils.DateKey(d))
		}
	}
	return out
}

// MonthlyPrice: 22-ish rides at a 20 % pass discount.
func MonthlyPrice(single float64, rides int) float64 {
	return float64(int(single*float64(rides)*0.8 + 0.5))
}
