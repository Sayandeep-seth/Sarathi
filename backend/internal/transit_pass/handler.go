package transit_pass

import (
	"net/http"

	"sarathi/internal/common/response"
	"sarathi/internal/common/utils"
)

type Handler struct{ svc *Service }

func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/transit/routes", h.routes)
	mux.HandleFunc("PUT /api/v1/transit/routes", h.saveRoute)
	mux.HandleFunc("DELETE /api/v1/transit/routes/{name}", h.deleteRoute)
	mux.HandleFunc("POST /api/v1/transit/routes/{name}/start", h.start)
	mux.HandleFunc("GET /api/v1/transit/journeys/{id}", h.journey)
	mux.HandleFunc("GET /api/v1/transit/journeys/{id}/qr", h.qr)
	mux.HandleFunc("POST /api/v1/transit/scan", h.scan)
}

func (h *Handler) routes(w http.ResponseWriter, r *http.Request) {
	rs, err := h.svc.Routes(r.Context(), utils.UserID(r))
	if err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, map[string]any{"routes": rs})
}

func (h *Handler) saveRoute(w http.ResponseWriter, r *http.Request) {
	var rt SavedRoute
	if err := response.Decode(r, &rt); err != nil {
		response.Error(w, err)
		return
	}
	if err := h.svc.SaveRoute(r.Context(), utils.UserID(r), rt); err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, rt)
}

func (h *Handler) deleteRoute(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteRoute(r.Context(), utils.UserID(r), r.PathValue("name")); err != nil {
		response.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/v1/transit/routes/{name}/start -> journey + payment intent for the whole fare
func (h *Handler) start(w http.ResponseWriter, r *http.Request) {
	j, intent, err := h.svc.Start(r.Context(), utils.UserID(r), r.PathValue("name"))
	if err != nil {
		response.Error(w, err)
		return
	}
	response.Created(w, map[string]any{"journey": j, "payment": intent})
}

func (h *Handler) journey(w http.ResponseWriter, r *http.Request) {
	j, err := h.svc.Journey(r.Context(), utils.UserID(r), r.PathValue("id"))
	if err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, j)
}

// GET /api/v1/transit/journeys/{id}/qr  (?format=png returns the image)
func (h *Handler) qr(w http.ResponseWriter, r *http.Request) {
	q, err := h.svc.QR(r.Context(), utils.UserID(r), r.PathValue("id"))
	if err != nil {
		response.Error(w, err)
		return
	}
	if r.URL.Query().Get("format") == "png" {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(q.PNG)
		return
	}
	response.OK(w, q)
}

func (h *Handler) scan(w http.ResponseWriter, r *http.Request) {
	var req ScanRequest
	if err := response.Decode(r, &req); err != nil {
		response.Error(w, err)
		return
	}
	res, err := h.svc.Scan(r.Context(), req)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, res)
}
