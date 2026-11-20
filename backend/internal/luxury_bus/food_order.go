package luxury_bus

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"sarathi/internal/cache"
	apperr "sarathi/internal/common/errors"
	"sarathi/internal/common/utils"
)

// OrderLine is one dish for one stop.
type OrderLine struct {
	StopID string `json:"stop_id"`
	ItemID string `json:"item_id"`
	Qty    int    `json:"qty"`
}

// FoodOrder groups lines of one PNR.
type FoodOrder struct {
	ID     string      `json:"id"`
	PNR    string      `json:"pnr"`
	TripID string      `json:"trip_id"`
	Lines  []OrderLine `json:"lines"`
	Total  float64     `json:"total"`
	Status string      `json:"status"` // pending_payment | paid | dispatched
}

// PriceOrder validates lines against menus and the per-stop cut-off.
func (s *Store) PriceOrder(ctx context.Context, tripID string, lines []OrderLine) (float64, error) {
	total := 0.0
	menus := map[string]map[string]MenuItem{}
	for _, l := range lines {
		if l.Qty <= 0 || l.Qty > 10 {
			return 0, apperr.BadRequest("quantity must be 1..10")
		}
		st, err := s.Stop(ctx, tripID, l.StopID)
		if err != nil {
			return 0, err
		}
		if time.Until(st.ETA) < OrderCutoff {
			return 0, apperr.Conflict("ordering for " + st.Name + " has closed")
		}
		menu, ok := menus[st.RestaurantID]
		if !ok {
			if menu, err = s.Menu(ctx, st.RestaurantID); err != nil {
				return 0, err
			}
			menus[st.RestaurantID] = menu
		}
		item, ok := menu[l.ItemID]
		if !ok {
			return 0, apperr.BadRequest("item " + l.ItemID + " not on the menu at " + st.Name)
		}
		total += item.Price * float64(l.Qty)
	}
	return total, nil
}

// CreateOrder stores a pending order.
func (s *Store) CreateOrder(ctx context.Context, pnr, tripID string, lines []OrderLine, total float64) (*FoodOrder, error) {
	o := &FoodOrder{ID: utils.NewID("fo"), PNR: pnr, TripID: tripID, Lines: lines, Total: total, Status: "pending_payment"}
	b, _ := json.Marshal(lines)
	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, cache.KeyLuxFoodOrder(o.ID), "pnr", pnr, "trip", tripID, "lines", b, "total", total, "status", o.Status)
	pipe.Expire(ctx, cache.KeyLuxFoodOrder(o.ID), cache.TTLLuxTrip)
	pipe.RPush(ctx, cache.KeyLuxTicketFood(pnr), o.ID)
	pipe.Expire(ctx, cache.KeyLuxTicketFood(pnr), cache.TTLLuxTrip)
	_, err := pipe.Exec(ctx)
	return o, err
}

func (s *Store) Order(ctx context.Context, id string) (*FoodOrder, error) {
	m, err := s.rdb.HGetAll(ctx, cache.KeyLuxFoodOrder(id)).Result()
	if err != nil {
		return nil, err
	}
	if len(m) == 0 {
		return nil, apperr.NotFound("order not found")
	}
	o := &FoodOrder{ID: id, PNR: m["pnr"], TripID: m["trip"], Total: cache.HGetFloat(m, "total"), Status: m["status"]}
	_ = json.Unmarshal([]byte(m["lines"]), &o.Lines)
	return o, nil
}

// MarkPaid flips pending -> paid once (HSETNX on paid_at) and adds quantities to
// the per-stop aggregate the restaurant will prepare.
func (s *Store) MarkPaid(ctx context.Context, o *FoodOrder) (bool, error) {
	first, err := s.rdb.HSetNX(ctx, cache.KeyLuxFoodOrder(o.ID), "paid_at", time.Now().Unix()).Result()
	if err != nil || !first {
		return false, err
	}
	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, cache.KeyLuxFoodOrder(o.ID), "status", "paid")
	for _, l := range o.Lines {
		agg := cache.KeyLuxFoodAgg(o.TripID, l.StopID)
		pipe.HIncrBy(ctx, agg, l.ItemID, int64(l.Qty))
		pipe.HIncrBy(ctx, agg, "seat_orders:"+o.PNR, int64(l.Qty))
		pipe.Expire(ctx, agg, cache.TTLLuxTrip)
	}
	_, err = pipe.Exec(ctx)
	return err == nil, err
}

// Aggregate returns item -> qty for a stop (what the restaurant cooks).
func (s *Store) Aggregate(ctx context.Context, tripID, stopID string) (map[string]int, map[string]int, error) {
	m, err := s.rdb.HGetAll(ctx, cache.KeyLuxFoodAgg(tripID, stopID)).Result()
	if err != nil {
		return nil, nil, err
	}
	items, perPNR := map[string]int{}, map[string]int{}
	for k, v := range m {
		n, _ := strconv.Atoi(v)
		if pnr, ok := strings.CutPrefix(k, "seat_orders:"); ok {
			perPNR[pnr] = n
		} else {
			items[k] = n
		}
	}
	return items, perPNR, nil
}
