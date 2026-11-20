package vehicles

import (
	"net/http"

	"sarathi/internal/common/response"
	"sarathi/internal/common/utils"
)

type Handler struct{ svc *Service }

func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/vehicles/specs/{model}", h.get)
	mux.HandleFunc("GET /api/v1/vehicles/suggest", h.suggest)
	mux.HandleFunc("PUT /api/v1/vehicles/specs", h.upsert)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	spec, err := h.svc.GetSpec(r.Context(), r.PathValue("model"))
	if err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, spec)
}

func (h *Handler) suggest(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Suggest(r.Context(), r.URL.Query().Get("q"), utils.QueryInt(r, "limit", 10))
	if err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, map[string]any{"models": out})
}

func (h *Handler) upsert(w http.ResponseWriter, r *http.Request) {
	var s Spec
	if err := response.Decode(r, &s); err != nil {
		response.Error(w, err)
		return
	}
	if err := h.svc.Upsert(r.Context(), &s); err != nil {
		response.Error(w, err)
		return
	}
	response.OK(w, s)
}
