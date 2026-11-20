package cache

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/common/utils"
)

// These tests need a real Redis: REDIS_TEST_ADDR=localhost:6379 go test ./internal/cache/...
// They use DB 15 and flush it.
func testClient(t *testing.T) *redis.Client {
	addr := os.Getenv("REDIS_TEST_ADDR")
	if addr == "" {
		t.Skip("REDIS_TEST_ADDR not set")
	}
	rdb, err := NewClient(context.Background(), Options{Addr: addr, DB: 15, PoolSize: 5, DialTimeout: time.Second, ReadTimeout: time.Second, WriteTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	rdb.FlushDB(context.Background())
	t.Cleanup(func() { rdb.FlushDB(context.Background()); rdb.Close() })
	return rdb
}

func TestLockOwnership(t *testing.T) {
	rdb := testClient(t)
	ctx := context.Background()
	l := NewLocker(rdb)

	a, err := l.Acquire(ctx, "driver:d1", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.Acquire(ctx, "driver:d1", time.Second); !errors.Is(err, ErrLockNotAcquired) {
		t.Fatalf("second acquire should fail, got %v", err)
	}
	// a wrong token must not release
	if err := l.ReleaseToken(ctx, "driver:d1", "bogus"); err != nil {
		t.Fatal(err)
	}
	if locked, _ := l.IsLocked(ctx, "driver:d1"); !locked {
		t.Fatal("lock released by wrong token")
	}
	if err := a.Release(ctx); err != nil {
		t.Fatal(err)
	}
	if locked, _ := l.IsLocked(ctx, "driver:d1"); locked {
		t.Fatal("lock still held after release")
	}
}

func TestSlotsHoldConfirmCancel(t *testing.T) {
	rdb := testClient(t)
	ctx := context.Background()
	s := NewSlots(rdb)
	bm := KeyDailyBusSeats("trip1", "20261120")

	if err := s.Hold(ctx, bm, "alice", 40, time.Minute, 3, 4); err != nil {
		t.Fatal(err)
	}
	if err := s.Hold(ctx, bm, "bob", 40, time.Minute, 4); err == nil {
		t.Fatal("bob must not hold a seat alice holds")
	}
	if err := s.Confirm(ctx, bm, "bob", 40, 0, 3); err == nil {
		t.Fatal("bob must not confirm alice's hold")
	}
	if err := s.Confirm(ctx, bm, "alice", 40, time.Hour, 3, 4); err != nil {
		t.Fatal(err)
	}
	if n, _ := s.BookedCount(ctx, bm); n != 2 {
		t.Fatalf("booked=%d want 2", n)
	}
	if err := s.Hold(ctx, bm, "bob", 40, time.Minute, 50); err == nil {
		t.Fatal("seat beyond capacity must fail (no standing passengers)")
	}
	if err := s.Cancel(ctx, bm, "alice", 40, 4); err != nil {
		t.Fatal(err)
	}
	free, _ := s.Free(ctx, bm, 3, 4)
	if free[0] || !free[1] {
		t.Fatalf("free=%v want [false true]", free)
	}
}

func TestRateLimiter(t *testing.T) {
	rdb := testClient(t)
	rl := NewRateLimiter(rdb, nil)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if r, _ := rl.Allow(ctx, "test", "u1", 3, time.Minute); !r.Allowed {
			t.Fatalf("hit %d should pass", i)
		}
	}
	if r, _ := rl.Allow(ctx, "test", "u1", 3, time.Minute); r.Allowed || r.RetryAfter <= 0 {
		t.Fatalf("4th hit should be limited: %+v", r)
	}
}

func TestGeoNearby(t *testing.T) {
	rdb := testClient(t)
	g := NewGeo(rdb)
	ctx := context.Background()
	key := KeyStationGeo("petrol")
	_ = g.Add(ctx, key, "near", utils.Point{Lat: 28.6139, Lng: 77.2090})
	_ = g.Add(ctx, key, "far", utils.Point{Lat: 28.7041, Lng: 77.1025})
	hits, err := g.Nearby(ctx, key, utils.Point{Lat: 28.6140, Lng: 77.2091}, 2, 10)
	if err != nil || len(hits) != 1 || hits[0].Member != "near" {
		t.Fatalf("hits=%v err=%v", hits, err)
	}
}

func TestRememberAndIdempotency(t *testing.T) {
	rdb := testClient(t)
	s := NewService(rdb)
	ctx := context.Background()
	calls := 0
	load := func(context.Context) (int, error) { calls++; return 42, nil }
	for i := 0; i < 3; i++ {
		if v, _ := Remember(ctx, s, "sr:test:remember", time.Minute, load); v != 42 {
			t.Fatal("wrong value")
		}
	}
	if calls != 1 {
		t.Fatalf("loader called %d times", calls)
	}
	if v, fresh, _ := s.Idempotent(ctx, "booking", "k1", "b1"); !fresh || v != "b1" {
		t.Fatal("first claim should be fresh")
	}
	if v, fresh, _ := s.Idempotent(ctx, "booking", "k1", "b2"); fresh || v != "b1" {
		t.Fatal("second claim should return original value")
	}
}
