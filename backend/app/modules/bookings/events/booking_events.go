
package events

const (
    BookingCreated         = "booking.created"
    BookingDriverSearching = "booking.driver_searching"
    BookingDriverAssigned  = "booking.driver_assigned"
    BookingDriverArrived   = "booking.driver_arrived"
    BookingTripStarted     = "booking.trip_started"
    BookingTripCompleted   = "booking.trip_completed"
    BookingCancelled       = "booking.cancelled"
    BookingPaymentCompleted = "booking.payment_completed"
)

const (
    SharedRideCreated          = "shared_ride.created"
    SharedRidePassengerJoined  = "shared_ride.passenger_joined"
    SharedRidePassengerCancelled = "shared_ride.passenger_cancelled"
    SharedRideSeatsUpdated     = "shared_ride.seats_updated"
    SharedRidePickupUpdated    = "shared_ride.pickup_updated"
    SharedRideRouteUpdated     = "shared_ride.route_updated"
    SharedRideCompleted        = "shared_ride.completed"
)