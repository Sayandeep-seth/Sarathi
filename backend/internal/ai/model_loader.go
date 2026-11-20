package ai

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"sarathi/internal/cache"
)

// ModelLoader tracks which model version is live. The version lives in Redis so
// every API pod switches together; a pub/sub message invalidates local copies.
//
//	sr:ai:model:<name>:version      STRING  active version
//	sr:ai:model:<name>:<version>    HASH    metadata (metrics, trained_at, …)
//	sr:ch:ai:model                  PUBSUB  {"name":..,"version":..}
type ModelLoader struct {
	rdb    *redis.Client
	locker *cache.Locker
	log    *slog.Logger

	mu       sync.RWMutex
	versions map[string]cachedVersion
}

type cachedVersion struct {
	version string
	at      time.Time
}

const (
	ModelRouteETA   = "route_eta"
	ModelRoadWidth  = "road_width"
	localVersionTTL = 30 * time.Second
)

func NewModelLoader(rdb *redis.Client, l *cache.Locker, log *slog.Logger) *ModelLoader {
	return &ModelLoader{rdb: rdb, locker: l, log: log, versions: map[string]cachedVersion{}}
}

// Version returns the active version of a model ("v1" if never published).
func (m *ModelLoader) Version(ctx context.Context, name string) string {
	m.mu.RLock()
	cv, ok := m.versions[name]
	m.mu.RUnlock()
	if ok && time.Since(cv.at) < localVersionTTL {
		return cv.version
	}
	v, err := m.rdb.Get(ctx, cache.KeyAIModelVersion(name)).Result()
	if err != nil {
		v = "v1"
	}
	m.mu.Lock()
	m.versions[name] = cachedVersion{version: v, at: time.Now()}
	m.mu.Unlock()
	return v
}

// Publish activates a new version (called by the training pipeline / admin).
func (m *ModelLoader) Publish(ctx context.Context, name, version string, meta map[string]any) error {
	return m.locker.WithLock(ctx, "ai:publish:"+name, 30*time.Second, func(ctx context.Context) error {
		pipe := m.rdb.TxPipeline()
		pipe.Set(ctx, cache.KeyAIModelVersion(name), version, 0)
		if len(meta) > 0 {
			pipe.HSet(ctx, cache.KeyAIModelMeta(name, version), meta)
		}
		msg, _ := json.Marshal(map[string]string{"name": name, "version": version})
		pipe.Publish(ctx, cache.ChanAIModel(), msg)
		_, err := pipe.Exec(ctx)
		return err
	})
}

// Watch listens for version changes and drops local copies immediately.
func (m *ModelLoader) Watch(ctx context.Context) {
	sub := m.rdb.Subscribe(ctx, cache.ChanAIModel())
	defer sub.Close()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-sub.Channel():
			if !ok {
				return
			}
			var ev struct{ Name, Version string }
			if json.Unmarshal([]byte(msg.Payload), &ev) == nil {
				m.mu.Lock()
				m.versions[ev.Name] = cachedVersion{version: ev.Version, at: time.Now()}
				m.mu.Unlock()
				m.log.Info("ai model switched", "model", ev.Name, "version", ev.Version)
			}
		}
	}
}
