package luxury_bus

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
	apperr "sarathi/internal/common/errors"
	"sarathi/internal/common/utils"
	"sarathi/internal/notifications"
	"sarathi/internal/payments"
	"sarathi/internal/websocket"
)

type Service struct {
	store    *Store
	seats    *Seats
	payments *payments.Service
	pub      *websocket.Publisher
	notify   *notifications.Queue
	log      *slog.Logger
}

func NewService(st *Store, se *Seats, p *payments.Service, pub *websocket.Publisher, n *notifications.Queue, log *slog.Logger) *Service {
	s := &Service{store: st, seats: se, payments: p, pub: pub, notify: n, log: log}
	p.OnPaid("luxury_bus", s.onTicketPaid)
	p.OnPaid("luxury_food", s.onFoodPaid)
	return s
}

// CreateTrip publishes a run with its restaurant stops (operator).
func (s *Service) CreateTrip(ctx context.Context, t Trip, stops []Stop) error {
	if t.ID == "" || t.Seats <= 0 || t.Seats > 60 || t.FarePerSeat <= 0 {
		return apperr.BadRequest("trip needs id, 1..60 seats and a fare")
	}
	if err := s.store.PutTrip(ctx, t); err != nil {
		return err
	}
	return s.store.PutStops(ctx, t.ID, stops)
}

// Details is what the booking screen shows: seat map, stops and menus.
func (s *Service) Details(ctx context.Context, tripID string) (map[string]any, error) {
	t, err := s.store.Trip(ctx, tripID)
	if err != nil {
		return nil, err
	}
	seatMap, err := s.seats.Map(ctx, t)
	if err != nil {
		return nil, err
	}
	stops, err := s.store.Stops(ctx, tripID)
	if err != nil {
		return nil, err
	}
	menus := map[string]map[string]MenuItem{}
	for _, st := range stops {
		menus[st.ID], _ = s.store.Menu(ctx, st.RestaurantID)
	}
	return map[string]any{"trip": t, "seats_free": seatMap, "stops": stops, "menus": menus}, nil
}

type BookRequest struct {
	TripID string      `json:"trip_id"`
	Seats  []int       `json:"seats"` // explicit seat numbers, or
	Count  int         `json:"count"` // auto-pick
	Food   []OrderLine `json:"food"`  // optional meals, like a train
}

// Book holds seats, prices meals and opens one payment for ticket + food.
func (s *Service) Book(ctx context.Context, userID string, req BookRequest) (*Ticket, *payments.Intent, error) {
	t, err := s.store.Trip(ctx, req.TripID)
	if err != nil {
		return nil, nil, err
	}
	if time.Until(t.Departure) < 30*time.Minute {
		return nil, nil, apperr.Conflict("booking closed for this departure")
	}
	n := len(req.Seats)
	if n == 0 {
		n = req.Count
	}
	if n < 1 || n > 6 {
		return nil, nil, apperr.BadRequest("book 1..6 seats")
	}
	seats, err := s.seats.Pick(ctx, t, req.Seats, n)
	if err != nil {
		return nil, nil, err
	}
	tk := &Ticket{PNR: NewPNR(), TripID: t.ID, UserID: userID, Seats: seats, Fare: t.FarePerSeat * float64(n), Status: "pending_payment"}
	if err := s.seats.Hold(ctx, t, tk.PNR, seats); err != nil {
		return nil, nil, err
	}
	fail := func(err error) (*Ticket, *payments.Intent, error) {
		s.seats.Free(ctx, t, tk.PNR, seats)
		return nil, nil, err
	}
	if len(req.Food) > 0 {
		total, err := s.store.PriceOrder(ctx, t.ID, req.Food)
		if err != nil {
			return fail(err)
		}
		if _, err := s.store.CreateOrder(ctx, tk.PNR, t.ID, req.Food, total); err != nil {
			return fail(err)
		}
		tk.FoodTotal = total
	}
	intent, err := s.payments.CreateIntent(ctx, "luxury_bus", tk.PNR, userID, tk.Fare+tk.FoodTotal)
	if err != nil {
		return fail(err)
	}
	tk.PaymentID = intent.ID
	if err := s.store.SaveTicket(ctx, tk); err != nil {
		return fail(err)
	}
	return tk, intent, nil
}

