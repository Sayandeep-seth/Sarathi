package luxury_bus

import (
	"net/http"

	"sarathi/internal/common/response"
	"sarathi/internal/common/utils"
)

type Handler struct{ svc *Service }

func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("PUT /api/v1/luxury-bus/trips", h.createTrip)
	mux.HandleFunc("GET /api/v1/luxury-bus/trips/{id}", h.details)
	mux.HandleFunc("PUT /api/v1/luxury-bus/restaurants/{id}/menu", h.putMenu)
	mux.HandleFunc("POST /api/v1/luxury-bus/book", h.book)
	mux.HandleFunc("POST /api/v1/luxury-bus/tickets/{pnr}/food", h.food)
	mux.HandleFunc("POST /api/v1/luxury-bus/tickets/{pnr}/cancel", h.cancel)
	mux.HandleFunc("GET /api/v1/luxury-bus/owners/{id}/earnings", h.earnings)
}

func (h *Handler) createTrip(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Trip  Trip   `json:"trip"`
		Stops []Stop `json:"stops"`
	}
	if err := response.Decode(r, &body); err != nil {
		response.Error(w, err)
		return
	}
	if err := h.svc.CreateTrip(r.Context(), body.Trip, body.Stops); err != nil {
		response.Error(w, err)
		return
	}
	response.Created(w, body)
}

func (h *Handler) details(w http.ResponseWriter, r *http.Request) {
	d, err := h.svc.Details(r.Context(), r.PathValue("id"))
	if err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, d)
}

func (h *Handler) putMenu(w http.ResponseWriter, r *http.Request) {
	var items []MenuItem
	if err := response.Decode(r, &items); err != nil {
		response.Error(w, err)
		return
	}
	if err := h.svc.store.PutMenu(r.Context(), r.PathValue("id"), items); err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, map[string]int{"items": len(items)})
}

func (h *Handler) book(w http.ResponseWriter, r *http.Request) {
	var req BookRequest
	if err := response.Decode(r, &req); err != nil {
		response.Error(w, err)
		return
	}
	t, intent, err := h.svc.Book(r.Context(), utils.UserID(r), req)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.Created(w, map[string]any{"ticket": t, "payment": intent})
}

func (h *Handler) food(w http.ResponseWriter, r *http.Request) {
	var lines []OrderLine
	if err := response.Decode(r, &lines); err != nil {
		response.Error(w, err)
		return
	}
	o, intent, err := h.svc.OrderFood(r.Context(), utils.UserID(r), r.PathValue("pnr"), lines)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.Created(w, map[string]any{"order": o, "payment": intent})
}

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Cancel(r.Context(), utils.UserID(r), r.PathValue("pnr")); err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, map[string]string{"status": "cancelled"})
}

func (h *Handler) earnings(w http.ResponseWriter, r *http.Request) {
	e, err := h.svc.OwnerEarnings(r.Context(), r.PathValue("id"))
	if err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, e)
}
