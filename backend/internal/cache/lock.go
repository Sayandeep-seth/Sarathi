package cache

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	apperr "sarathi/internal/common/errors"
	"sarathi/internal/common/utils"
)

// ErrLockNotAcquired is returned when a lock is held by someone else.
var ErrLockNotAcquired = errors.New("cache: lock not acquired")

var extendLockScript = redis.NewScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then
  return redis.call('PEXPIRE', KEYS[1], ARGV[2])
end
return 0`)

// ---------------------------------------------------------------------------
// Distributed locks: SET key token NX PX ttl; release only by the token owner.
// Used for drivers during an offer, refunds, migrations, model reloads, etc.
// ---------------------------------------------------------------------------

type Locker struct{ rdb *redis.Client }

func NewLocker(rdb *redis.Client) *Locker { return &Locker{rdb: rdb} }

// Lock is a held lock.
type Lock struct {
	rdb   *redis.Client
	Key   string
	Token string
}

// Acquire tries once to take the lock.
func (l *Locker) Acquire(ctx context.Context, name string, ttl time.Duration) (*Lock, error) {
	k, token := KeyLock(name), utils.RandomToken(16)
	ok, err := l.rdb.SetNX(ctx, k, token, ttl).Result()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrLockNotAcquired
	}
	return &Lock{rdb: l.rdb, Key: k, Token: token}, nil
}

// AcquireWait retries with backoff until wait elapses.
func (l *Locker) AcquireWait(ctx context.Context, name string, ttl, wait time.Duration) (*Lock, error) {
	deadline := time.Now().Add(wait)
	backoff := 25 * time.Millisecond
	for {
		lk, err := l.Acquire(ctx, name, ttl)
		if !errors.Is(err, ErrLockNotAcquired) || time.Now().After(deadline) {
			return lk, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < 400*time.Millisecond {
			backoff *= 2
		}
	}
}

// IsLocked reports whether someone holds the lock.
func (l *Locker) IsLocked(ctx context.Context, name string) (bool, error) {
	n, err := l.rdb.Exists(ctx, KeyLock(name)).Result()
	return n == 1, err
}

// ReleaseToken releases a lock by name+token (when the Lock struct was not kept,
// e.g. the token was stored in a dispatch offer hash).
func (l *Locker) ReleaseToken(ctx context.Context, name, token string) error {
	return ReleaseLockScript.Run(ctx, l.rdb, []string{KeyLock(name)}, token).Err()
}

// WithLock runs fn while holding the lock.
func (l *Locker) WithLock(ctx context.Context, name string, ttl time.Duration, fn func(context.Context) error) error {
	lk, err := l.Acquire(ctx, name, ttl)
	if err != nil {
		return err
	}
	defer lk.Release(context.WithoutCancel(ctx))
	return fn(ctx)
}

// Release deletes the lock if we still own it (atomic via release_lock.lua).
func (lk *Lock) Release(ctx context.Context) error {
	return ReleaseLockScript.Run(ctx, lk.rdb, []string{lk.Key}, lk.Token).Err()
}

// Extend pushes the expiry if we still own the lock.
func (lk *Lock) Extend(ctx context.Context, ttl time.Duration) (bool, error) {
	n, err := extendLockScript.Run(ctx, lk.rdb, []string{lk.Key}, lk.Token, ttl.Milliseconds()).Int64()
	return n == 1, err
}

// ---------------------------------------------------------------------------
// Slots: temporary holds + permanent bookings on a bitmap (reserve_seat.lua).
// One abstraction serves bus seats, self-drive/tourist calendar days, premium
// hourly slots and test-drive half-hour slots.
//   hold    -> SET <bitmap>:hold:<slot> owner EX ttl   (temporary lock during payment)
//   confirm -> SETBIT <bitmap> slot 1                  (permanent)
//   release -> drop our holds;  cancel -> clear our confirmed bits
// ---------------------------------------------------------------------------

type Slots struct{ rdb *redis.Client }

func NewSlots(rdb *redis.Client) *Slots { return &Slots{rdb: rdb} }

func (s *Slots) run(ctx context.Context, mode, bitmap, owner string, ttl time.Duration, capacity int, slots []int) error {
	if len(slots) == 0 {
		return apperr.BadRequest("no slots requested")
	}
	args := []any{mode, owner, int64(ttl.Seconds()), capacity}
	for _, sl := range slots {
		args = append(args, sl)
	}
	res, err := ReserveSeatScript.Run(ctx, s.rdb, []string{bitmap}, args...).Int64Slice()
	if err != nil {
		return err
	}
	switch res[0] {
	case 1:
		return nil
	case 0:
		return apperr.Conflict(fmt.Sprintf("slot %d is already booked or held", res[1]))
	case -1:
		return apperr.BadRequest(fmt.Sprintf("slot index out of range (capacity %d)", capacity))
	case -2:
		return apperr.Conflict(fmt.Sprintf("slot %d hold expired or owned by someone else", res[1]))
	}
	return fmt.Errorf("reserve_seat: unexpected reply %v", res)
}

// Hold temporarily locks slots for owner (all-or-nothing).
func (s *Slots) Hold(ctx context.Context, bitmap, owner string, capacity int, ttl time.Duration, slots ...int) error {
	return s.run(ctx, "hold", bitmap, owner, ttl, capacity, slots)
}

// Confirm converts owner's holds into permanent bookings; keyTTL>0 expires the bitmap later.
func (s *Slots) Confirm(ctx context.Context, bitmap, owner string, capacity int, keyTTL time.Duration, slots ...int) error {
	if err := s.run(ctx, "confirm", bitmap, owner, 0, capacity, slots); err != nil {
		return err
	}
	if keyTTL > 0 {
		pipe := s.rdb.Pipeline()
		pipe.Expire(ctx, bitmap, keyTTL)
		pipe.Expire(ctx, bitmap+":owner", keyTTL)
		_, err := pipe.Exec(ctx)
		return err
	}
	return nil
}

// Release drops owner's temporary holds.
func (s *Slots) Release(ctx context.Context, bitmap, owner string, capacity int, slots ...int) error {
	return s.run(ctx, "release", bitmap, owner, 0, capacity, slots)
}

// Cancel frees owner's confirmed slots.
func (s *Slots) Cancel(ctx context.Context, bitmap, owner string, capacity int, slots ...int) error {
	return s.run(ctx, "cancel", bitmap, owner, 0, capacity, slots)
}

// Free returns which of the given slots are neither booked nor held.
func (s *Slots) Free(ctx context.Context, bitmap string, slots ...int) ([]bool, error) {
	pipe := s.rdb.Pipeline()
	bits := make([]*redis.IntCmd, len(slots))
	holds := make([]*redis.IntCmd, len(slots))
	for i, sl := range slots {
		bits[i] = pipe.GetBit(ctx, bitmap, int64(sl))
		holds[i] = pipe.Exists(ctx, bitmap+":hold:"+strconv.Itoa(sl))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, err
	}
	out := make([]bool, len(slots))
	for i := range slots {
		out[i] = bits[i].Val() == 0 && holds[i].Val() == 0
	}
	return out, nil
}

// BookedCount returns the number of confirmed slots (BITCOUNT).
func (s *Slots) BookedCount(ctx context.Context, bitmap string) (int64, error) {
	return s.rdb.BitCount(ctx, bitmap, nil).Result()
}

// AllFree reports whether every slot is free.
func (s *Slots) AllFree(ctx context.Context, bitmap string, slots ...int) (bool, error) {
	free, err := s.Free(ctx, bitmap, slots...)
	if err != nil {
		return false, err
	}
	for _, f := range free {
		if !f {
			return false, nil
		}
	}
	return true, nil
}
