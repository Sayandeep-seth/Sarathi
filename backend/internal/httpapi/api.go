package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sarathi/sarathi/backend/internal/auth"
	"github.com/sarathi/sarathi/backend/internal/config"
	"github.com/sarathi/sarathi/backend/internal/core"
	"github.com/sarathi/sarathi/backend/internal/geo"
	"github.com/sarathi/sarathi/backend/internal/live"
	"github.com/sarathi/sarathi/backend/internal/realtime"
	"github.com/sarathi/sarathi/backend/internal/store"
	"github.com/sarathi/sarathi/backend/internal/telemetry"
)

// API holds the dependencies of the product endpoints.
type API struct {
	Cfg    config.Config
	Log    *slog.Logger
	Core   *core.Core
	Store  *store.Store
	Live   *live.Live
	Tel    *telemetry.Store
	Tokens auth.Tokens
	Sender auth.Sender
	Hub    *realtime.Hub
}

type ctxKey int

const identityKey ctxKey = 1

const refreshCookie = "sarathi_rt"

var (
	uuidRE = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	idemRE = regexp.MustCompile(`^[A-Za-z0-9._:-]{8,64}$`)
)

// routes registers every product endpoint on mux.
func (a *API) routes(mux *http.ServeMux) {
	pub := func(h http.HandlerFunc) http.Handler { return a.limitIP(h) }
	user := func(h http.HandlerFunc) http.Handler { return a.limitIP(a.requireRole(h)) }
	driver := func(h http.HandlerFunc) http.Handler { return a.limitIP(a.requireRole(h, "driver", "admin")) }
	admin := func(h http.HandlerFunc) http.Handler { return a.limitIP(a.requireRole(h, "admin")) }

	mux.Handle("POST /api/v1/auth/otp/request", pub(a.otpRequest))
	mux.Handle("POST /api/v1/auth/otp/verify", pub(a.otpVerify))
	mux.Handle("POST /api/v1/auth/refresh", pub(a.refresh))
	mux.Handle("POST /api/v1/auth/logout", pub(a.logout))
	mux.Handle("GET /api/v1/city", pub(a.city))

	mux.Handle("GET /api/v1/me", user(a.me))
	mux.Handle("PATCH /api/v1/me", user(a.updateMe))
	mux.Handle("GET /api/v1/cabs", user(a.cabs))
	mux.Handle("POST /api/v1/trips/estimate", user(a.estimate))
	mux.Handle("POST /api/v1/trips", user(a.bookTrip))
	mux.Handle("GET /api/v1/trips", user(a.listTrips))
	mux.Handle("GET /api/v1/trips/{id}", user(a.getTrip))
	mux.Handle("GET /api/v1/trips/{id}/timeline", user(a.tripTimeline))
	mux.Handle("POST /api/v1/trips/{id}/cancel", user(a.cancelTrip))
	mux.Handle("GET /api/v1/wifi/plans", user(a.plans))
	mux.Handle("GET /api/v1/wifi/quota", user(a.quota))
	mux.Handle("POST /api/v1/wifi/purchase", user(a.purchase))
	mux.Handle("POST /api/v1/ws/ticket", user(a.wsTicket))

	mux.Handle("GET /api/v1/driver/cab", driver(a.driverCab))
	mux.Handle("POST /api/v1/driver/availability", driver(a.driverAvailability))
	mux.Handle("POST /api/v1/driver/location", driver(a.driverLocation))
	mux.Handle("POST /api/v1/driver/trips/{id}/start", driver(a.driverStart))
	mux.Handle("POST /api/v1/driver/trips/{id}/complete", driver(a.driverComplete))

	mux.Handle("GET /api/v1/admin/stats", admin(a.adminStats))
	mux.Handle("POST /api/v1/admin/cabs", admin(a.adminCreateCab))

	mux.Handle("POST /internal/v1/gateway/usage", pub(a.gatewayUsage))
}

// ---------- helpers ----------

// decode reads exactly one JSON object, rejecting unknown fields and trailing data.
func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			WriteError(w, r, http.StatusRequestEntityTooLarge, "payload_too_large", "request body is too large")
		} else {
			WriteError(w, r, http.StatusBadRequest, "invalid_json", "request body must be a valid JSON object")
		}
		return false
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		WriteError(w, r, http.StatusBadRequest, "invalid_json", "request body must contain a single JSON object")
		return false
	}
	return true
}

