// Package live holds the Redis-backed hot state: OTP challenges, rate limits,
// WebSocket tickets, live cab positions (hash + geo index), the realtime pub/sub bus
// and the simulator leader lock. Nothing here is a source of truth; PostgreSQL is.
package live

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	keyOTP      = "sarathi:otp:"
	keyRate     = "sarathi:rl:"
	keyTicket   = "sarathi:ws:"
	keyCabsLive = "sarathi:cabs:live"
	keyCabsGeo  = "sarathi:cabs:geo"
	keyPersist  = "sarathi:cab:persist:"
	keyLeader   = "sarathi:sim:leader"

	// ChannelPrefix prefixes every realtime pub/sub channel.
	ChannelPrefix = "sarathi:rt:"
	// FleetTopic carries cab position updates for every connected client.
	FleetTopic = "fleet"

	otpTTL         = 5 * time.Minute
	otpMaxAttempts = 5
)

// UserTopic is the private topic for one user's trip and quota updates.
func UserTopic(userID string) string { return "user:" + userID }

// Errors returned by OTP verification.
var (
	ErrOTPInvalid = errors.New("otp invalid")
	ErrOTPExpired = errors.New("otp expired or not requested")
	ErrOTPLocked  = errors.New("too many otp attempts")
)

// Live wraps the Redis client.
type Live struct {
	RDB    *redis.Client
	secret []byte
}

// New connects lazily to Redis. secret keys the OTP HMAC.
func New(addr, password string, secret []byte) *Live {
	return &Live{
		RDB: redis.NewClient(&redis.Options{
			Addr: addr, Password: password,
			DialTimeout: 3 * time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second,
			PoolSize: 20, MinIdleConns: 2,
		}),
		secret: secret,
	}
}

// Close releases the connection pool.
func (l *Live) Close() error { return l.RDB.Close() }

// ---------- OTP ----------

func (l *Live) otpMAC(phone, code string) string {
	m := hmac.New(sha256.New, l.secret)
	m.Write([]byte("otp|" + phone + "|" + code))
	return hex.EncodeToString(m.Sum(nil))
}

// NewOTP creates a 6-digit challenge for phone, replacing any previous one. Only an
// HMAC of the code is stored.
func (l *Live) NewOTP(ctx context.Context, phone string) (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	code := fmt.Sprintf("%06d", n.Int64())
	key := keyOTP + phone
	pipe := l.RDB.TxPipeline()
	pipe.Del(ctx, key)
	pipe.HSet(ctx, key, "h", l.otpMAC(phone, code), "n", 0)
	pipe.Expire(ctx, key, otpTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return "", err
	}
	return code, nil
}

// OTPTTLSeconds is how long a challenge stays valid.
func OTPTTLSeconds() int { return int(otpTTL.Seconds()) }

// The attempt counter and comparison are one atomic script, so parallel guesses
// cannot exceed the attempt budget. A challenge is single-use.
var otpVerify = redis.NewScript(`
local h = redis.call('HGET', KEYS[1], 'h')
if not h then return -1 end
local n = redis.call('HINCRBY', KEYS[1], 'n', 1)
if n > tonumber(ARGV[2]) then redis.call('DEL', KEYS[1]) return -2 end
if h == ARGV[1] then redis.call('DEL', KEYS[1]) return 1 end
return 0`)

// VerifyOTP checks a code against the stored challenge.
func (l *Live) VerifyOTP(ctx context.Context, phone, code string) error {
	res, err := otpVerify.Run(ctx, l.RDB, []string{keyOTP + phone}, l.otpMAC(phone, code), otpMaxAttempts).Int()
	if err != nil {
		return err
	}
	switch res {
	case 1:
		return nil
	case -1:
		return ErrOTPExpired
	case -2:
		return ErrOTPLocked
	}
	return ErrOTPInvalid
}

// ---------- rate limiting ----------

var rateScript = redis.NewScript(`
local n = redis.call('INCR', KEYS[1])
if n == 1 then redis.call('PEXPIRE', KEYS[1], ARGV[1]) end
return {n, redis.call('PTTL', KEYS[1])}`)

// Allow implements a fixed-window counter. It returns whether the call is within
// limit and, if not, how long until the window resets.
func (l *Live) Allow(ctx context.Context, bucket string, limit int, window time.Duration) (bool, time.Duration, error) {
	res, err := rateScript.Run(ctx, l.RDB, []string{keyRate + bucket}, window.Milliseconds()).Int64Slice()
	if err != nil {
		return false, 0, err
	}
	if res[0] > int64(limit) {
		return false, time.Duration(max(res[1], 0)) * time.Millisecond, nil
	}
	return true, 0, nil
}

// ---------- WebSocket tickets ----------

