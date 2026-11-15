package store

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sarathi/sarathi/backend/internal/database"
	"github.com/sarathi/sarathi/backend/migrations"
)

// These tests need a real PostgreSQL (SARATHI_TEST_DATABASE_URL) and are skipped
// otherwise. Each run migrates into its own schema, so it cannot collide with other
// packages using the same database.
func newStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	url := os.Getenv("SARATHI_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("SARATHI_TEST_DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)

	schema := fmt.Sprintf("store_test_%d", rand.Uint64())
	admin, err := database.Open(url)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.ExecContext(context.Background(), `DROP SCHEMA `+schema+` CASCADE`)
		admin.Close()
	})

	sep := "?"
	if strings.Contains(url, "?") {
		sep = "&"
	}
	db, err := database.Open(url + sep + "search_path=" + schema)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := database.Migrate(ctx, db, migrations.FS, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatal(err)
	}
	return &Store{DB: db}, ctx
}

func mustUser(t *testing.T, s *Store, ctx context.Context, i int) User {
	t.Helper()
	u, err := s.UpsertUserByPhone(ctx, fmt.Sprintf("+9198765%05d", i), "", false)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func mustCab(t *testing.T, s *Store, ctx context.Context, plate string) Cab {
	t.Helper()
	if err := s.EnsureSimulatedCab(ctx, NewCab{Plate: plate, Model: "Test", Seats: 4}); err != nil {
		t.Fatal(err)
	}
	cabs, err := s.SimulatedCabs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cabs {
		if c.Plate == plate {
			if err := s.SetCabAvailability(ctx, c.ID, true); err != nil {
				t.Fatal(err)
			}
			return c
		}
	}
	t.Fatal("cab not created")
	return Cab{}
}

func booking(u User, cabs ...Cab) NewTrip {
	n := NewTrip{
		PassengerID: u.ID, PickupPlace: "A", DropPlace: "B", PickupLat: 12.9, PickupLng: 77.5, DropLat: 12.95, DropLng: 77.6,
		DistanceM: 1000, DurationS: 120, FarePaise: 5000,
	}
	for _, c := range cabs {
		n.CandidateCabs = append(n.CandidateCabs, c.ID)
	}
	return n
}

func TestOneCabIsNeverDoubleBooked(t *testing.T) {
	s, ctx := newStore(t)
	cab := mustCab(t, s, ctx, "T 1")

	const riders = 8
	var wg sync.WaitGroup
	results := make([]error, riders)
	trips := make([]Trip, riders)
	for i := range riders {
		u := mustUser(t, s, ctx, i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			trips[i], _, results[i] = s.BookTrip(ctx, booking(u, cab))
		}()
	}
	wg.Wait()

	won := -1
	for i, err := range results {
		switch {
		case err == nil && won == -1:
			won = i
		case err == nil:
			t.Fatalf("cab was booked twice (riders %d and %d)", won, i)
		case !errors.Is(err, ErrNoCab):
			t.Fatalf("rider %d: unexpected error %v", i, err)
		}
	}
	if won == -1 {
		t.Fatal("nobody got the cab")
	}

	// The lifecycle only moves forward, and finishing releases the cab.
	id := trips[won].ID
	if _, err := s.Transition(ctx, id, "in_progress", "completed"); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("completing an unstarted trip: %v", err)
	}
	if _, err := s.Transition(ctx, id, "assigned", "in_progress"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Transition(ctx, id, "assigned", "cancelled"); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("cancelling a started trip: %v", err)
	}
	done, err := s.Transition(ctx, id, "in_progress", "completed")
	if err != nil || done.Cab == nil || done.Cab.Status != "available" {
		t.Fatalf("completion did not release the cab: %+v %v", done.Cab, err)
	}
}

func TestConcurrentRidersGetDifferentCabsAndIdempotency(t *testing.T) {
	s, ctx := newStore(t)
	a, b := mustCab(t, s, ctx, "T 1"), mustCab(t, s, ctx, "T 2")
	u1, u2 := mustUser(t, s, ctx, 1), mustUser(t, s, ctx, 2)

	var wg sync.WaitGroup
	var t1, t2 Trip
	var e1, e2 error
	wg.Add(2)
	go func() { defer wg.Done(); t1, _, e1 = s.BookTrip(ctx, booking(u1, a, b)) }()
	go func() { defer wg.Done(); t2, _, e2 = s.BookTrip(ctx, booking(u2, a, b)) }()
	wg.Wait()
	if e1 != nil || e2 != nil || *t1.CabID == *t2.CabID {
		t.Fatalf("expected two trips on two cabs: %v %v", e1, e2)
	}

	// A second live trip for the same passenger is refused...
	if _, _, err := s.BookTrip(ctx, booking(u1, a, b)); !errors.Is(err, ErrNoCab) && !errors.Is(err, ErrActiveTrip) {
		t.Fatalf("second live trip: %v", err)
	}
	// ...and a replayed idempotency key returns the original trip.
	if _, err := s.Transition(ctx, t1.ID, "assigned", "cancelled"); err != nil {
		t.Fatal(err)
	}
	n := booking(u1, a, b)
	n.IdempotencyKey = "key-12345"
	first, replayed, err := s.BookTrip(ctx, n)
	if err != nil || replayed {
		t.Fatalf("first keyed booking: %v replayed=%v", err, replayed)
	}
	again, replayed, err := s.BookTrip(ctx, n)
	if err != nil || !replayed || again.ID != first.ID {
		t.Fatalf("replay must return the same trip: %v replayed=%v", err, replayed)
	}
}