func (a *API) fail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		WriteError(w, r, http.StatusNotFound, "not_found", "resource not found")
	case errors.Is(err, store.ErrInvalidState):
		WriteError(w, r, http.StatusConflict, "invalid_state", "the trip is not in a state that allows this")
	case errors.Is(err, context.DeadlineExceeded):
		WriteError(w, r, http.StatusGatewayTimeout, "timeout", "the request took too long")
	default:
		a.Log.Error("request_failed", slog.String("path", r.URL.Path), slog.String("error", err.Error()))
		WriteError(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func identity(r *http.Request) auth.Identity {
	id, _ := r.Context().Value(identityKey).(auth.Identity)
	return id
}

func (a *API) clientIP(r *http.Request) string {
	if a.Cfg.TrustProxy {
		if ip := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); ip != nil {
			return ip.String()
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// allow applies a rate limit. failClosed decides what happens when Redis is down.
func (a *API) allow(w http.ResponseWriter, r *http.Request, bucket string, limit int, window time.Duration, failClosed bool) bool {
	ok, retry, err := a.Live.Allow(r.Context(), bucket, limit, window)
	if err != nil {
		a.Log.Warn("rate_limit_unavailable", slog.String("error", err.Error()))
		if failClosed {
			WriteError(w, r, http.StatusServiceUnavailable, "unavailable", "service temporarily unavailable")
			return false
		}
		return true
	}
	if !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
		WriteError(w, r, http.StatusTooManyRequests, "rate_limited", "too many requests, slow down")
		return false
	}
	return true
}

// limitIP is the coarse per-IP limiter in front of every product endpoint.
func (a *API) limitIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.allow(w, r, "ip:"+a.clientIP(r), 600, time.Minute, false) {
			next.ServeHTTP(w, r)
		}
	})
}

// requireRole authenticates the bearer token and, if roles are given, authorizes.
func (a *API) requireRole(next http.HandlerFunc, roles ...string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok {
			WriteError(w, r, http.StatusUnauthorized, "unauthenticated", "missing bearer token")
			return
		}
		id, err := a.Tokens.Parse(raw)
		if err != nil {
			WriteError(w, r, http.StatusUnauthorized, "unauthenticated", "invalid or expired token")
			return
		}
		if len(roles) > 0 {
			allowed := false
			for _, role := range roles {
				allowed = allowed || role == id.Role
			}
			if !allowed {
				WriteError(w, r, http.StatusForbidden, "forbidden", "you do not have access to this resource")
				return
			}
		}
		next(w, r.WithContext(context.WithValue(r.Context(), identityKey, id)))
	})
}

func pathID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if !uuidRE.MatchString(id) {
		WriteError(w, r, http.StatusNotFound, "not_found", "resource not found")
		return "", false
	}
	return id, true
}

// ---------- auth ----------

