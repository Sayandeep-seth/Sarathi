package petrol

import (
	"net/http"

	"sarathi/internal/common/response"
	"sarathi/internal/common/utils"
)

type Handler struct{ svc *Service }

func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/stations/petrol/along-route", h.alongRoute)
	mux.HandleFunc("GET /api/v1/stations/petrol/nearby", h.nearby)
	mux.HandleFunc("GET /api/v1/stations/petrol/{id}/busy-hours", h.busy)
	mux.HandleFunc("PUT /api/v1/stations/petrol", h.upsert)
}

func (h *Handler) alongRoute(w http.ResponseWriter, r *http.Request) {
	var req AlongRouteRequest
	if err := response.Decode(r, &req); err != nil {
		response.Error(w, err)
		return
	}
	res, err := h.svc.AlongRoute(r.Context(), req)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, res)
}

// GET /api/v1/stations/petrol/nearby?lat=..&lng=..&radius_km=3  (sorted best first)
func (h *Handler) nearby(w http.ResponseWriter, r *http.Request) {
	p := utils.Point{Lat: utils.QueryFloat(r, "lat", 0), Lng: utils.QueryFloat(r, "lng", 0)}
	res, err := h.svc.Nearby(r.Context(), p, utils.Clamp(utils.QueryFloat(r, "radius_km", 3), 0.5, 25))
	if err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, map[string]any{"stations": res})
}

func (h *Handler) busy(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.Busy(r.Context(), r.PathValue("id"))
	if err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, map[string]any{"last_24h": res})
}

func (h *Handler) upsert(w http.ResponseWriter, r *http.Request) {
	var st Station
	if err := response.Decode(r, &st); err != nil {
		response.Error(w, err)
		return
	}
	if err := h.svc.Upsert(r.Context(), st); err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, st)
}
