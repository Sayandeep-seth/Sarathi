package pickup_return

import (
	"net/http"

	"sarathi/internal/common/response"
	"sarathi/internal/common/utils"
)

type Handler struct{ svc *Service }

func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/pickup-return/quote", h.quote)
	mux.HandleFunc("POST /api/v1/pickup-return/book", h.book)
	mux.HandleFunc("PUT /api/v1/pickup-return/hubs", h.upsertHub)
}

func (h *Handler) quote(w http.ResponseWriter, r *http.Request) {
	var req Request
	if err := response.Decode(r, &req); err != nil {
		response.Error(w, err)
		return
	}
	fare, hub, err := h.svc.Quote(r.Context(), req)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, map[string]any{"fare": fare, "hub": hub})
}

func (h *Handler) book(w http.ResponseWriter, r *http.Request) {
	var req Request
	if err := response.Decode(r, &req); err != nil {
		response.Error(w, err)
		return
	}
	b, err := h.svc.Book(r.Context(), utils.UserID(r), req)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.Created(w, b)
}

func (h *Handler) upsertHub(w http.ResponseWriter, r *http.Request) {
	var hub Hub
	if err := response.Decode(r, &hub); err != nil {
		response.Error(w, err)
		return
	}
	if err := h.svc.parking.UpsertHub(r.Context(), hub); err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, hub)
}