func (a *API) otpRequest(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Phone string `json:"phone"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !auth.ValidPhone(in.Phone) {
		WriteError(w, r, http.StatusBadRequest, "invalid_phone", "phone must be in E.164 format, e.g. +919876543210")
		return
	}
	if !a.allow(w, r, "otp-ip:"+a.clientIP(r), 10, 10*time.Minute, true) ||
		!a.allow(w, r, "otp-phone:"+in.Phone, 3, 10*time.Minute, true) {
		return
	}
	code, err := a.Live.NewOTP(r.Context(), in.Phone)
	if err == nil {
		err = a.Sender.Send(r.Context(), in.Phone, code)
	}
	if err != nil {
		a.Log.Error("otp_send_failed", slog.String("error", err.Error()))
		WriteError(w, r, http.StatusServiceUnavailable, "otp_unavailable", "could not send the verification code, try again")
		return
	}
	out := map[string]any{"expires_in": live.OTPTTLSeconds()}
	if a.Cfg.OTPDevEcho() {
		out["dev_code"] = code // local development only; never set outside SARATHI_ENV=local
	}
	WriteJSON(w, http.StatusOK, out)
}

type session struct {
	AccessToken  string     `json:"access_token"`
	ExpiresIn    int        `json:"expires_in"`
	RefreshToken string     `json:"refresh_token,omitempty"`
	User         store.User `json:"user"`
}

// issue creates the token pair. Browsers get the refresh token as an HttpOnly,
// SameSite=Strict cookie scoped to the auth endpoints; mobile clients get it in the body.
func (a *API) issue(w http.ResponseWriter, u store.User, refresh string, mobile bool) (session, error) {
	access, err := a.Tokens.Issue(u.ID, u.Role)
	if err != nil {
		return session{}, err
	}
	s := session{AccessToken: access, ExpiresIn: int(a.Cfg.AccessTokenTTL.Seconds()), User: u}
	if mobile {
		s.RefreshToken = refresh
	} else {
		a.setRefreshCookie(w, refresh, int(a.Cfg.RefreshTokenTTL.Seconds()))
	}
	return s, nil
}

func (a *API) setRefreshCookie(w http.ResponseWriter, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name: refreshCookie, Value: value, Path: "/api/v1/auth", MaxAge: maxAge,
		HttpOnly: true, Secure: a.Cfg.Env != "local", SameSite: http.SameSiteStrictMode,
	})
}

func (a *API) otpVerify(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Phone  string `json:"phone"`
		Code   string `json:"code"`
		Name   string `json:"name"`
		Client string `json:"client"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if !auth.ValidPhone(in.Phone) || len(in.Code) != 6 || len(in.Name) > 80 {
		WriteError(w, r, http.StatusBadRequest, "invalid_request", "phone, 6-digit code and an optional name (max 80 chars) are required")
		return
	}
	if !a.allow(w, r, "verify-ip:"+a.clientIP(r), 30, 10*time.Minute, true) {
		return
	}
	switch err := a.Live.VerifyOTP(r.Context(), in.Phone, in.Code); {
	case errors.Is(err, live.ErrOTPInvalid), errors.Is(err, live.ErrOTPExpired):
		WriteError(w, r, http.StatusUnauthorized, "otp_invalid", "the code is incorrect or has expired")
		return
	case errors.Is(err, live.ErrOTPLocked):
		WriteError(w, r, http.StatusTooManyRequests, "otp_locked", "too many attempts, request a new code")
		return
	case err != nil:
		a.fail(w, r, err)
		return
	}
	isAdmin := false
	for _, p := range a.Cfg.AdminPhones {
		isAdmin = isAdmin || p == in.Phone
	}
	u, err := a.Store.UpsertUserByPhone(r.Context(), in.Phone, in.Name, isAdmin)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	refresh, err := auth.NewRefreshToken()
	if err == nil {
		err = a.Store.SaveRefreshToken(r.Context(), u.ID, refresh, time.Now().Add(a.Cfg.RefreshTokenTTL))
	}
	if err != nil {
		a.fail(w, r, err)
		return
	}
	s, err := a.issue(w, u, refresh, in.Client == "mobile")
	if err != nil {
		a.fail(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, s)
}

// presentedRefresh returns the refresh token from the JSON body (mobile) or cookie (web).
func presentedRefresh(r *http.Request) (token string, mobile bool) {
	var in struct {
		RefreshToken string `json:"refresh_token"`
	}
	if r.ContentLength != 0 {
		_ = json.NewDecoder(r.Body).Decode(&in)
	}
	if in.RefreshToken != "" {
		return in.RefreshToken, true
	}
	if c, err := r.Cookie(refreshCookie); err == nil {
		return c.Value, false
	}
	return "", false
}

func (a *API) refresh(w http.ResponseWriter, r *http.Request) {
	old, mobile := presentedRefresh(r)
	if old == "" {
		WriteError(w, r, http.StatusUnauthorized, "unauthenticated", "no refresh token presented")
		return
	}
	next, err := auth.NewRefreshToken()
	if err != nil {
		a.fail(w, r, err)
		return
	}
	u, err := a.Store.RotateRefreshToken(r.Context(), old, next, time.Now().Add(a.Cfg.RefreshTokenTTL))
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrTokenReused) {
		if errors.Is(err, store.ErrTokenReused) {
			a.Log.Warn("refresh_token_reuse", slog.String("user_id", u.ID))
		}
		if !mobile {
			a.setRefreshCookie(w, "", -1)
		}
		WriteError(w, r, http.StatusUnauthorized, "unauthenticated", "session expired, sign in again")
		return
	}
	if err != nil {
		a.fail(w, r, err)
		return
	}
	s, err := a.issue(w, u, next, mobile)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, s)
}

