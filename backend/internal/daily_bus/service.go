package daily_bus

import (
	"context"
	"log/slog"
	"sort"
	"strconv"
	"time"

	apperr "sarathi/internal/common/errors"
	"sarathi/internal/common/utils"
	"sarathi/internal/notifications"
	"sarathi/internal/payments"
)

type Service struct {
	store    *Store
	seats    *Seats
	payments *payments.Service
	notify   *notifications.Queue
	log      *slog.Logger
}

func NewService(st *Store, se *Seats, p *payments.Service, n *notifications.Queue, log *slog.Logger) *Service {
	s := &Service{store: st, seats: se, payments: p, notify: n, log: log}
	p.OnPaid("daily_bus", s.onPaid)
	return s
}

// Option is a route that serves home -> office.
type Option struct {
	RouteID      string  `json:"route_id"`
	RouteName    string  `json:"route_name"`
	BoardStop    Stop    `json:"board_stop"`
	DropStop     Stop    `json:"drop_stop"`
	WalkToBoardM int     `json:"walk_to_board_m"`
	WalkFromDrop int     `json:"walk_from_drop_m"` // 0-100 m when the bus stops at the office gate
	DepartTime   string  `json:"depart_time"`
	Fare         float64 `json:"fare"`
	MonthlyPass  float64 `json:"monthly_pass"`
	SeatsLeft    int     `json:"seats_left_tomorrow"`
}