// NewTicket issues a single-use, 30-second ticket that authenticates one WebSocket
// upgrade, so access tokens never appear in URLs.
func (l *Live) NewTicket(ctx context.Context, userID string) (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	t := hex.EncodeToString(b)
	return t, l.RDB.Set(ctx, keyTicket+t, userID, 30*time.Second).Err()
}

// RedeemTicket consumes a ticket and returns its user id ("" when unknown/expired).
func (l *Live) RedeemTicket(ctx context.Context, ticket string) (string, error) {
	id, err := l.RDB.GetDel(ctx, keyTicket+ticket).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil
	}
	return id, err
}

// ---------- cab positions ----------

// CabPos is a live cab position broadcast to clients.
type CabPos struct {
	ID      string  `json:"id"`
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
	Heading float64 `json:"heading"`
	Status  string  `json:"status"`
	TS      int64   `json:"ts"`
}

// UpdateCabs stores live positions, keeps the geo index limited to available cabs,
// and publishes the batch on the fleet topic.
func (l *Live) UpdateCabs(ctx context.Context, cabs []CabPos) error {
	if len(cabs) == 0 {
		return nil
	}
	pipe := l.RDB.Pipeline()
	for _, c := range cabs {
		if c.Status == "offline" {
			pipe.HDel(ctx, keyCabsLive, c.ID)
			pipe.ZRem(ctx, keyCabsGeo, c.ID)
			continue
		}
		b, _ := json.Marshal(c)
		pipe.HSet(ctx, keyCabsLive, c.ID, b)
		if c.Status == "available" {
			pipe.GeoAdd(ctx, keyCabsGeo, &redis.GeoLocation{Name: c.ID, Longitude: c.Lng, Latitude: c.Lat})
		} else {
			pipe.ZRem(ctx, keyCabsGeo, c.ID)
		}
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return err
	}
	return l.Publish(ctx, FleetTopic, map[string]any{"type": "fleet", "cabs": cabs})
}

// LiveCabs returns every live position keyed by cab id.
func (l *Live) LiveCabs(ctx context.Context) (map[string]CabPos, error) {
	raw, err := l.RDB.HGetAll(ctx, keyCabsLive).Result()
	if err != nil {
		return nil, err
	}
	out := make(map[string]CabPos, len(raw))
	for id, v := range raw {
		var c CabPos
		if json.Unmarshal([]byte(v), &c) == nil {
			out[id] = c
		}
	}
	return out, nil
}

// Cab returns one live position, or false when the cab has none.
func (l *Live) Cab(ctx context.Context, id string) (CabPos, bool) {
	var c CabPos
	v, err := l.RDB.HGet(ctx, keyCabsLive, id).Result()
	if err != nil || json.Unmarshal([]byte(v), &c) != nil {
		return c, false
	}
	return c, true
}

// NearestAvailable returns up to count available cab ids ordered nearest-first.
// Availability is re-checked under a row lock in PostgreSQL before assignment.
func (l *Live) NearestAvailable(ctx context.Context, lat, lng float64, count int) ([]string, error) {
	return l.RDB.GeoSearch(ctx, keyCabsGeo, &redis.GeoSearchQuery{
		Longitude: lng, Latitude: lat, Radius: 25, RadiusUnit: "km", Sort: "ASC", Count: count,
	}).Result()
}

// ShouldPersist throttles durable position writes to one per cab per interval.
func (l *Live) ShouldPersist(ctx context.Context, cabID string, every time.Duration) bool {
	ok, err := l.RDB.SetNX(ctx, keyPersist+cabID, 1, every).Result()
	return err == nil && ok
}

// ---------- pub/sub ----------

// Publish sends a JSON event to every API replica subscribed to the topic.
func (l *Live) Publish(ctx context.Context, topic string, event any) error {
	b, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return l.RDB.Publish(ctx, ChannelPrefix+topic, b).Err()
}

// ---------- leader lock ----------

var renewScript = redis.NewScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then return redis.call('PEXPIRE', KEYS[1], ARGV[2]) end
return 0`)

// AcquireLeader takes or renews the simulator leadership lease for this instance.
func (l *Live) AcquireLeader(ctx context.Context, instance string, ttl time.Duration) (bool, error) {
	ok, err := l.RDB.SetNX(ctx, keyLeader, instance, ttl).Result()
	if err != nil || ok {
		return ok, err
	}
	n, err := renewScript.Run(ctx, l.RDB, []string{keyLeader}, instance, ttl.Milliseconds()).Int()
	return n == 1, err
}

// Checker reports Redis readiness via the pooled client.
type Checker struct{ L *Live }

// Name implements httpapi.Checker.
func (Checker) Name() string { return "redis" }

// Check pings Redis.
func (c Checker) Check(ctx context.Context) error { return c.L.RDB.Ping(ctx).Err() }