func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	if tok, _ := presentedRefresh(r); tok != "" {
		if err := a.Store.RevokeRefreshToken(r.Context(), tok); err != nil {
			a.fail(w, r, err)
			return
		}
	}
	a.setRefreshCookie(w, "", -1)
	WriteJSON(w, http.StatusOK, map[string]string{"status": "signed_out"})
}

func (a *API) me(w http.ResponseWriter, r *http.Request) {
	u, err := a.Store.User(r.Context(), identity(r).UserID)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, u)
}

func (a *API) updateMe(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len(in.Name) > 80 {
		WriteError(w, r, http.StatusBadRequest, "invalid_name", "name must be 1-80 characters")
		return
	}
	u, err := a.Store.UpdateUserName(r.Context(), identity(r).UserID, in.Name)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, u)
}

// ---------- city & fleet ----------

func (a *API) city(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=300")
	WriteJSON(w, http.StatusOK, map[string]any{
		"name":   "Bengaluru (stylised grid)",
		"origin": map[string]float64{"lat": geo.OriginLat, "lng": geo.OriginLng},
		"grid":   map[string]any{"size": geo.Size, "spacing_m": geo.Spacing},
		"places": geo.Places,
	})
}

func (a *API) cabs(w http.ResponseWriter, r *http.Request) {
	cabs, err := a.Store.OnlineCabs(r.Context())
	if err != nil {
		a.fail(w, r, err)
		return
	}
	// Redis has fresher positions than the throttled PostgreSQL copy.
	if livePos, err := a.Live.LiveCabs(r.Context()); err == nil {
		for i := range cabs {
			if p, ok := livePos[cabs[i].ID]; ok {
				cabs[i].Lat, cabs[i].Lng, cabs[i].Heading = &p.Lat, &p.Lng, p.Heading
			}
		}
	}
	WriteJSON(w, http.StatusOK, cabs)
}

// ---------- trips ----------

type placesIn struct {
	Pickup string `json:"pickup_place_id"`
	Drop   string `json:"drop_place_id"`
}

func (a *API) estimate(w http.ResponseWriter, r *http.Request) {
	var in placesIn
	if !decode(w, r, &in) {
		return
	}
	from, ok1 := geo.PlaceByID(in.Pickup)
	to, ok2 := geo.PlaceByID(in.Drop)
	if !ok1 || !ok2 || from.ID == to.ID {
		WriteError(w, r, http.StatusBadRequest, "invalid_places", "pickup and drop must be two different known places")
		return
	}
	WriteJSON(w, http.StatusOK, geo.Plan(from, to))
}

func (a *API) bookTrip(w http.ResponseWriter, r *http.Request) {
	var in placesIn
	if !decode(w, r, &in) {
		return
	}
	idem := r.Header.Get("Idempotency-Key")
	if idem != "" && !idemRE.MatchString(idem) {
		WriteError(w, r, http.StatusBadRequest, "invalid_idempotency_key", "Idempotency-Key must be 8-64 URL-safe characters")
		return
	}
	uid := identity(r).UserID
	if !a.allow(w, r, "book:"+uid, 10, time.Minute, false) {
		return
	}
	trip, replayed, err := a.Core.Book(r.Context(), uid, in.Pickup, in.Drop, idem)
	switch {
	case errors.Is(err, core.ErrUnknownPlace), errors.Is(err, core.ErrSamePlace):
		WriteError(w, r, http.StatusBadRequest, "invalid_places", "pickup and drop must be two different known places")
	case errors.Is(err, store.ErrNoCab):
		WriteError(w, r, http.StatusServiceUnavailable, "no_cabs_available", "no cabs are available right now, try again shortly")
	case errors.Is(err, store.ErrActiveTrip):
		WriteError(w, r, http.StatusConflict, "active_trip_exists", "you already have a trip in progress")
	case err != nil:
		a.fail(w, r, err)
	case replayed:
		WriteJSON(w, http.StatusOK, trip)
	default:
		WriteJSON(w, http.StatusCreated, trip)
	}
}

