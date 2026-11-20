// Package driver manages driver availability: status, live GEO membership per
// vehicle class, heartbeats and stale-driver cleanup — all in Redis.
package driver

// Driver statuses.
const (
	StatusOffline   = "offline"
	StatusAvailable = "available"
	StatusOnTrip    = "on_trip"
	StatusBreak     = "break"
)

// allowedFrom lists the statuses a driver may move FROM to reach a status.
var allowedFrom = map[string][]string{
	StatusAvailable: {StatusOffline, StatusOnTrip, StatusBreak, StatusAvailable},
	StatusOnTrip:    {StatusAvailable},
	StatusBreak:     {StatusAvailable},
	StatusOffline:   {StatusAvailable, StatusBreak, StatusOffline, StatusOnTrip},
}

// AllowedFrom returns the valid previous statuses for `to`.
func AllowedFrom(to string) []string { return allowedFrom[to] }

// Valid reports whether s is a known status.
func Valid(s string) bool { _, ok := allowedFrom[s]; return ok }