func (s *Service) onTicketPaid(ctx context.Context, in payments.Intent) error {
	tk, err := s.store.Ticket(ctx, in.RefID)
	if err != nil {
		return err
	}
	t, err := s.store.Trip(ctx, tk.TripID)
	if err != nil {
		return err
	}
	if err := s.seats.Confirm(ctx, t, tk.PNR, tk.Seats); err != nil {
		_ = s.store.SetTicketStatus(ctx, tk.PNR, "cancelled")
		_, rerr := s.payments.RefundByRef(ctx, "luxury_bus", tk.PNR, 0, "seat hold expired")
		return rerr
	}
	_ = s.store.SetTicketStatus(ctx, tk.PNR, "confirmed")
	orderIDs, _ := s.store.rdb.LRange(ctx, cache.KeyLuxTicketFood(tk.PNR), 0, -1).Result()
	for _, id := range orderIDs {
		if o, err := s.store.Order(ctx, id); err == nil {
			if first, _ := s.store.MarkPaid(ctx, o); first {
				_ = s.store.CreditOwner(ctx, t.OwnerID, o.Total)
			}
		}
	}
	s.notify.Notify(ctx, tk.UserID, "Bus ticket confirmed", "PNR "+tk.PNR+" · "+t.From+" → "+t.To+" · seats "+seatList(tk.Seats), map[string]string{"pnr": tk.PNR},
		notifications.ChannelPush, notifications.ChannelSMS)
	return nil
}

// OrderFood adds meals to a confirmed ticket (before each stop's cut-off).
func (s *Service) OrderFood(ctx context.Context, userID, pnr string, lines []OrderLine) (*FoodOrder, *payments.Intent, error) {
	tk, err := s.store.Ticket(ctx, pnr)
	if err != nil {
		return nil, nil, err
	}
	if tk.UserID != userID || tk.Status != "confirmed" {
		return nil, nil, apperr.Forbidden("not a confirmed ticket of yours")
	}
	total, err := s.store.PriceOrder(ctx, tk.TripID, lines)
	if err != nil {
		return nil, nil, err
	}
	o, err := s.store.CreateOrder(ctx, pnr, tk.TripID, lines, total)
	if err != nil {
		return nil, nil, err
	}
	intent, err := s.payments.CreateIntent(ctx, "luxury_food", o.ID, userID, total)
	return o, intent, err
}

func (s *Service) onFoodPaid(ctx context.Context, in payments.Intent) error {
	o, err := s.store.Order(ctx, in.RefID)
	if err != nil {
		return err
	}
	t, err := s.store.Trip(ctx, o.TripID)
	if err != nil {
		return err
	}
	if first, err := s.store.MarkPaid(ctx, o); err != nil || !first {
		return err
	}
	return s.store.CreditOwner(ctx, t.OwnerID, o.Total)
}

// DispatchFoodOrders (worker, every minute): one hour before the bus reaches a stop,
// the aggregated order is pushed to the restaurant so food is packed when the bus halts.
func (s *Service) DispatchFoodOrders(ctx context.Context) (int, error) {
	max := strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10)
	due, err := s.store.rdb.ZRangeByScore(ctx, cache.KeyLuxDispatching(), &redis.ZRangeBy{Min: "-inf", Max: max, Count: 100}).Result()
	if err != nil {
		return 0, err
	}
	n := 0
	for _, member := range due {
		if removed, _ := s.store.rdb.ZRem(ctx, cache.KeyLuxDispatching(), member).Result(); removed == 0 {
			continue // another worker took it
		}
		tripID, stopID, _ := strings.Cut(member, "|")
		st, err := s.store.Stop(ctx, tripID, stopID)
		if err != nil {
			continue
		}
		items, perPNR, err := s.store.Aggregate(ctx, tripID, stopID)
		if err != nil || len(items) == 0 {
			continue
		}
		_ = s.pub.Restaurant(ctx, st.RestaurantID, websocket.EventFoodOrder, map[string]any{
			"trip_id": tripID, "stop": st.Name, "bus_eta": st.ETA, "halt_min": st.HaltMin, "items": items, "by_pnr": perPNR,
		})
		s.store.rdb.HSet(ctx, cache.KeyLuxStop(tripID, stopID), "dispatched", time.Now().Unix())
		n++
	}
	return n, nil
}

// Cancel a ticket up to 2 h before departure (full refund incl. undelivered food).
func (s *Service) Cancel(ctx context.Context, userID, pnr string) error {
	tk, err := s.store.Ticket(ctx, pnr)
	if err != nil {
		return err
	}
	if tk.UserID != userID {
		return apperr.Forbidden("not your ticket")
	}
	t, err := s.store.Trip(ctx, tk.TripID)
	if err != nil {
		return err
	}
	if time.Until(t.Departure) < 2*time.Hour {
		return apperr.Conflict("cancellation closes 2 hours before departure")
	}
	s.seats.Free(ctx, t, pnr, tk.Seats)
	_ = s.store.SetTicketStatus(ctx, pnr, "cancelled")
	if paid, _ := s.payments.IsPaid(ctx, "luxury_bus", pnr); paid {
		_, err = s.payments.RefundByRef(ctx, "luxury_bus", pnr, 0, "ticket cancelled")
	}
	return err
}

func (s *Service) OwnerEarnings(ctx context.Context, ownerID string) (map[string]string, error) {
	return s.store.OwnerEarnings(ctx, ownerID, utils.MonthKey(time.Now()))
}

func seatList(seats []int) string {
	out := make([]string, len(seats))
	for i, n := range seats {
		out[i] = strconv.Itoa(n + 1)
	}
	return strings.Join(out, ",")
}
