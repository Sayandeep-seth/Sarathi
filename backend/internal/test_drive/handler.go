package test_drive

import (
	"net/http"

	"sarathi/internal/common/response"
	"sarathi/internal/common/utils"
)

type Handler struct{ svc *Service }

func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/test-drive/book", h.book)
	mux.HandleFunc("GET /api/v1/test-drive/eligibility", h.eligible)
	mux.HandleFunc("POST /api/v1/test-drive/cab", h.cab)
	mux.HandleFunc("PUT /api/v1/test-drive/cars", h.putCar)
	mux.HandleFunc("PUT /api/v1/test-drive/zones", h.putZone)
	mux.HandleFunc("GET /api/v1/test-drive/dealers/{id}/earnings", h.earnings)
}

func (h *Handler) book(w http.ResponseWriter, r *http.Request) {
	var req TestDriveRequest
	if err := response.Decode(r, &req); err != nil {
		response.Error(w, err)
		return
	}
	b, err := h.svc.BookTestDrive(r.Context(), utils.UserID(r), req)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.Created(w, b)
}

// GET /api/v1/test-drive/eligibility?showroom=..&model=..
func (h *Handler) eligible(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	ok, err := h.svc.eligibility.Eligible(r.Context(), q.Get("showroom"), utils.NormalizeName(q.Get("model")), utils.UserID(r))
	if err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, map[string]bool{"eligible": ok})
}

func (h *Handler) cab(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Pickup utils.Point `json:"pickup"`
		Drop   utils.Point `json:"drop"`
	}
	if err := response.Decode(r, &body); err != nil {
		response.Error(w, err)
		return
	}
	b, err := h.svc.RequestCab(r.Context(), utils.UserID(r), body.Pickup, body.Drop)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.Created(w, b)
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

func (h *Handler) putZone(w http.ResponseWriter, r *http.Request) {
	var z Zone
	if err := response.Decode(r, &z); err != nil {
		response.Error(w, err)
		return
	}
	if err := h.svc.cache.SetZone(r.Context(), z); err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, z)
}

func (h *Handler) earnings(w http.ResponseWriter, r *http.Request) {
	e, err := h.svc.Earnings(r.Context(), r.PathValue("id"))
	if err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, e)
}
