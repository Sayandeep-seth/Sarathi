package gps

import (
	"net/http"

	"sarathi/internal/common/response"
)

type Handler struct{ svc *Service }

func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/gps/ping", h.ping)
	mux.HandleFunc("POST /api/v1/gps/batch", h.batch)
	mux.HandleFunc("GET /api/v1/gps/vehicles/{id}", h.last)
	mux.HandleFunc("GET /api/v1/gps/trips/{id}/path", h.path)
}

func (h *Handler) ping(w http.ResponseWriter, r *http.Request) {
	var p Ping
	if err := response.Decode(r, &p); err != nil {
		response.Error(w, err)
		return
	}
	if err := h.svc.Ingest(r.Context(), p); err != nil {
		response.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// batch accepts buffered pings sent after the phone regains network.
func (h *Handler) batch(w http.ResponseWriter, r *http.Request) {
	var ps []Ping
	if err := response.Decode(r, &ps); err != nil {
		response.Error(w, err)
		return
	}
	accepted := 0
	for _, p := range ps {
		if h.svc.Ingest(r.Context(), p) == nil {
			accepted++
		}
	}
	response.OK(w, map[string]int{"accepted": accepted, "received": len(ps)})
}

func (h *Handler) last(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.Last(r.Context(), r.PathValue("id"))
	if err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, p)
}

func (h *Handler) path(w http.ResponseWriter, r *http.Request) {
	path, err := h.svc.TripPath(r.Context(), r.PathValue("id"))
	if err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, map[string]any{"points": path})
}