func TestQuotaLedgerNeverGoesNegativeAndIsAppendOnly(t *testing.T) {
	s, ctx := newStore(t)
	cab := mustCab(t, s, ctx, "T 1")
	u := mustUser(t, s, ctx, 1)
	trip, _, err := s.BookTrip(ctx, booking(u, cab))
	if err != nil {
		t.Fatal(err)
	}

	if bal, _ := s.QuotaBalance(ctx, u.ID); bal != SignupGrantBytes {
		t.Fatalf("signup grant = %d", bal)
	}
	// Signing in again must not grant again.
	if _, err := s.UpsertUserByPhone(ctx, u.Phone, "", false); err != nil {
		t.Fatal(err)
	}
	if bal, _ := s.QuotaBalance(ctx, u.ID); bal != SignupGrantBytes {
		t.Fatalf("grant repeated on second sign-in: %d", bal)
	}

	debited, remaining, err := s.DebitUsage(ctx, u.ID, trip.ID, "report-1", 100<<20)
	if err != nil || debited != 100<<20 || remaining != 400<<20 {
		t.Fatalf("debit: %d %d %v", debited, remaining, err)
	}
	// A retried report is not charged twice.
	_, remaining, err = s.DebitUsage(ctx, u.ID, trip.ID, "report-1", 100<<20)
	if !errors.Is(err, ErrDuplicateRepo) || remaining != 400<<20 {
		t.Fatalf("duplicate report: %d %v", remaining, err)
	}

	// 20 concurrent 50 MiB reports against 400 MiB: exactly 8 can be honoured.
	var wg sync.WaitGroup
	var mu sync.Mutex
	var total int64
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d, _, err := s.DebitUsage(ctx, u.ID, trip.ID, fmt.Sprintf("burst-%d", i), 50<<20)
			if err != nil {
				t.Errorf("burst %d: %v", i, err)
			}
			mu.Lock()
			total += d
			mu.Unlock()
		}()
	}
	wg.Wait()
	bal, _ := s.QuotaBalance(ctx, u.ID)
	if total != 400<<20 || bal != 0 {
		t.Fatalf("overdraw: debited %d, balance %d", total, bal)
	}

	if _, err := s.DB.ExecContext(ctx, `UPDATE quota_ledger SET delta_bytes = 1`); err == nil {
		t.Fatal("ledger rows must not be updatable")
	}
	if _, err := s.DB.ExecContext(ctx, `DELETE FROM quota_ledger`); err == nil {
		t.Fatal("ledger rows must not be deletable")
	}
}

func TestRefreshTokenRotationAndReuseDetection(t *testing.T) {
	s, ctx := newStore(t)
	u := mustUser(t, s, ctx, 1)
	exp := time.Now().Add(time.Hour)
	if err := s.SaveRefreshToken(ctx, u.ID, "tok-1", exp); err != nil {
		t.Fatal(err)
	}
	got, err := s.RotateRefreshToken(ctx, "tok-1", "tok-2", exp)
	if err != nil || got.ID != u.ID {
		t.Fatalf("rotate: %v", err)
	}
	// Replaying the old token is treated as theft: every session is revoked.
	if _, err := s.RotateRefreshToken(ctx, "tok-1", "tok-3", exp); !errors.Is(err, ErrTokenReused) {
		t.Fatalf("reuse: %v", err)
	}
	if _, err := s.RotateRefreshToken(ctx, "tok-2", "tok-4", exp); !errors.Is(err, ErrTokenReused) {
		t.Fatalf("the rotated token must be revoked after reuse: %v", err)
	}
	if _, err := s.RotateRefreshToken(ctx, "never-issued", "x", exp); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown token: %v", err)
	}
	if err := s.SaveRefreshToken(ctx, u.ID, "expired", time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RotateRefreshToken(ctx, "expired", "y", exp); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired token: %v", err)
	}
}
