package daily_bus

import (
	"net/http"

	"sarathi/internal/common/response"
	"sarathi/internal/common/utils"
)

type Handler struct{ svc *Service }

func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/daily-bus/routes/search", h.search)
	mux.HandleFunc("PUT /api/v1/daily-bus/routes", h.putRoute)
	mux.HandleFunc("GET /api/v1/daily-bus/routes/{id}/seats", h.seats)
	mux.HandleFunc("POST /api/v1/daily-bus/book", h.book)
	mux.HandleFunc("POST /api/v1/daily-bus/tickets/{id}/cancel", h.cancel)
}

func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Home   utils.Point `json:"home"`
		Office utils.Point `json:"office"`
	}
	if err := response.Decode(r, &body); err != nil {
		response.Error(w, err)
		return
	}
	opts, err := h.svc.FindRoutes(r.Context(), body.Home, body.Office)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, map[string]any{"options": opts})
}

func (h *Handler) putRoute(w http.ResponseWriter, r *http.Request) {
	var rt Route
	if err := response.Decode(r, &rt); err != nil {
		response.Error(w, err)
		return
	}
	if err := h.svc.store.PutRoute(r.Context(), rt); err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, rt)
}

// GET /api/v1/daily-bus/routes/{id}/seats?date=20261121
func (h *Handler) seats(w http.ResponseWriter, r *http.Request) {
	m, err := h.svc.SeatMap(r.Context(), r.PathValue("id"), r.URL.Query().Get("date"))
	if err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, map[string]any{"free": m})
}

func (h *Handler) book(w http.ResponseWriter, r *http.Request) {
	var req BookRequest
	if err := response.Decode(r, &req); err != nil {
		response.Error(w, err)
		return
	}
	t, intent, err := h.svc.Book(r.Context(), utils.UserID(r), req)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.Created(w, map[string]any{"ticket": t, "payment": intent})
}

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Cancel(r.Context(), utils.UserID(r), r.PathValue("id")); err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, map[string]string{"status": "cancelled"})
}
