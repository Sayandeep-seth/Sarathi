package payments

import (
	"io"
	"net/http"

	"sarathi/internal/common/response"
)

type Handler struct{ svc *Service }

func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/payments/{id}", h.get)
	mux.HandleFunc("POST /api/v1/payments/{id}/confirm", h.confirm)
	mux.HandleFunc("POST /api/v1/payments/{id}/refund", h.refund)
	mux.HandleFunc("POST /api/v1/payments/webhook", h.webhook)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	in, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, in)
}

// POST /api/v1/payments/{id}/confirm {"payment_id":"pay_..","signature":".."} (after checkout)
func (h *Handler) confirm(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PaymentID string `json:"payment_id"`
		Signature string `json:"signature"`
	}
	if err := response.Decode(r, &body); err != nil {
		response.Error(w, err)
		return
	}
	in, err := h.svc.Confirm(r.Context(), r.PathValue("id"), body.PaymentID, body.Signature)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, in)
}

func (h *Handler) refund(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Amount float64 `json:"amount"`
		Reason string  `json:"reason"`
	}
	if err := response.Decode(r, &body); err != nil {
		response.Error(w, err)
		return
	}
	rf, err := h.svc.Refund(r.Context(), r.PathValue("id"), body.Amount, body.Reason)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, rf)
}

func (h *Handler) webhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		response.Error(w, err)
		return
	}
	if err := h.svc.HandleWebhook(r.Context(), body, r.Header.Get("X-Signature")); err != nil {
		response.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}
