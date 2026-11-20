// Package utils contains small helpers: ids, geo math, dates, request helpers.
package utils

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"sarathi/internal/common/constants"
)

// Point is a WGS84 coordinate.
type Point struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// Valid reports whether the point is a real coordinate.
func (p Point) Valid() bool {
	return p.Lat >= -85.05 && p.Lat <= 85.05 && p.Lng >= -180 && p.Lng <= 180 && !(p.Lat == 0 && p.Lng == 0)
}

// NewID returns "<prefix>_<16 hex chars>".
func NewID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return prefix + "_" + hex.EncodeToString(b)
}

// RandomToken returns n random bytes hex encoded.
func RandomToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// HashKey builds a short stable hash from parts, used for cache keys.
func HashKey(parts ...any) string {
	h := sha1.New()
	for _, p := range parts {
		fmt.Fprintf(h, "%v|", p)
	}
	return hex.EncodeToString(h.Sum(nil))[:20]
}

// HaversineKm returns the great-circle distance in kilometres.
func HaversineKm(a, b Point) float64 {
	const r = 6371.0
	dLat := (b.Lat - a.Lat) * math.Pi / 180
	dLng := (b.Lng - a.Lng) * math.Pi / 180
	la1 := a.Lat * math.Pi / 180
	la2 := b.Lat * math.Pi / 180
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(la1)*math.Cos(la2)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * r * math.Asin(math.Sqrt(h))
}

// Sample is a point on a polyline with its distance from the start.
type Sample struct {
	Point   Point
	AlongKm float64
}

// SamplePolyline returns points every `everyKm` along a polyline (always includes start and end).
func SamplePolyline(line []Point, everyKm float64) []Sample {
	if len(line) == 0 {
		return nil
	}
	out := []Sample{{Point: line[0]}}
	along, next := 0.0, everyKm
	for i := 1; i < len(line); i++ {
		seg := HaversineKm(line[i-1], line[i])
		along += seg
		if along >= next {
			out = append(out, Sample{Point: line[i], AlongKm: along})
			next = along + everyKm
		}
	}
	if last := out[len(out)-1]; last.Point != line[len(line)-1] {
		out = append(out, Sample{Point: line[len(line)-1], AlongKm: along})
	}
	return out
}

// PolylineKm returns the total length of a polyline.
func PolylineKm(line []Point) float64 {
	d := 0.0
	for i := 1; i < len(line); i++ {
		d += HaversineKm(line[i-1], line[i])
	}
	return d
}

// Cell buckets a coordinate into a grid cell (~2.2 km at 0.02 deg) for surge/heatmaps.
func Cell(p Point, sizeDeg float64) string {
	return fmt.Sprintf("%d:%d", int(math.Floor(p.Lat/sizeDeg)), int(math.Floor(p.Lng/sizeDeg)))
}

// IST is India Standard Time; all business dates are in IST.
var IST = time.FixedZone("IST", 5*3600+1800)

// DateKey is yyyymmdd in IST.
func DateKey(t time.Time) string { return t.In(IST).Format("20060102") }

// MonthKey is yyyymm in IST.
func MonthKey(t time.Time) string { return t.In(IST).Format("200601") }

// HourKey is yyyymmddhh in IST.
func HourKey(t time.Time) string { return t.In(IST).Format("2006010215") }

// Epoch for day-index bitmaps (calendar availability).
var calendarEpoch = time.Date(2024, 1, 1, 0, 0, 0, 0, IST)

// DayIndex returns the number of days since 2024-01-01 (IST); used as bitmap offset.
func DayIndex(t time.Time) int {
	y, m, d := t.In(IST).Date()
	day := time.Date(y, m, d, 0, 0, 0, 0, IST)
	return int(day.Sub(calendarEpoch).Hours() / 24)
}

// DayRange returns all day indexes from start to end inclusive.
func DayRange(start, end time.Time) []int {
	a, b := DayIndex(start), DayIndex(end)
	out := make([]int, 0, b-a+1)
	for i := a; i <= b; i++ {
		out = append(out, i)
	}
	return out
}

// ParseDate parses yyyy-mm-dd in IST.
func ParseDate(s string) (time.Time, error) { return time.ParseInLocation("2006-01-02", s, IST) }

// Round rounds to n decimals.
func Round(v float64, n int) float64 {
	p := math.Pow(10, float64(n))
	return math.Round(v*p) / p
}

// Clamp bounds v to [lo, hi].
func Clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

// UserID extracts the authenticated user id set by the gateway.
func UserID(r *http.Request) string { return strings.TrimSpace(r.Header.Get(constants.HeaderUserID)) }

// QueryFloat parses a float query parameter.
func QueryFloat(r *http.Request, name string, def float64) float64 {
	if v, err := strconv.ParseFloat(r.URL.Query().Get(name), 64); err == nil {
		return v
	}
	return def
}

// QueryInt parses an int query parameter.
func QueryInt(r *http.Request, name string, def int) int {
	if v, err := strconv.Atoi(r.URL.Query().Get(name)); err == nil {
		return v
	}
	return def
}

// NormalizeName lower-cases and collapses whitespace (car models, place names).
func NormalizeName(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}
