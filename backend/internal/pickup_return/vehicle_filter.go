package pickup_return

import (
	"sarathi/internal/common/constants"
	apperr "sarathi/internal/common/errors"
)

// ClassForSeats maps the only allowed choice (5 or 7 seats) to the driver GEO set.
// Passengers cannot pick a car model for this booking type.
func ClassForSeats(seats int) (string, error) {
	switch seats {
	case 5:
		return constants.ClassSeat5, nil
	case 7:
		return constants.ClassSeat7, nil
	}
	return "", apperr.BadRequest("pickup & return can only be booked as a 5 seater or 7 seater")
}

// RejectModel makes sure the client did not try to choose a model.
func RejectModel(model string) error {
	if model != "" {
		return apperr.BadRequest("car model cannot be selected for pickup & return; choose 5 or 7 seats")
	}
	return nil
}