func (a *API) listTrips(w http.ResponseWriter, r *http.Request) {
	trips, err := a.Store.TripsByPassenger(r.Context(), identity(r).UserID, 50)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, trips)
}

// loadTrip returns the trip if the caller is its passenger, its driver or an admin.
// Other callers get 404 so trip ids cannot be probed.
func (a *API) loadTrip(w http.ResponseWriter, r *http.Request) (store.Trip, bool) {
	id, ok := pathID(w, r)
	if !ok {
		return store.Trip{}, false
	}
	trip, err := a.Store.Trip(r.Context(), id)
	if err != nil {
		a.fail(w, r, err)
		return trip, false
	}
	me := identity(r)
	isDriver := trip.Cab != nil && trip.Cab.DriverID != nil && *trip.Cab.DriverID == me.UserID
	if trip.PassengerID != me.UserID && !isDriver && me.Role != "admin" {
		WriteError(w, r, http.StatusNotFound, "not_found", "resource not found")
		return trip, false
	}
	if trip.PassengerID == me.UserID {
		trip.BoardingCode = auth.BoardingCode(a.Cfg.JWTSecret, trip.ID)
	}
	return trip, true
}

func (a *API) getTrip(w http.ResponseWriter, r *http.Request) {
	if trip, ok := a.loadTrip(w, r); ok {
		WriteJSON(w, http.StatusOK, trip)
	}
}

func (a *API) tripTimeline(w http.ResponseWriter, r *http.Request) {
	trip, ok := a.loadTrip(w, r)
	if !ok {
		return
	}
	events, err := a.Tel.TripEvents(r.Context(), trip.ID)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	path, err := a.Tel.TripPath(r.Context(), trip.ID)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"available": a.Tel.Enabled(), "events": events, "path": path})
}

func (a *API) cancelTrip(w http.ResponseWriter, r *http.Request) {
	trip, ok := a.loadTrip(w, r)
	if !ok {
		return
	}
	if trip.PassengerID != identity(r).UserID {
		WriteError(w, r, http.StatusForbidden, "forbidden", "only the passenger can cancel a trip")
		return
	}
	out, err := a.Core.CancelTrip(r.Context(), trip.ID)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, out)
}

// ---------- Wi-Fi ----------

func (a *API) plans(w http.ResponseWriter, r *http.Request) {
	plans, err := a.Store.Plans(r.Context())
	if err != nil {
		a.fail(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"plans": plans, "purchases_enabled": a.Cfg.PaymentProvider == "mock"})
}

func (a *API) quota(w http.ResponseWriter, r *http.Request) {
	uid := identity(r).UserID
	balance, err := a.Store.QuotaBalance(r.Context(), uid)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	history, err := a.Store.QuotaHistory(r.Context(), uid, 20)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"remaining_bytes": balance, "history": history})
}

func (a *API) purchase(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PlanID string `json:"plan_id"`
	}
	if !decode(w, r, &in) {
		return
	}
	uid := identity(r).UserID
	if !a.allow(w, r, "purchase:"+uid, 10, time.Minute, true) {
		return
	}
	balance, plan, err := a.Core.PurchasePlan(r.Context(), uid, in.PlanID)
	switch {
	case errors.Is(err, store.ErrPlanNotFound):
		WriteError(w, r, http.StatusNotFound, "plan_not_found", "unknown plan")
	case errors.Is(err, core.ErrPaymentsDisabled):
		WriteError(w, r, http.StatusNotImplemented, "payments_disabled", "online payments are not enabled on this deployment")
	case err != nil:
		a.fail(w, r, err)
	default:
		WriteJSON(w, http.StatusCreated, map[string]any{"remaining_bytes": balance, "plan": plan, "payment_provider": "mock"})
	}
}

// ---------- realtime ----------

func (a *API) wsTicket(w http.ResponseWriter, r *http.Request) {
	t, err := a.Live.NewTicket(r.Context(), identity(r).UserID)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"ticket": t, "expires_in": 30})
}

