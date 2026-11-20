package driver

import (
	"net/http"

	apperr "sarathi/internal/common/errors"
	"sarathi/internal/common/response"
	"sarathi/internal/common/utils"
)

type Handler struct{ svc *Service }

func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/driver/online", h.online)
	mux.HandleFunc("POST /api/v1/driver/offline", h.offline)
	mux.HandleFunc("POST /api/v1/driver/break", h.brk)
	mux.HandleFunc("POST /api/v1/driver/location", h.location)
	mux.HandleFunc("GET /api/v1/driver/me", h.me)
}

func driverID(r *http.Request) (string, error) {
	id := utils.UserID(r)
	if id == "" {
		return "", apperr.Forbidden("driver auth required")
	}
	return id, nil
}

func (h *Handler) online(w http.ResponseWriter, r *http.Request) {
	id, err := driverID(r)
	if err != nil {
		response.Error(w, err)
		return
	}
	var p utils.Point
	if err := response.Decode(r, &p); err != nil {
		response.Error(w, err)
		return
	}
	if err := h.svc.GoOnline(r.Context(), id, p); err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, map[string]string{"status": StatusAvailable})
}

func (h *Handler) offline(w http.ResponseWriter, r *http.Request) {
	id, err := driverID(r)
	if err == nil {
		err = h.svc.GoOffline(r.Context(), id)
	}
	if err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, map[string]string{"status": StatusOffline})
}

func (h *Handler) brk(w http.ResponseWriter, r *http.Request) {
	id, err := driverID(r)
	if err == nil {
		err = h.svc.TakeBreak(r.Context(), id)
	}
	if err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, map[string]string{"status": StatusBreak})
}

func (h *Handler) location(w http.ResponseWriter, r *http.Request) {
	id, err := driverID(r)
	if err != nil {
		response.Error(w, err)
		return
	}
	var p utils.Point
	if err := response.Decode(r, &p); err != nil {
		response.Error(w, err)
		return
	}
	if err := h.svc.UpdateLocation(r.Context(), id, p); err != nil {
		response.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	id, err := driverID(r)
	if err != nil {
		response.Error(w, err)
		return
	}
	st, err := h.svc.State(r.Context(), id)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, st)
}
