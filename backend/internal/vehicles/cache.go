package vehicles

import (
	"context"
	"strings"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
)

// Cache keeps car specs and the "live scrolling" model autocomplete index in Redis.
//
//	sr:vehicle:spec:<model>       STRING json   (7 days)
//	sr:vehicle:spec:miss:<model>  STRING "1"    (10 min negative cache for unknown models)
//	sr:vehicle:models             ZSET score 0  lexicographic index: "<suffix>|<model>"
type Cache struct {
	cs  *cache.Service
	rdb *redis.Client
}

func NewCache(cs *cache.Service) *Cache { return &Cache{cs: cs, rdb: cs.Client()} }

func (c *Cache) Get(ctx context.Context, model string) (*Spec, bool, error) {
	var s Spec
	found, err := c.cs.GetJSON(ctx, cache.KeyCarSpec(model), &s)
	if err != nil || !found {
		return nil, false, err
	}
	return &s, true, nil
}

func (c *Cache) Set(ctx context.Context, s *Spec) error {
	pipe := c.rdb.Pipeline()
	pipe.Del(ctx, cache.KeyCarSpecMissing(s.Model))
	if err := c.cs.SetJSON(ctx, cache.KeyCarSpec(s.Model), s, cache.TTLCarSpec); err != nil {
		return err
	}
	c.indexPipe(ctx, pipe, s.Model)
	_, err := pipe.Exec(ctx)
	return err
}

// Invalidate drops a spec (after an admin update) and dependent map envelopes.
func (c *Cache) Invalidate(ctx context.Context, model string) error {
	return c.cs.Del(ctx, cache.KeyCarSpec(model), cache.KeyCarEnvelope(model), cache.KeyRoadCompat(model))
}

func (c *Cache) MarkMissing(ctx context.Context, model string) error {
	return c.rdb.Set(ctx, cache.KeyCarSpecMissing(model), "1", cache.TTLCarSpecMissing).Err()
}

func (c *Cache) IsMissing(ctx context.Context, model string) bool {
	n, _ := c.rdb.Exists(ctx, cache.KeyCarSpecMissing(model)).Result()
	return n == 1
}

// Index adds models to the autocomplete index. Each word-suffix is indexed so
// "swift" finds "maruti suzuki swift" as the user scrolls/types.
func (c *Cache) Index(ctx context.Context, models ...string) error {
	pipe := c.rdb.Pipeline()
	for _, m := range models {
		c.indexPipe(ctx, pipe, m)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (c *Cache) indexPipe(ctx context.Context, pipe redis.Pipeliner, model string) {
	words := strings.Fields(model)
	members := make([]redis.Z, 0, len(words))
	for i := range words {
		suffix := strings.Join(words[i:], " ")
		members = append(members, redis.Z{Score: 0, Member: suffix + "|" + model})
	}
	pipe.ZAdd(ctx, cache.KeyCarModelIndex(), members...)
}

// Suggest returns up to limit models matching the typed prefix (ZRANGEBYLEX).
func (c *Cache) Suggest(ctx context.Context, prefix string, limit int) ([]string, error) {
	prefix = NormalizeModel(prefix)
	if prefix == "" {
		return []string{}, nil
	}
	raw, err := c.rdb.ZRangeByLex(ctx, cache.KeyCarModelIndex(), &redis.ZRangeBy{
		Min: "[" + prefix, Max: "[" + prefix + "\xff", Count: int64(limit * 3),
	}).Result()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := make([]string, 0, limit)
	for _, r := range raw {
		_, model, _ := strings.Cut(r, "|")
		if !seen[model] {
			seen[model] = true
			out = append(out, model)
			if len(out) == limit {
				break
			}
		}
	}
	return out, nil
}
