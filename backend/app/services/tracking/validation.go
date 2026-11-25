
func ValidateLocation(latitude, longitude, speed, heading float64) bool {
    if latitude < -90 || latitude > 90 {
        return false
    }

    if longitude < -180 || longitude > 180 {
        return false
    }

    if speed < 0 || speed > 250 {
        return false
    }

    if heading < 0 || heading >= 360 {
        return false
    }

    return true
}
