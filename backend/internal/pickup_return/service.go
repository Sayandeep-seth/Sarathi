package pickup_return

import (
	"context"
	"strconv"
	"time"

	"sarathi/internal/booking"
	"sarathi/internal/common/constants"
	apperr "sarathi/internal/common/errors"
	"sarathi/internal/common/utils"
	"sarathi/internal/pricing"
)

type Service struct {
	parking  *Parking
	bookings *booking.Service
	pricing  *pricing.Service
}

func NewService(p *Parking, b *booking.Service, pr *pricing.Service) *Service {
	return &Service{parking: p, bookings: b, pricing: pr}
}

// Request: origin (home), the hub to pick someone up from, seats and expected wait.
type Request struct {
	Origin       utils.Point `json:"origin"`
	PickupPlace  utils.Point `json:"pickup_place"`
	Seats        int         `json:"seats"`
	CarModel     string      `json:"car_model,omitempty"` // must be empty
	ExpectedWait int         `json:"expected_wait_min"`   // e.g. flight delay buffer
	ArrivalAt    *time.Time  `json:"arrival_at"`          // when the guest arrives at the hub
	GuestName    string      `json:"guest_name"`
	GuestPhone   string      `json:"guest_phone"`
}

// Quote prices: out to the hub + back to origin + waiting + parking fee.
func (s *Service) Quote(ctx context.Context, req Request) (*pricing.Breakdown, *Hub, error) {
	if err := RejectModel(req.CarModel); err != nil {
		return nil, nil, err
	}
	class, err := ClassForSeats(req.Seats)
	if err != nil {
		return nil, nil, err
	}
	if !req.Origin.Valid() || !req.PickupPlace.Valid() {
		return nil, nil, apperr.BadRequest("origin and pickup place required")
	}
	hub, err := s.parking.HubAt(ctx, req.PickupPlace)
	if err != nil {
		return nil, nil, err
	}
	km, min := pricing.EstimateTrip(req.Origin, req.PickupPlace)
	rc := s.pricing.RateCard(ctx, class)
	surge := s.pricing.Surge.Multiplier(ctx, class, req.Origin)

	b := pricing.NewBreakdown()
	b.AddSurgeable("base", rc.Base)
	b.AddSurgeable("distance_out_and_back", rc.PerKm*km*2)
	b.AddSurgeable("time_out_and_back", rc.PerMin*min*2)
	b.SurgeMultiplier = surge
	b.MinFare = rc.MinFare * 2
	wait := float64(max(req.ExpectedWait, 0))
	b.AddFixed("waiting", rc.WaitPerMin*wait)
	if hub != nil {
		t, _ := s.parking.Tariff(ctx, hub.ID)
		b.AddFixed("parking", pricing.ParkingFee(t, wait+15)) // +15 min to walk out with the guest
	} else {
		b.AddFixed("parking", 0)
	}
	b.Finalize()
	return &b, hub, nil
}

// Book creates the booking and dispatches a 5/7-seat cab (scheduled before arrival_at).
func (s *Service) Book(ctx context.Context, userID string, req Request) (*booking.Booking, error) {
	fare, hub, err := s.Quote(ctx, req)
	if err != nil {
		return nil, err
	}
	class, _ := ClassForSeats(req.Seats)
	var when *time.Time
	if req.ArrivalAt != nil {
		_, driveMin := pricing.EstimateTrip(req.Origin, req.PickupPlace)
		leave := req.ArrivalAt.Add(-time.Duration(driveMin+10) * time.Minute) // reach the hub 10 min early
		when = &leave
	}
	meta := map[string]string{
		"seats": strconv.Itoa(req.Seats), "guest_name": req.GuestName, "guest_phone": req.GuestPhone,
		"return_to_lat": strconv.FormatFloat(req.Origin.Lat, 'f', 6, 64), "return_to_lng": strconv.FormatFloat(req.Origin.Lng, 'f', 6, 64),
	}
	if hub != nil {
		meta["hub_id"], meta["hub_name"], meta["hub_type"] = hub.ID, hub.Name, hub.Type
	}
	return s.bookings.Create(ctx, booking.CreateInput{
		UserID: userID, Type: constants.BookingPickupReturn, Class: class,
		Pickup: req.Origin, Drop: req.PickupPlace, ScheduledAt: when, Fare: fare, Meta: meta,
	})
}
