package dispatch

import (
	"net/http"

	"sarathi/internal/common/response"
	"sarathi/internal/common/utils"
)

type Handler struct{ svc *Service }

func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/dispatch/offers/{booking}/accept", h.accept)
	mux.HandleFunc("POST /api/v1/dispatch/offers/{booking}/reject", h.reject)
	mux.HandleFunc("GET /api/v1/dispatch/nearby", h.nearby)
}

func (h *Handler) accept(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Accept(r.Context(), utils.UserID(r), r.PathValue("booking")); err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, map[string]string{"status": "accepted"})
}

func (h *Handler) reject(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Reject(r.Context(), utils.UserID(r), r.PathValue("booking")); err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, map[string]string{"status": "rejected"})
}

// GET /api/v1/dispatch/nearby?class=cab&lat=..&lng=..  (cars around the rider)
func (h *Handler) nearby(w http.ResponseWriter, r *http.Request) {
	class := r.URL.Query().Get("class")
	if class == "" {
		class = "cab"
	}
	p := utils.Point{Lat: utils.QueryFloat(r, "lat", 0), Lng: utils.QueryFloat(r, "lng", 0)}
	c, err := h.svc.Nearby(r.Context(), class, p)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, map[string]any{"count": len(c), "drivers": c})
}
