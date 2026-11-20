package shared_ride

import (
	"net/http"

	"sarathi/internal/common/response"
	"sarathi/internal/common/utils"
)

type Handler struct{ svc *Service }

func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/shared/request", h.request)
	mux.HandleFunc("GET /api/v1/shared/{trip}", h.get)
	mux.HandleFunc("POST /api/v1/shared/{trip}/leave", h.leave)
	mux.HandleFunc("POST /api/v1/shared/{trip}/close", h.close)
}

func (h *Handler) request(w http.ResponseWriter, r *http.Request) {
	var req Request
	if err := response.Decode(r, &req); err != nil {
		response.Error(w, err)
		return
	}
	res, err := h.svc.Request(r.Context(), utils.UserID(r), req)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.Created(w, res)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	t, err := h.svc.Get(r.Context(), r.PathValue("trip"))
	if err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, t)
}

func (h *Handler) leave(w http.ResponseWriter, r *http.Request) {
	var body struct {
		BookingID string `json:"booking_id"`
		Seats     int    `json:"seats"`
	}
	if err := response.Decode(r, &body); err != nil {
		response.Error(w, err)
		return
	}
	if body.Seats <= 0 {
		body.Seats = 1
	}
	if err := h.svc.Leave(r.Context(), utils.UserID(r), r.PathValue("trip"), body.BookingID, body.Seats); err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, map[string]string{"status": "left"})
}

// driver starts the pool: no more riders can join
func (h *Handler) close(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Close(r.Context(), r.PathValue("trip")); err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, map[string]string{"status": "closed"})
}
