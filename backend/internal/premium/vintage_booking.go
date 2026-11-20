package premium

import (
	"context"

	"sarathi/internal/booking"
	"sarathi/internal/common/constants"
	apperr "sarathi/internal/common/errors"
)

const vintageMinHours = 3

// Vintage add-ons for ceremonies (INR, flat).
var vintageExtras = map[string]float64{
	"flower_decoration":   3500,
	"ribbon_and_bow":      800,
	"red_carpet":          1500,
	"uniformed_chauffeur": 1000,
	"photo_shoot_stop":    2000,
}

var ceremonyTypes = map[string]bool{
	"wedding": true, "baraat": true, "reception": true, "engagement": true,
	"photo_shoot": true, "film_shoot": true, "corporate": true, "anniversary": true,
}

// BookVintage books a vintage car for a ceremony, charged per hour (min 3 h) plus add-ons.
func (s *Service) BookVintage(ctx context.Context, userID string, req HourlyRequest) (*booking.Booking, error) {
	if !ceremonyTypes[req.EventType] {
		return nil, apperr.BadRequest("event_type must be one of wedding, baraat, reception, engagement, photo_shoot, film_shoot, corporate, anniversary")
	}
	extras := map[string]float64{}
	for _, e := range req.Extras {
		price, ok := vintageExtras[e]
		if !ok {
			return nil, apperr.BadRequest("unknown extra: " + e)
		}
		extras[e] = price
	}
	return s.book(ctx, userID, req, CategoryVintage, constants.BookingVintage, vintageMinHours, extras)
}

// VintageExtras lists add-ons for the app.
func VintageExtras() map[string]float64 { return vintageExtras }
