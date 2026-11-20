package booking

import (
	"net/http"
	"time"

	"sarathi/internal/common/constants"
	apperr "sarathi/internal/common/errors"
	"sarathi/internal/common/response"
	"sarathi/internal/common/utils"
)

type Handler struct{ svc *Service }

func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/bookings/quote", h.quote)
	mux.HandleFunc("POST /api/v1/bookings/ride", h.ride)
	mux.HandleFunc("GET /api/v1/bookings/{id}", h.get)
	mux.HandleFunc("POST /api/v1/bookings/{id}/cancel", h.cancel)
	mux.HandleFunc("POST /api/v1/bookings/{id}/arrived", h.arrived)
	mux.HandleFunc("POST /api/v1/bookings/{id}/start", h.start)
	mux.HandleFunc("POST /api/v1/bookings/{id}/complete", h.complete)
}

type rideReq struct {
	Pickup      utils.Point `json:"pickup"`
	Drop        utils.Point `json:"drop"`
	Class       string      `json:"class"` // cab (default) | seat5 | seat7
	ScheduledAt *time.Time  `json:"scheduled_at"`
}

func (h *Handler) quote(w http.ResponseWriter, r *http.Request) {
	var req rideReq
	if err := response.Decode(r, &req); err != nil {
		response.Error(w, err)
		return
	}
	if req.Class == "" {
		req.Class = constants.ClassCab
	}
	q, err := h.svc.Quote(r.Context(), req.Class, req.Pickup, req.Drop)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, q)
}

// POST /api/v1/bookings/ride — option 1: cab to any place, no parking fee.
func (h *Handler) ride(w http.ResponseWriter, r *http.Request) {
	var req rideReq
	if err := response.Decode(r, &req); err != nil {
		response.Error(w, err)
		return
	}
	if !req.Drop.Valid() {
		response.Error(w, apperr.BadRequest("drop location required"))
		return
	}
	if req.Class == "" {
		req.Class = constants.ClassCab
	}
	b, err := h.svc.Create(r.Context(), CreateInput{
		UserID: utils.UserID(r), Type: constants.BookingRide, Class: req.Class,
		Pickup: req.Pickup, Drop: req.Drop, ScheduledAt: req.ScheduledAt,
		IdempotencyKey: r.Header.Get("Idempotency-Key"),
	})
	if err != nil {
		response.Error(w, err)
		return
	}
	response.Created(w, b)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	b, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		response.Error(w, err)
		return
	}
	uid := utils.UserID(r)
	if b.UserID != uid && b.DriverID != uid {
		response.Error(w, apperr.Forbidden("not your booking"))
		return
	}
	response.OK(w, b)
}

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Reason string `json:"reason"`
	}
	_ = response.Decode(r, &body)
	if err := h.svc.Cancel(r.Context(), utils.UserID(r), r.PathValue("id"), body.Reason); err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, map[string]string{"state": StateCancelled})
}

func (h *Handler) arrived(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Arrived(r.Context(), utils.UserID(r), r.PathValue("id")); err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, map[string]string{"state": StateArrived})
}

func (h *Handler) start(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Start(r.Context(), utils.UserID(r), r.PathValue("id")); err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, map[string]string{"state": StateInProgress})
}

func (h *Handler) complete(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ActualKm float64 `json:"actual_km"`
	}
	_ = response.Decode(r, &body)
	b, err := h.svc.Complete(r.Context(), utils.UserID(r), r.PathValue("id"), body.ActualKm)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, b)
}
