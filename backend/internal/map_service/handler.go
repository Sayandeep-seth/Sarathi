package map_service

import (
	"net/http"

	"sarathi/internal/common/response"
	"sarathi/internal/common/utils"
)

type Handler struct{ svc *Service }

func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/map/routes", h.routes)
	mux.HandleFunc("GET /api/v1/map/roads/{id}/compat", h.compat)
	mux.HandleFunc("PUT /api/v1/map/roads/{id}", h.reportWidth)
}

// POST /api/v1/map/routes {"origin":{lat,lng},"destination":{lat,lng},"car_name":"Toyota Innova Crysta"}
func (h *Handler) routes(w http.ResponseWriter, r *http.Request) {
	var req RouteRequest
	if err := response.Decode(r, &req); err != nil {
		response.Error(w, err)
		return
	}
	res, err := h.svc.PlanRoutes(r.Context(), utils.UserID(r), req)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, res)
}

func (h *Handler) compat(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.RoadCompat(r.Context(), r.PathValue("id"), r.URL.Query().Get("car"))
	if err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, res)
}

func (h *Handler) reportWidth(w http.ResponseWriter, r *http.Request) {
	var a RoadAttr
	if err := response.Decode(r, &a); err != nil {
		response.Error(w, err)
		return
	}
	a.ID = r.PathValue("id")
	if err := h.svc.ReportRoadWidth(r.Context(), a); err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, map[string]string{"status": "updated"})
}
