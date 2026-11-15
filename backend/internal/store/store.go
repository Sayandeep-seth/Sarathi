// Package store is the PostgreSQL data layer. PostgreSQL is the source of truth for
// identity, fleet state, trips, payments and the quota ledger.
package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// Sentinel errors returned by the store.
var (
	ErrNotFound      = errors.New("not found")
	ErrConflict      = errors.New("conflict")
	ErrNoCab         = errors.New("no cab available")
	ErrActiveTrip    = errors.New("passenger already has an active trip")
	ErrInvalidState  = errors.New("invalid state for this operation")
	ErrTokenReused   = errors.New("refresh token reuse detected")
	ErrPlanNotFound  = errors.New("plan not found")
	ErrDuplicateRepo = errors.New("duplicate usage report")
)

// SignupGrantBytes is the free quota granted when an account is created (500 MiB).
const SignupGrantBytes = 500 << 20

// Store wraps the connection pool.
type Store struct{ DB *sql.DB }

// User is an account identified by a verified phone number.
type User struct {
	ID        string    `json:"id"`
	Phone     string    `json:"phone"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// Cab is a vehicle in the fleet.
type Cab struct {
	ID         string   `json:"id"`
	Plate      string   `json:"plate"`
	Model      string   `json:"model"`
	Seats      int      `json:"seats"`
	Electric   bool     `json:"electric"`
	DriverID   *string  `json:"-"`
	DriverName string   `json:"driver_name"`
	Simulated  bool     `json:"simulated"`
	Status     string   `json:"status"`
	WifiSSID   string   `json:"wifi_ssid"`
	Lat        *float64 `json:"lat"`
	Lng        *float64 `json:"lng"`
	Heading    float64  `json:"heading"`
}

// Trip is a booked ride.
type Trip struct {
	ID          string     `json:"id"`
	PassengerID string     `json:"passenger_id"`
	CabID       *string    `json:"cab_id"`
	Status      string     `json:"status"`
	PickupPlace string     `json:"pickup_place"`
	DropPlace   string     `json:"drop_place"`
	PickupLat   float64    `json:"pickup_lat"`
	PickupLng   float64    `json:"pickup_lng"`
	DropLat     float64    `json:"drop_lat"`
	DropLng     float64    `json:"drop_lng"`
	DistanceM   int        `json:"distance_m"`
	DurationS   int        `json:"duration_s"`
	FarePaise   int64      `json:"fare_paise"`
	RequestedAt time.Time  `json:"requested_at"`
	AssignedAt  *time.Time `json:"assigned_at"`
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	CancelledAt *time.Time `json:"cancelled_at"`
	Cab         *Cab       `json:"cab,omitempty"`
	// BoardingCode is derived, never stored; set only for the passenger's own view.
	BoardingCode string `json:"boarding_code,omitempty"`
}

// Plan is a purchasable Wi-Fi quota pack.
type Plan struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	QuotaBytes int64  `json:"quota_bytes"`
	PricePaise int64  `json:"price_paise"`
}

// LedgerEntry is one append-only quota movement.
type LedgerEntry struct {
	ID         int64     `json:"id"`
	DeltaBytes int64     `json:"delta_bytes"`
	Reason     string    `json:"reason"`
	TripID     *string   `json:"trip_id"`
	CreatedAt  time.Time `json:"created_at"`
}

func isUnique(err error, constraint string) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505" && (constraint == "" || pg.ConstraintName == constraint)
}

func (s *Store) tx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// ---------- users ----------

const userCols = `id, phone, name, role, created_at`

func scanUser(row interface{ Scan(...any) error }) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Phone, &u.Name, &u.Role, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return u, ErrNotFound
	}
	return u, err
}

// UpsertUserByPhone returns the account for a verified phone, creating it (with the
// signup quota grant) on first sign-in. Admin promotion is applied when requested.
func (s *Store) UpsertUserByPhone(ctx context.Context, phone, name string, admin bool) (User, error) {
	var u User
	err := s.tx(ctx, func(tx *sql.Tx) error {
		var created bool
		row := tx.QueryRowContext(ctx, `
			INSERT INTO users (phone, name) VALUES ($1, $2)
			ON CONFLICT (phone) DO UPDATE SET phone = EXCLUDED.phone
			RETURNING `+userCols+`, (xmax = 0)`, phone, name)
		if err := row.Scan(&u.ID, &u.Phone, &u.Name, &u.Role, &u.CreatedAt, &created); err != nil {
			return err
		}
		if created {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO quota_ledger (user_id, delta_bytes, reason) VALUES ($1, $2, 'signup_grant')`,
				u.ID, int64(SignupGrantBytes)); err != nil {
				return err
			}
		}
		if admin && u.Role != "admin" {
			u.Role = "admin"
			if _, err := tx.ExecContext(ctx, `UPDATE users SET role = 'admin' WHERE id = $1`, u.ID); err != nil {
				return err
			}
		}
		return nil
	})
	return u, err
}

