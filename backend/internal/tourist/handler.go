package tourist

import (
	"net/http"

	"sarathi/internal/common/response"
	"sarathi/internal/common/utils"
	"sarathi/internal/pricing"
)

type Handler struct{ svc *Service }

func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/tourist/quote", h.quote)
	mux.HandleFunc("POST /api/v1/tourist/book", h.book)
	mux.HandleFunc("PUT /api/v1/tourist/rates", h.setRates)
	mux.HandleFunc("PUT /api/v1/tourist/cars", h.putCar)
}

func (h *Handler) quote(w http.ResponseWriter, r *http.Request) {
	var req Request
	if err := response.Decode(r, &req); err != nil {
		response.Error(w, err)
		return
	}
	q, err := h.svc.Quote(r.Context(), req)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, q)
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

func (h *Handler) setRates(w http.ResponseWriter, r *http.Request) {
	var rate pricing.DailyRate
	if err := response.Decode(r, &rate); err != nil {
		response.Error(w, err)
		return
	}
	if err := h.svc.cache.SetRates(r.Context(), rate); err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, rate)
}

func (h *Handler) putCar(w http.ResponseWriter, r *http.Request) {
	var c Car
	if err := response.Decode(r, &c); err != nil {
		response.Error(w, err)
		return
	}
	if err := h.svc.cache.PutCar(r.Context(), c); err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, c)
}
