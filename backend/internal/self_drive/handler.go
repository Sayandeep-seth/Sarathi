package self_drive

import (
	"net/http"

	apperr "sarathi/internal/common/errors"
	"sarathi/internal/common/response"
	"sarathi/internal/common/utils"
	"sarathi/internal/pricing"
)

type Handler struct{ svc *Service }

func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/self-drive/bands", h.bands)
	mux.HandleFunc("PUT /api/v1/self-drive/bands", h.setBands)
	mux.HandleFunc("GET /api/v1/self-drive/cars", h.list)
	mux.HandleFunc("PUT /api/v1/self-drive/cars", h.addCar)
	mux.HandleFunc("POST /api/v1/self-drive/reserve", h.reserve)
	mux.HandleFunc("POST /api/v1/self-drive/{booking}/return", h.ret)
}

func (h *Handler) bands(w http.ResponseWriter, r *http.Request) {
	response.OK(w, map[string]any{"bands": h.svc.cache.Bands(r.Context())})
}

func (h *Handler) setBands(w http.ResponseWriter, r *http.Request) {
	var bands []pricing.Band
	if err := response.Decode(r, &bands); err != nil {
		response.Error(w, err)
		return
	}
	if err := h.svc.cache.SetBands(r.Context(), bands); err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, map[string]any{"bands": bands})
}

// GET /api/v1/self-drive/cars?lat=..&lng=..&start=2026-12-01&days=3
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	start, err := utils.ParseDate(r.URL.Query().Get("start"))
	if err != nil {
		response.Error(w, apperr.BadRequest("start=yyyy-mm-dd required"))
		return
	}
	p := utils.Point{Lat: utils.QueryFloat(r, "lat", 0), Lng: utils.QueryFloat(r, "lng", 0)}
	items, err := h.svc.List(r.Context(), p, start, max(utils.QueryInt(r, "days", 1), 1))
	if err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, map[string]any{"cars": items})
}

func (h *Handler) addCar(w http.ResponseWriter, r *http.Request) {
	var c Car
	if err := response.Decode(r, &c); err != nil {
		response.Error(w, err)
		return
	}
	if err := h.svc.AddCar(r.Context(), c); err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, map[string]string{"status": "ok"})
}

func (h *Handler) reserve(w http.ResponseWriter, r *http.Request) {
	var req ReserveRequest
	if err := response.Decode(r, &req); err != nil {
		response.Error(w, err)
		return
	}
	b, err := h.svc.Reserve(r.Context(), utils.UserID(r), req)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.Created(w, b)
}

func (h *Handler) ret(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Damage    float64 `json:"damage"`
		ExtraKm   float64 `json:"extra_km"`
		LateHours int     `json:"late_hours"`
	}
	if err := response.Decode(r, &body); err != nil {
		response.Error(w, err)
		return
	}
	refund, err := h.svc.Return(r.Context(), r.PathValue("booking"), body.Damage, body.ExtraKm, body.LateHours)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, map[string]float64{"deposit_refunded": refund})
}