// User loads an account by id.
func (s *Store) User(ctx context.Context, id string) (User, error) {
	return scanUser(s.DB.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE id = $1`, id))
}

// UpdateUserName sets the display name.
func (s *Store) UpdateUserName(ctx context.Context, id, name string) (User, error) {
	return scanUser(s.DB.QueryRowContext(ctx,
		`UPDATE users SET name = $2 WHERE id = $1 RETURNING `+userCols, id, name))
}

// ---------- refresh tokens ----------

func hashToken(raw string) []byte {
	h := sha256.Sum256([]byte(raw))
	return h[:]
}

// SaveRefreshToken stores the hash of a newly issued refresh token.
func (s *Store) SaveRefreshToken(ctx context.Context, userID, raw string, expires time.Time) error {
	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		userID, hashToken(raw), expires)
	return err
}

// RotateRefreshToken atomically revokes oldRaw and stores newRaw. Presenting an
// already-revoked token revokes every session of that user and returns ErrTokenReused.
func (s *Store) RotateRefreshToken(ctx context.Context, oldRaw, newRaw string, expires time.Time) (User, error) {
	var u User
	var reused bool
	err := s.tx(ctx, func(tx *sql.Tx) error {
		var id, userID string
		var revoked *time.Time
		var exp time.Time
		err := tx.QueryRowContext(ctx,
			`SELECT id, user_id, revoked_at, expires_at FROM refresh_tokens WHERE token_hash = $1 FOR UPDATE`,
			hashToken(oldRaw)).Scan(&id, &userID, &revoked, &exp)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if revoked != nil {
			reused = true
			_, err = tx.ExecContext(ctx,
				`UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
			return err
		}
		if time.Now().After(exp) {
			return ErrNotFound
		}
		if _, err = tx.ExecContext(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE id = $1`, id); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx,
			`INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
			userID, hashToken(newRaw), expires); err != nil {
			return err
		}
		u, err = scanUser(tx.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE id = $1`, userID))
		return err
	})
	if err == nil && reused {
		return u, ErrTokenReused
	}
	return u, err
}

// RevokeRefreshToken revokes a single token; unknown tokens are ignored.
func (s *Store) RevokeRefreshToken(ctx context.Context, raw string) error {
	_, err := s.DB.ExecContext(ctx,
		`UPDATE refresh_tokens SET revoked_at = now() WHERE token_hash = $1 AND revoked_at IS NULL`, hashToken(raw))
	return err
}

// PurgeExpiredRefreshTokens deletes tokens that expired more than a day ago.
func (s *Store) PurgeExpiredRefreshTokens(ctx context.Context) (int64, error) {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE expires_at < now() - interval '1 day'`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ---------- cabs ----------

const cabCols = `c.id, c.plate, c.model, c.seats, c.electric, c.driver_id, c.driver_name, c.simulated,
	c.status, c.wifi_ssid, c.last_lat, c.last_lng, c.last_heading`

func scanCab(row interface{ Scan(...any) error }) (Cab, error) {
	var c Cab
	err := row.Scan(&c.ID, &c.Plate, &c.Model, &c.Seats, &c.Electric, &c.DriverID, &c.DriverName, &c.Simulated,
		&c.Status, &c.WifiSSID, &c.Lat, &c.Lng, &c.Heading)
	if errors.Is(err, sql.ErrNoRows) {
		return c, ErrNotFound
	}
	return c, err
}

func (s *Store) queryCabs(ctx context.Context, where string, args ...any) ([]Cab, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+cabCols+` FROM cabs c `+where+` ORDER BY c.plate`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Cab{}
	for rows.Next() {
		c, err := scanCab(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// OnlineCabs lists every cab that is not offline.
func (s *Store) OnlineCabs(ctx context.Context) ([]Cab, error) {
	return s.queryCabs(ctx, `WHERE c.status <> 'offline'`)
}

// SimulatedCabs lists simulator-owned cabs.
func (s *Store) SimulatedCabs(ctx context.Context) ([]Cab, error) {
	return s.queryCabs(ctx, `WHERE c.simulated`)
}

// Cab loads one cab.
func (s *Store) Cab(ctx context.Context, id string) (Cab, error) {
	return scanCab(s.DB.QueryRowContext(ctx, `SELECT `+cabCols+` FROM cabs c WHERE c.id = $1`, id))
}

// CabByDriver loads the cab assigned to a driver account.
func (s *Store) CabByDriver(ctx context.Context, driverID string) (Cab, error) {
	return scanCab(s.DB.QueryRowContext(ctx, `SELECT `+cabCols+` FROM cabs c WHERE c.driver_id = $1`, driverID))
}

// NewCab describes a cab to register.
type NewCab struct {
	Plate, Model, DriverName, DriverPhone string
	Seats                                 int
	Electric, Simulated                   bool
}

// CreateCab registers a cab. When DriverPhone is set, that account is created if
// needed and promoted to the driver role. Plates are unique (ErrConflict).
func (s *Store) CreateCab(ctx context.Context, n NewCab) (Cab, error) {
	var cab Cab
	err := s.tx(ctx, func(tx *sql.Tx) error {
		var driverID *string
		if n.DriverPhone != "" {
			var id string
			if err := tx.QueryRowContext(ctx, `
				INSERT INTO users (phone, name, role) VALUES ($1, $2, 'driver')
				ON CONFLICT (phone) DO UPDATE SET role = CASE WHEN users.role = 'admin' THEN 'admin' ELSE 'driver' END
				RETURNING id`, n.DriverPhone, n.DriverName).Scan(&id); err != nil {
				return err
			}
			driverID = &id
		}
		ssid := "SARATHI-" + strings.ReplaceAll(n.Plate, " ", "")
		var err error
		cab, err = scanCab(tx.QueryRowContext(ctx, `
			INSERT INTO cabs AS c (plate, model, seats, electric, driver_id, driver_name, simulated, wifi_ssid)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING `+cabCols,
			n.Plate, n.Model, n.Seats, n.Electric, driverID, n.DriverName, n.Simulated, ssid))
		return err
	})
	if isUnique(err, "") {
		return cab, ErrConflict
	}
	return cab, err
}

// EnsureSimulatedCab inserts a simulator cab if its plate is not registered yet.
func (s *Store) EnsureSimulatedCab(ctx context.Context, n NewCab) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO cabs (plate, model, seats, electric, driver_name, simulated, wifi_ssid)
		VALUES ($1, $2, $3, $4, $5, true, $6) ON CONFLICT (plate) DO NOTHING`,
		n.Plate, n.Model, n.Seats, n.Electric, n.DriverName, "SARATHI-"+strings.ReplaceAll(n.Plate, " ", ""))
	return err
}

// SetCabAvailability switches a cab between offline and available. A cab on a trip
// cannot change availability (ErrInvalidState).
func (s *Store) SetCabAvailability(ctx context.Context, cabID string, available bool) error {
	status := "offline"
	if available {
		status = "available"
	}
	res, err := s.DB.ExecContext(ctx,
		`UPDATE cabs SET status = $2 WHERE id = $1 AND status <> 'on_trip'`, cabID, status)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrInvalidState
	}
	return nil
}

// SaveCabPosition persists the last known position (called at a throttled rate).
func (s *Store) SaveCabPosition(ctx context.Context, cabID string, lat, lng, heading float64) error {
	_, err := s.DB.ExecContext(ctx,
		`UPDATE cabs SET last_lat = $2, last_lng = $3, last_heading = $4, last_seen_at = now() WHERE id = $1`,
		cabID, lat, lng, heading)
	return err
}

// ---------- trips ----------

const tripCols = `t.id, t.passenger_id, t.cab_id, t.status, t.pickup_place, t.drop_place, t.pickup_lat, t.pickup_lng,
	t.drop_lat, t.drop_lng, t.distance_m, t.duration_s, t.fare_paise, t.requested_at, t.assigned_at, t.started_at,
	t.completed_at, t.cancelled_at`

func scanTrip(row interface{ Scan(...any) error }) (Trip, error) {
	var t Trip
	err := row.Scan(&t.ID, &t.PassengerID, &t.CabID, &t.Status, &t.PickupPlace, &t.DropPlace, &t.PickupLat, &t.PickupLng,
		&t.DropLat, &t.DropLng, &t.DistanceM, &t.DurationS, &t.FarePaise, &t.RequestedAt, &t.AssignedAt, &t.StartedAt,
		&t.CompletedAt, &t.CancelledAt)
	if errors.Is(err, sql.ErrNoRows) {
		return t, ErrNotFound
	}
	return t, err
}

// NewTrip is a priced booking request.
type NewTrip struct {
	PassengerID, PickupPlace, DropPlace    string
	PickupLat, PickupLng, DropLat, DropLng float64
	DistanceM, DurationS                   int
	FarePaise                              int64
	IdempotencyKey                         string
	// CandidateCabs are cab ids ordered nearest-first; the first one still available wins.
	CandidateCabs []string
}

// BookTrip assigns the nearest available candidate cab and creates the trip in one
// transaction. Row locks with SKIP LOCKED make concurrent bookings pick different cabs.
// A repeated IdempotencyKey returns the original trip.
func (s *Store) BookTrip(ctx context.Context, n NewTrip) (trip Trip, replayed bool, err error) {
	if n.IdempotencyKey != "" {
		t, err := scanTrip(s.DB.QueryRowContext(ctx,
			`SELECT `+tripCols+` FROM trips t WHERE t.passenger_id = $1 AND t.idempotency_key = $2`,
			n.PassengerID, n.IdempotencyKey))
		if err == nil {
			return t, true, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return t, false, err
		}
	}
	err = s.tx(ctx, func(tx *sql.Tx) error {
		var cabID string
		err := tx.QueryRowContext(ctx, `
			UPDATE cabs SET status = 'on_trip' WHERE id = (
				SELECT id FROM cabs WHERE id::text = ANY($1) AND status = 'available'
				ORDER BY array_position($1, id::text) LIMIT 1 FOR UPDATE SKIP LOCKED
			) RETURNING id`, n.CandidateCabs).Scan(&cabID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNoCab
		}
		if err != nil {
			return err
		}
		var idem *string
		if n.IdempotencyKey != "" {
			idem = &n.IdempotencyKey
		}
		trip, err = scanTrip(tx.QueryRowContext(ctx, `
			INSERT INTO trips AS t (passenger_id, cab_id, status, pickup_place, drop_place, pickup_lat, pickup_lng,
				drop_lat, drop_lng, distance_m, duration_s, fare_paise, idempotency_key, assigned_at)
			VALUES ($1, $2, 'assigned', $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, now())
			RETURNING `+tripCols,
			n.PassengerID, cabID, n.PickupPlace, n.DropPlace, n.PickupLat, n.PickupLng, n.DropLat, n.DropLng,
			n.DistanceM, n.DurationS, n.FarePaise, idem))
		return err
	})
	if isUnique(err, "trips_one_active_per_passenger") {
		return trip, false, ErrActiveTrip
	}
	return trip, false, err
}

// Trip loads a trip with its cab.
func (s *Store) Trip(ctx context.Context, id string) (Trip, error) {
	t, err := scanTrip(s.DB.QueryRowContext(ctx, `SELECT `+tripCols+` FROM trips t WHERE t.id = $1`, id))
	if err != nil {
		return t, err
	}
	return s.withCab(ctx, t)
}

func (s *Store) withCab(ctx context.Context, t Trip) (Trip, error) {
	if t.CabID == nil {
		return t, nil
	}
	c, err := s.Cab(ctx, *t.CabID)
	if err != nil {
		return t, err
	}
	t.Cab = &c
	return t, nil
}

// TripsByPassenger lists a passenger's most recent trips.
func (s *Store) TripsByPassenger(ctx context.Context, passengerID string, limit int) ([]Trip, error) {
	return s.queryTrips(ctx, `WHERE t.passenger_id = $1 ORDER BY t.requested_at DESC LIMIT $2`, passengerID, limit)
}

// ActiveTripForCab returns the live trip a cab is serving, if any.
func (s *Store) ActiveTripForCab(ctx context.Context, cabID string) (Trip, error) {
	return scanTrip(s.DB.QueryRowContext(ctx,
		`SELECT `+tripCols+` FROM trips t WHERE t.cab_id = $1 AND t.status IN ('assigned', 'in_progress')`, cabID))
}

// ActiveSimulatedTrips lists live trips served by simulator cabs.
func (s *Store) ActiveSimulatedTrips(ctx context.Context) ([]Trip, error) {
	return s.queryTrips(ctx, `JOIN cabs c ON c.id = t.cab_id
		WHERE c.simulated AND t.status IN ('assigned', 'in_progress')`)
}

func (s *Store) queryTrips(ctx context.Context, tail string, args ...any) ([]Trip, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+tripCols+` FROM trips t `+tail, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Trip{}
	for rows.Next() {
		t, err := scanTrip(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Transition moves a trip from one status to another, releasing the cab when the trip
// ends. It returns ErrInvalidState when the trip is not in `from`.
func (s *Store) Transition(ctx context.Context, tripID, from, to string) (Trip, error) {
	stamp := map[string]string{"in_progress": "started_at", "completed": "completed_at", "cancelled": "cancelled_at"}[to]
	if stamp == "" {
		return Trip{}, fmt.Errorf("unsupported transition to %q", to)
	}
	var trip Trip
	err := s.tx(ctx, func(tx *sql.Tx) error {
		var err error
		trip, err = scanTrip(tx.QueryRowContext(ctx,
			`UPDATE trips AS t SET status = $3, `+stamp+` = now() WHERE t.id = $1 AND t.status = $2 RETURNING `+tripCols,
			tripID, from, to))
		if errors.Is(err, ErrNotFound) {
			return ErrInvalidState
		}
		if err != nil {
			return err
		}
		if (to == "completed" || to == "cancelled") && trip.CabID != nil {
			_, err = tx.ExecContext(ctx,
				`UPDATE cabs SET status = 'available' WHERE id = $1 AND status = 'on_trip'`, *trip.CabID)
		}
		return err
	})
	if err != nil {
		return trip, err
	}
	return s.withCab(ctx, trip)
}

// ---------- Wi-Fi quota ----------

// Plans lists the active Wi-Fi plans, cheapest first.
func (s *Store) Plans(ctx context.Context) ([]Plan, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, name, quota_bytes, price_paise FROM wifi_plans WHERE active ORDER BY price_paise`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Plan{}
	for rows.Next() {
		var p Plan
		if err := rows.Scan(&p.ID, &p.Name, &p.QuotaBytes, &p.PricePaise); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Plan loads one active plan.
func (s *Store) Plan(ctx context.Context, id string) (Plan, error) {
	var p Plan
	err := s.DB.QueryRowContext(ctx,
		`SELECT id, name, quota_bytes, price_paise FROM wifi_plans WHERE id = $1 AND active`, id).
		Scan(&p.ID, &p.Name, &p.QuotaBytes, &p.PricePaise)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrPlanNotFound
	}
	return p, err
}

// QuotaBalance returns the remaining bytes for a user.
func (s *Store) QuotaBalance(ctx context.Context, userID string) (int64, error) {
	var b int64
	err := s.DB.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(delta_bytes), 0)::bigint FROM quota_ledger WHERE user_id = $1`, userID).Scan(&b)
	return b, err
}

// QuotaHistory returns the most recent ledger entries for a user.
func (s *Store) QuotaHistory(ctx context.Context, userID string, limit int) ([]LedgerEntry, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, delta_bytes, reason, trip_id, created_at FROM quota_ledger
		WHERE user_id = $1 ORDER BY id DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LedgerEntry{}
	for rows.Next() {
		var e LedgerEntry
		if err := rows.Scan(&e.ID, &e.DeltaBytes, &e.Reason, &e.TripID, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// GrantPlan records a succeeded payment and credits the plan's quota atomically.
func (s *Store) GrantPlan(ctx context.Context, userID string, p Plan, provider, providerRef string) (int64, error) {
	var balance int64
	err := s.tx(ctx, func(tx *sql.Tx) error {
		var payID string
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO payments (user_id, purpose, reference, amount_paise, provider, provider_ref, status)
			VALUES ($1, 'wifi_plan', $2, $3, $4, $5, 'succeeded') RETURNING id`,
			userID, p.ID, p.PricePaise, provider, providerRef).Scan(&payID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO quota_ledger (user_id, delta_bytes, reason, payment_id) VALUES ($1, $2, 'plan_purchase', $3)`,
			userID, p.QuotaBytes, payID); err != nil {
			return err
		}
		return tx.QueryRowContext(ctx,
			`SELECT COALESCE(SUM(delta_bytes), 0)::bigint FROM quota_ledger WHERE user_id = $1`, userID).Scan(&balance)
	})
	return balance, err
}

// DebitUsage records metered usage against a user's quota. The debit is clamped to
// the remaining balance so the ledger never goes negative. reportID makes the call
// idempotent: a repeated report returns ErrDuplicateRepo with the current balance.
// The user row is locked so concurrent reports for one user serialize.
func (s *Store) DebitUsage(ctx context.Context, userID, tripID, reportID string, bytes int64) (debited, remaining int64, err error) {
	err = s.tx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `SELECT 1 FROM users WHERE id = $1 FOR UPDATE`, userID); err != nil {
			return err
		}
		if err := tx.QueryRowContext(ctx,
			`SELECT COALESCE(SUM(delta_bytes), 0)::bigint FROM quota_ledger WHERE user_id = $1`, userID).Scan(&remaining); err != nil {
			return err
		}
		var dup bool
		if err := tx.QueryRowContext(ctx,
			`SELECT EXISTS (SELECT 1 FROM quota_ledger WHERE report_id = $1)`, reportID).Scan(&dup); err != nil {
			return err
		}
		if dup {
			return ErrDuplicateRepo
		}
		debited = min(bytes, max(remaining, 0))
		if debited == 0 {
			return nil
		}
		remaining -= debited
		_, err := tx.ExecContext(ctx, `
			INSERT INTO quota_ledger (user_id, delta_bytes, reason, trip_id, report_id) VALUES ($1, $2, 'usage', $3, $4)`,
			userID, -debited, tripID, reportID)
		return err
	})
	return debited, remaining, err
}

// ---------- admin ----------

// Stats is an operational summary.
type Stats struct {
	Users          int64 `json:"users"`
	CabsOnline     int64 `json:"cabs_online"`
	TripsActive    int64 `json:"trips_active"`
	TripsCompleted int64 `json:"trips_completed"`
	RevenuePaise   int64 `json:"completed_fare_paise"`
}

// Stats returns fleet-wide counters.
func (s *Store) Stats(ctx context.Context) (Stats, error) {
	var st Stats
	err := s.DB.QueryRowContext(ctx, `SELECT
		(SELECT count(*) FROM users),
		(SELECT count(*) FROM cabs WHERE status <> 'offline'),
		(SELECT count(*) FROM trips WHERE status IN ('assigned', 'in_progress')),
		(SELECT count(*) FROM trips WHERE status = 'completed'),
		(SELECT COALESCE(SUM(fare_paise), 0)::bigint FROM trips WHERE status = 'completed')`).
		Scan(&st.Users, &st.CabsOnline, &st.TripsActive, &st.TripsCompleted, &st.RevenuePaise)
	return st, err
}
