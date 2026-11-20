// Package booking is the core trip record shared by every booking type
// (cab, shared, pickup-return, premium, vintage, tourist, self-drive, test-drive cab).
package booking

// Booking states.
const (
	StatePendingPayment = "pending_payment" // prepaid types wait for payment
	StateScheduled      = "scheduled"       // future pickup, dispatched by the worker
	StateConfirmed      = "confirmed"       // prepaid & no driver needed yet (self-drive, premium)
	StateSearching      = "searching"       // dispatch in progress
	StateAssigned       = "driver_assigned"
	StateArrived        = "driver_arrived"
	StateInProgress     = "in_progress"
	StateCompleted      = "completed"
	StateCancelled      = "cancelled"
	StateNoDriver       = "no_driver"
)

// from[to] = states a booking may be in before moving to `to`.
var from = map[string][]string{
	StateScheduled:  {StatePendingPayment},
	StateConfirmed:  {StatePendingPayment},
	StateSearching:  {StateScheduled, StateConfirmed, StateNoDriver},
	StateAssigned:   {StateSearching},
	StateArrived:    {StateAssigned},
	StateInProgress: {StateAssigned, StateArrived, StateConfirmed},
	StateCompleted:  {StateInProgress},
	StateNoDriver:   {StateSearching},
	StateCancelled:  {StatePendingPayment, StateScheduled, StateConfirmed, StateSearching, StateAssigned, StateArrived, StateNoDriver},
}

// AllowedFrom returns the valid previous states for `to`.
func AllowedFrom(to string) []string { return from[to] }

// Active reports whether a booking still occupies the rider (one active cab at a time).
func Active(state string) bool {
	switch state {
	case StateSearching, StateAssigned, StateArrived, StateInProgress:
		return true
	}
	return false
}
