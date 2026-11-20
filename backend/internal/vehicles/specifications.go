// Package vehicles owns car specifications (dimensions, fuel, range) and their caching.
package vehicles

import (
	"fmt"
	"time"

	"sarathi/internal/common/constants"
	apperr "sarathi/internal/common/errors"
	"sarathi/internal/common/utils"
)

// Spec is the physical and energy specification of a car model.
type Spec struct {
	Model       string `json:"model"` // normalized key, e.g. "maruti suzuki swift"
	DisplayName string `json:"display_name"`
	Make        string `json:"make"`
	BodyType    string `json:"body_type"` // hatchback, sedan, suv, muv, luxury, vintage

	LengthMM          int     `json:"length_mm"`
	WidthMM           int     `json:"width_mm"`
	MirrorWidthMM     int     `json:"mirror_width_mm"` // width incl. mirrors (0 = unknown)
	HeightMM          int     `json:"height_mm"`
	WheelbaseMM       int     `json:"wheelbase_mm"`
	GroundClearanceMM int     `json:"ground_clearance_mm"`
	TurningRadiusM    float64 `json:"turning_radius_m"`
	Seats             int     `json:"seats"`

	FuelType      string  `json:"fuel_type"`
	TankLitres    float64 `json:"tank_litres"`
	MileageKmpl   float64 `json:"mileage_kmpl"` // real-world km per litre (km/kg for CNG)
	BatteryKWh    float64 `json:"battery_kwh"`
	FullRangeKm   float64 `json:"full_range_km"` // EV range on 100% charge
	ConnectorType string  `json:"connector_type"`

	PriceINR  float64   `json:"price_inr"` // ex-showroom, used for self-drive price bands
	Source    string    `json:"source"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NormalizeModel turns "  Maruti  SWIFT " into "maruti swift".
func NormalizeModel(name string) string { return utils.NormalizeName(name) }

// IsEV reports whether the car runs on a battery only.
func (s *Spec) IsEV() bool { return s.FuelType == constants.FuelEV }

// EffectiveWidthM is the width that must physically pass (mirrors included).
func (s *Spec) EffectiveWidthM() float64 {
	w := s.MirrorWidthMM
	if w == 0 {
		w = s.WidthMM + 180 // typical mirror overhang on both sides
	}
	return float64(w) / 1000
}

func (s *Spec) LengthM() float64 { return float64(s.LengthMM) / 1000 }
func (s *Spec) HeightM() float64 { return float64(s.HeightMM) / 1000 }

// RangeFor returns the driving range for the given amount of fuel / charge.
// unit: "litres", "kwh" or "percent".
func (s *Spec) RangeFor(amount float64, unit string) (float64, error) {
	if amount < 0 {
		return 0, apperr.BadRequest("fuel amount cannot be negative")
	}
	switch {
	case s.IsEV() && unit == "percent":
		return s.FullRangeKm * utils.Clamp(amount, 0, 100) / 100, nil
	case s.IsEV() && unit == "kwh":
		if s.BatteryKWh == 0 {
			return 0, apperr.BadRequest("battery capacity unknown for " + s.DisplayName)
		}
		return s.FullRangeKm * amount / s.BatteryKWh, nil
	case !s.IsEV() && unit == "litres":
		return amount * s.MileageKmpl, nil
	case !s.IsEV() && unit == "percent":
		return s.TankLitres * utils.Clamp(amount, 0, 100) / 100 * s.MileageKmpl, nil
	}
	return 0, apperr.BadRequest(fmt.Sprintf("unit %q not valid for a %s car", unit, s.FuelType))
}

// Validate checks that the spec has the fields routing and fuel planning need.
func (s *Spec) Validate() error {
	if s.Model == "" || s.WidthMM <= 0 || s.LengthMM <= 0 || s.HeightMM <= 0 {
		return apperr.BadRequest("spec needs model, length, width and height")
	}
	if s.IsEV() && s.FullRangeKm <= 0 {
		return apperr.BadRequest("EV spec needs full_range_km")
	}
	if !s.IsEV() && s.MileageKmpl <= 0 {
		return apperr.BadRequest("fuel spec needs mileage_kmpl")
	}
	return nil
}
