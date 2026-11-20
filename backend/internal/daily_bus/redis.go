package daily_bus

import (
	"context"
	"strings"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
	apperr "sarathi/internal/common/errors"
	"sarathi/internal/common/utils"
)

// Store:
//
//	sr:dbus:route:<id>       STRING json route
//	sr:dbus:routes           SET    route ids
//	sr:geo:dbus:stops        GEO    member "<route>|<stop>" (find routes near home & office)
//	sr:dbus:sub:<id>         HASH   ticket / monthly subscription
type Store struct {
	rdb *redis.Client
	cs  *cache.Service
	geo *cache.Geo
}

func NewStore(cs *cache.Service, geo *cache.Geo) *Store {
	return &Store{rdb: cs.Client(), cs: cs, geo: geo}
}

func (s *Store) PutRoute(ctx context.Context, r Route) error {
	if err := s.cs.SetJSON(ctx, cache.KeyDailyBusRoute(r.ID), r, 0); err != nil {
		return err
	}
	pipe := s.rdb.Pipeline()
	pipe.SAdd(ctx, cache.KeyDailyBusRoutes(), r.ID)
	for _, st := range r.Stops {
		cache.AddPipe(ctx, pipe, cache.KeyDailyBusStopsGeo(), r.ID+"|"+st.ID, st.Location)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (s *Store) Route(ctx context.Context, id string) (*Route, error) {
	var r Route
	found, err := s.cs.GetJSON(ctx, cache.KeyDailyBusRoute(id), &r)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, apperr.NotFound("bus route not found")
	}
	return &r, nil
}

// StopHit is a stop near a point.
type StopHit struct {
	RouteID string
	StopID  string
	WalkKm  float64
}

func (s *Store) StopsNear(ctx context.Context, p utils.Point, radiusKm float64) ([]StopHit, error) {
	hits, err := s.geo.Nearby(ctx, cache.KeyDailyBusStopsGeo(), p, radiusKm, 50)
	if err != nil {
		return nil, err
	}
	out := make([]StopHit, 0, len(hits))
	for _, h := range hits {
		route, stop, _ := strings.Cut(h.Member, "|")
		out = append(out, StopHit{RouteID: route, StopID: stop, WalkKm: h.DistKm})
	}
	return out, nil
}

func (s *Store) SaveTicket(ctx context.Context, t *Ticket) error {
	return s.rdb.HSet(ctx, cache.KeyDailyBusSubscription(t.ID), map[string]any{
		"user_id": t.UserID, "route_id": t.RouteID, "from": t.FromStop, "to": t.ToStop, "seat": t.Seat,
		"dates": strings.Join(t.Dates, ","), "fare": t.Fare, "kind": t.Kind, "status": t.Status,
	}).Err()
}

func (s *Store) Ticket(ctx context.Context, id string) (*Ticket, error) {
	m, err := s.rdb.HGetAll(ctx, cache.KeyDailyBusSubscription(id)).Result()
	if err != nil {
		return nil, err
	}
	if len(m) == 0 {
		return nil, apperr.NotFound("ticket not found")
	}
	return &Ticket{
		ID: id, UserID: m["user_id"], RouteID: m["route_id"], FromStop: m["from"], ToStop: m["to"],
		Seat: int(cache.HGetInt(m, "seat")), Dates: strings.Split(m["dates"], ","), Fare: cache.HGetFloat(m, "fare"),
		Kind: m["kind"], Status: m["status"],
	}, nil
}

func (s *Store) SetTicketStatus(ctx context.Context, id, status string) error {
	return s.rdb.HSet(ctx, cache.KeyDailyBusSubscription(id), "status", status).Err()
}