// ServeWS authenticates a WebSocket upgrade with a single-use ticket. It is mounted
// outside the request-deadline middleware because the connection is long-lived.
func (a *API) ServeWS(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	uid, err := a.Live.RedeemTicket(ctx, r.URL.Query().Get("ticket"))
	cancel()
	if err != nil {
		WriteError(w, r, http.StatusServiceUnavailable, "unavailable", "service temporarily unavailable")
		return
	}
	if uid == "" {
		WriteError(w, r, http.StatusUnauthorized, "unauthenticated", "invalid or expired ticket")
		return
	}
	a.Hub.Serve(w, r, uid)
}

// ---------- driver ----------

func (a *API) driverCabOr404(w http.ResponseWriter, r *http.Request) (store.Cab, bool) {
	cab, err := a.Store.CabByDriver(r.Context(), identity(r).UserID)
	if errors.Is(err, store.ErrNotFound) {
		WriteError(w, r, http.StatusNotFound, "no_cab_assigned", "no cab is assigned to this driver account")
		return cab, false
	}
	if err != nil {
		a.fail(w, r, err)
		return cab, false
	}
	return cab, true
}

func (a *API) driverCab(w http.ResponseWriter, r *http.Request) {
	cab, ok := a.driverCabOr404(w, r)
	if !ok {
		return
	}
	out := map[string]any{"cab": cab, "trip": nil}
	if trip, err := a.Store.ActiveTripForCab(r.Context(), cab.ID); err == nil {
		out["trip"] = trip
	} else if !errors.Is(err, store.ErrNotFound) {
		a.fail(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, out)
}

func (a *API) driverAvailability(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Available *bool `json:"available"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Available == nil {
		WriteError(w, r, http.StatusBadRequest, "invalid_request", "available (boolean) is required")
		return
	}
	cab, ok := a.driverCabOr404(w, r)
	if !ok {
		return
	}
	if err := a.Store.SetCabAvailability(r.Context(), cab.ID, *in.Available); err != nil {
		a.fail(w, r, err)
		return
	}
	status := "offline"
	if *in.Available {
		status = "available"
	}
	if pos, ok := a.Live.Cab(r.Context(), cab.ID); ok || !*in.Available {
		pos.ID, pos.Status, pos.TS = cab.ID, status, time.Now().UnixMilli()
		if err := a.Live.UpdateCabs(r.Context(), []live.CabPos{pos}); err != nil {
			a.fail(w, r, err)
			return
		}
	}
	WriteJSON(w, http.StatusOK, map[string]string{"status": status})
}

func (a *API) driverLocation(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Lat     *float64 `json:"lat"`
		Lng     *float64 `json:"lng"`
		Heading float64  `json:"heading"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Lat == nil || in.Lng == nil || !geo.InServiceArea(*in.Lat, *in.Lng) || in.Heading < 0 || in.Heading >= 360 {
		WriteError(w, r, http.StatusBadRequest, "invalid_location", "lat/lng must be inside the service area and heading in [0, 360)")
		return
	}
	if !a.allow(w, r, "loc:"+identity(r).UserID, 120, time.Minute, false) {
		return
	}
	cab, ok := a.driverCabOr404(w, r)
	if !ok {
		return
	}
	if cab.Status == "offline" {
		WriteError(w, r, http.StatusConflict, "cab_offline", "go available before reporting location")
		return
	}
	tripByCab := map[string]string{}
	if trip, err := a.Store.ActiveTripForCab(r.Context(), cab.ID); err == nil {
		tripByCab[cab.ID] = trip.ID
	}
	pos := live.CabPos{ID: cab.ID, Lat: *in.Lat, Lng: *in.Lng, Heading: in.Heading, Status: cab.Status, TS: time.Now().UnixMilli()}
	if err := a.Core.ReportPositions(r.Context(), []live.CabPos{pos}, tripByCab); err != nil {
		a.fail(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"status": "recorded"})
}

// driverTrip loads the trip only if it belongs to the caller's cab.
func (a *API) driverTrip(w http.ResponseWriter, r *http.Request) (string, bool) {
	id, ok := pathID(w, r)
	if !ok {
		return "", false
	}
	cab, ok := a.driverCabOr404(w, r)
	if !ok {
		return "", false
	}
	trip, err := a.Store.ActiveTripForCab(r.Context(), cab.ID)
	if err != nil || trip.ID != id {
		WriteError(w, r, http.StatusNotFound, "not_found", "resource not found")
		return "", false
	}
	return id, true
}

func (a *API) driverStart(w http.ResponseWriter, r *http.Request) {
	var in struct {
		BoardingCode string `json:"boarding_code"`
	}
	if !decode(w, r, &in) {
		return
	}
	id, ok := a.driverTrip(w, r)
	if !ok {
		return
	}
	// Six digits is a small space: cap guesses per trip.
	if !a.allow(w, r, "board:"+id, 5, 10*time.Minute, true) {
		return
	}
	trip, err := a.Core.StartTrip(r.Context(), id, &in.BoardingCode)
	if errors.Is(err, core.ErrBadBoardingCode) {
		WriteError(w, r, http.StatusForbidden, "boarding_code_mismatch", "the boarding code does not match this trip")
		return
	}
	if err != nil {
		a.fail(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, trip)
}

func (a *API) driverComplete(w http.ResponseWriter, r *http.Request) {
	id, ok := a.driverTrip(w, r)
	if !ok {
		return
	}
	trip, err := a.Core.CompleteTrip(r.Context(), id)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, trip)
}

// ---------- admin ----------

func (a *API) adminStats(w http.ResponseWriter, r *http.Request) {
	st, err := a.Store.Stats(r.Context())
	if err != nil {
		a.fail(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"stats": st, "ws_connections_this_replica": a.Hub.Connections()})
}

var plateRE = regexp.MustCompile(`^[A-Z0-9][A-Z0-9 -]{3,14}$`)

func (a *API) adminCreateCab(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Plate       string `json:"plate"`
		Model       string `json:"model"`
		Seats       int    `json:"seats"`
		Electric    bool   `json:"electric"`
		DriverName  string `json:"driver_name"`
		DriverPhone string `json:"driver_phone"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Plate, in.Model, in.DriverName = strings.ToUpper(strings.TrimSpace(in.Plate)), strings.TrimSpace(in.Model), strings.TrimSpace(in.DriverName)
	if !plateRE.MatchString(in.Plate) || in.Model == "" || len(in.Model) > 60 || in.Seats < 1 || in.Seats > 12 ||
		len(in.DriverName) > 80 || !auth.ValidPhone(in.DriverPhone) {
		WriteError(w, r, http.StatusBadRequest, "invalid_cab", "plate, model, seats (1-12) and an E.164 driver_phone are required")
		return
	}
	cab, err := a.Store.CreateCab(r.Context(), store.NewCab{
		Plate: in.Plate, Model: in.Model, Seats: in.Seats, Electric: in.Electric,
		DriverName: in.DriverName, DriverPhone: in.DriverPhone,
	})
	if errors.Is(err, store.ErrConflict) {
		WriteError(w, r, http.StatusConflict, "cab_conflict", "the plate is registered or the driver already has a cab")
		return
	}
	if err != nil {
		a.fail(w, r, err)
		return
	}
	WriteJSON(w, http.StatusCreated, cab)
}

// ---------- gateway (service-to-service) ----------

func (a *API) gatewayUsage(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("X-Gateway-Token")
	if a.Cfg.GatewayToken == "" || subtle.ConstantTimeCompare([]byte(token), []byte(a.Cfg.GatewayToken)) != 1 {
		WriteError(w, r, http.StatusUnauthorized, "unauthenticated", "invalid gateway credentials")
		return
	}
	var in struct {
		ReportID string `json:"report_id"`
		TripID   string `json:"trip_id"`
		Bytes    int64  `json:"bytes"`
	}
	if !decode(w, r, &in) {
		return
	}
	// A single report is capped at 4 GiB to bound the damage of a faulty meter.
	if !idemRE.MatchString(in.ReportID) || !uuidRE.MatchString(in.TripID) || in.Bytes <= 0 || in.Bytes > 4<<30 {
		WriteError(w, r, http.StatusBadRequest, "invalid_report", "report_id, trip_id and bytes (1 to 4 GiB) are required")
		return
	}
	u, err := a.Core.ReportUsage(r.Context(), in.TripID, in.ReportID, in.Bytes)
	if errors.Is(err, core.ErrTripNotMetered) {
		WriteError(w, r, http.StatusConflict, "trip_not_in_progress", "usage can only be reported for an in-progress trip")
		return
	}
	if err != nil {
		a.fail(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, u)
}