// FindRoutes matches stops within 1 km of home and 500 m of the office on the same route, in order.
func (s *Service) FindRoutes(ctx context.Context, home, office utils.Point) ([]Option, error) {
	if !home.Valid() || !office.Valid() {
		return nil, apperr.BadRequest("home and office locations required")
	}
	boards, err := s.store.StopsNear(ctx, home, 1.0)
	if err != nil {
		return nil, err
	}
	drops, err := s.store.StopsNear(ctx, office, 0.5)
	if err != nil {
		return nil, err
	}
	tomorrow := utils.DateKey(time.Now().AddDate(0, 0, 1))
	var out []Option
	for _, b := range boards {
		for _, d := range drops {
			if b.RouteID != d.RouteID {
				continue
			}
			r, err := s.store.Route(ctx, b.RouteID)
			if err != nil {
				continue
			}
			from, ok1 := r.Stop(b.StopID)
			to, ok2 := r.Stop(d.StopID)
			if !ok1 || !ok2 || from.Seq >= to.Seq {
				continue
			}
			left := 0
			if m, err := s.seats.Map(ctx, r.ID, tomorrow, r.BusSeats); err == nil {
				for _, f := range m {
					if f {
						left++
					}
				}
			}
			fare := r.Fare(from, to)
			out = append(out, Option{
				RouteID: r.ID, RouteName: r.Name, BoardStop: from, DropStop: to,
				WalkToBoardM: int(b.WalkKm * 1000), WalkFromDrop: int(d.WalkKm * 1000),
				DepartTime: r.DepartTime, Fare: fare, MonthlyPass: MonthlyPrice(fare, 22), SeatsLeft: left,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].WalkToBoardM+out[i].WalkFromDrop < out[j].WalkToBoardM+out[j].WalkFromDrop
	})
	return out, nil
}

// BookRequest for a day ticket or a monthly pass.
type BookRequest struct {
	RouteID  string `json:"route_id"`
	FromStop string `json:"from_stop"`
	ToStop   string `json:"to_stop"`
	Date     string `json:"date"` // yyyy-mm-dd; for monthly = first day
	Monthly  bool   `json:"monthly"`
}

// Book holds a guaranteed seat (same seat every day for monthly) and opens a payment.
func (s *Service) Book(ctx context.Context, userID string, req BookRequest) (*Ticket, *payments.Intent, error) {
	r, err := s.store.Route(ctx, req.RouteID)
	if err != nil {
		return nil, nil, err
	}
	from, ok1 := r.Stop(req.FromStop)
	to, ok2 := r.Stop(req.ToStop)
	if !ok1 || !ok2 || from.Seq >= to.Seq {
		return nil, nil, apperr.BadRequest("invalid stops for this route")
	}
	day, err := utils.ParseDate(req.Date)
	if err != nil {
		return nil, nil, apperr.BadRequest("date yyyy-mm-dd required")
	}
	t := &Ticket{ID: utils.NewID("dbt"), UserID: userID, RouteID: r.ID, FromStop: from.ID, ToStop: to.ID, Status: "pending_payment"}
	single := r.Fare(from, to)
	if req.Monthly {
		t.Kind, t.Dates = KindMonthly, WorkingDays(day)
		t.Fare = MonthlyPrice(single, len(t.Dates))
	} else {
		t.Kind, t.Dates, t.Fare = KindDay, []string{utils.DateKey(day)}, single
	}
	seat, err := s.seats.PickSameSeat(ctx, r.ID, t.Dates, r.BusSeats)
	if err != nil {
		return nil, nil, err
	}
	if err := s.seats.HoldAll(ctx, r.ID, t.ID, seat, t.Dates, r.BusSeats); err != nil {
		return nil, nil, err
	}
	t.Seat = seat
	intent, err := s.payments.CreateIntent(ctx, "daily_bus", t.ID, userID, t.Fare)
	if err != nil {
		s.seats.FreeAll(ctx, r.ID, t.ID, seat, t.Dates, r.BusSeats)
		return nil, nil, err
	}
	t.PaymentID = intent.ID
	if err := s.store.SaveTicket(ctx, t); err != nil {
		return nil, nil, err
	}
	return t, intent, nil
}

func (s *Service) onPaid(ctx context.Context, in payments.Intent) error {
	t, err := s.store.Ticket(ctx, in.RefID)
	if err != nil {
		return err
	}
	r, err := s.store.Route(ctx, t.RouteID)
	if err != nil {
		return err
	}
	if err := s.seats.ConfirmAll(ctx, r.ID, t.ID, t.Seat, t.Dates, r.BusSeats); err != nil {
		s.log.Warn("bus seat hold expired before payment", "ticket", t.ID, "err", err)
		_ = s.store.SetTicketStatus(ctx, t.ID, "cancelled")
		_, rerr := s.payments.RefundByRef(ctx, "daily_bus", t.ID, 0, "seat no longer available")
		return rerr
	}
	_ = s.store.SetTicketStatus(ctx, t.ID, "confirmed")
	s.notify.Notify(ctx, t.UserID, "Seat confirmed", "Seat "+strconv.Itoa(t.Seat+1)+" on "+r.Name+" departs "+r.DepartTime+".", map[string]string{"ticket": t.ID})
	return nil
}

// Cancel frees the seat for the remaining days and refunds those days.
func (s *Service) Cancel(ctx context.Context, userID, ticketID string) error {
	t, err := s.store.Ticket(ctx, ticketID)
	if err != nil {
		return err
	}
	if t.UserID != userID {
		return apperr.Forbidden("not your ticket")
	}
	r, err := s.store.Route(ctx, t.RouteID)
	if err != nil {
		return err
	}
	today := utils.DateKey(time.Now())
	var future []string
	for _, d := range t.Dates {
		if d > today {
			future = append(future, d)
		}
	}
	if len(future) == 0 {
		return apperr.Conflict("nothing left to cancel")
	}
	s.seats.FreeAll(ctx, r.ID, t.ID, t.Seat, future, r.BusSeats)
	_ = s.store.SetTicketStatus(ctx, t.ID, "cancelled")
	refund := t.Fare * float64(len(future)) / float64(len(t.Dates))
	_, err = s.payments.RefundByRef(ctx, "daily_bus", t.ID, refund, "cancelled")
	return err
}

func (s *Service) SeatMap(ctx context.Context, routeID, date string) ([]bool, error) {
	r, err := s.store.Route(ctx, routeID)
	if err != nil {
		return nil, err
	}
	return s.seats.Map(ctx, routeID, date, r.BusSeats)
}
