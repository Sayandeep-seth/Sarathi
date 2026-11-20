package ev

import (
	"net/http"

	"sarathi/internal/common/response"
	"sarathi/internal/common/utils"
)

type Handler struct{ svc *Service }

func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/stations/ev/along-route", h.alongRoute)
	mux.HandleFunc("GET /api/v1/stations/ev/nearby", h.nearby)
	mux.HandleFunc("POST /api/v1/stations/ev/{id}/ports", h.ports)
	mux.HandleFunc("PUT /api/v1/stations/ev", h.upsert)
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

func (h *Handler) nearby(w http.ResponseWriter, r *http.Request) {
	p := utils.Point{Lat: utils.QueryFloat(r, "lat", 0), Lng: utils.QueryFloat(r, "lng", 0)}
	res, err := h.svc.Nearby(r.Context(), p, utils.Clamp(utils.QueryFloat(r, "radius_km", 5), 0.5, 50), r.URL.Query().Get("car"))
	if err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, map[string]any{"chargers": res})
}

// POST /api/v1/stations/ev/{id}/ports {"free":2,"total":4}  (operator feed)
func (h *Handler) ports(w http.ResponseWriter, r *http.Request) {
	var body struct{ Free, Total int }
	if err := response.Decode(r, &body); err != nil {
		response.Error(w, err)
		return
	}
	if err := h.svc.UpdatePorts(r.Context(), r.PathValue("id"), body.Free, body.Total); err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, map[string]string{"status": "ok"})
}

func (h *Handler) upsert(w http.ResponseWriter, r *http.Request) {
	var c Charger
	if err := response.Decode(r, &c); err != nil {
		response.Error(w, err)
		return
	}
	if err := h.svc.Upsert(r.Context(), c); err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, c)
}
