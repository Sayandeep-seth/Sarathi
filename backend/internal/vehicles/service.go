package vehicles

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"sarathi/internal/cache"
	apperr "sarathi/internal/common/errors"
)

// Provider fetches a spec live from an external catalogue when we don't know a model yet.
type Provider interface {
	Fetch(ctx context.Context, model string) (*Spec, error)
}

// HTTPProvider calls GET {baseURL}?model=<name> and expects a Spec JSON.
type HTTPProvider struct {
	baseURL string
	client  *http.Client
}

func NewHTTPProvider(baseURL string) *HTTPProvider {
	return &HTTPProvider{baseURL: baseURL, client: &http.Client{Timeout: 4 * time.Second}}
}

func (p *HTTPProvider) Fetch(ctx context.Context, model string) (*Spec, error) {
	if p.baseURL == "" {
		return nil, apperr.NotFound("no live spec provider configured")
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"?model="+url.QueryEscape(model), nil)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, apperr.NotFound("car model not found: " + model)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spec provider status %d", resp.StatusCode)
	}
	var s Spec
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return nil, err
	}
	s.Model, s.Source, s.UpdatedAt = model, "live", time.Now()
	return &s, nil
}

// Service resolves car specs: Redis -> Postgres -> live provider.
type Service struct {
	repo     *Repository
	cache    *Cache
	provider Provider
	locker   *cache.Locker
	log      *slog.Logger
}

func NewService(repo *Repository, c *Cache, p Provider, l *cache.Locker, log *slog.Logger) *Service {
	return &Service{repo: repo, cache: c, provider: p, locker: l, log: log}
}

// GetSpec returns the exact dimensions/energy data of a car model.
func (s *Service) GetSpec(ctx context.Context, name string) (*Spec, error) {
	model := NormalizeModel(name)
	if model == "" {
		return nil, apperr.BadRequest("car name is required")
	}
	if spec, ok, err := s.cache.Get(ctx, model); err == nil && ok {
		return spec, nil
	}
	if s.cache.IsMissing(ctx, model) {
		return nil, apperr.NotFound("car model not found: " + name)
	}

	// Cross-instance single flight: only one pod hits Postgres/provider for a cold model.
	lk, err := s.locker.AcquireWait(ctx, "carspec:"+model, 10*time.Second, 3*time.Second)
	if err == nil {
		defer lk.Release(context.WithoutCancel(ctx))
		if spec, ok, _ := s.cache.Get(ctx, model); ok {
			return spec, nil
		}
	}

	spec, err := s.repo.Get(ctx, model)
	if apperr.IsNotFound(err) {
		spec, err = s.provider.Fetch(ctx, model)
		if err == nil {
			if verr := spec.Validate(); verr != nil {
				return nil, verr
			}
			if perr := s.repo.Upsert(ctx, spec); perr != nil {
				s.log.Warn("persist live spec failed", "model", model, "err", perr)
			}
		}
	}
	if apperr.IsNotFound(err) {
		_ = s.cache.MarkMissing(ctx, model)
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	if err := s.cache.Set(ctx, spec); err != nil {
		s.log.Warn("cache car spec failed", "model", model, "err", err)
	}
	return spec, nil
}

// Upsert stores/overwrites a spec (admin / data ingestion).
func (s *Service) Upsert(ctx context.Context, spec *Spec) error {
	spec.Model = NormalizeModel(spec.Model)
	spec.UpdatedAt = time.Now()
	if err := spec.Validate(); err != nil {
		return err
	}
	if err := s.repo.Upsert(ctx, spec); err != nil {
		return err
	}
	_ = s.cache.Invalidate(ctx, spec.Model)
	return s.cache.Set(ctx, spec)
}

// Suggest powers the car-name picker.
func (s *Service) Suggest(ctx context.Context, q string, limit int) ([]string, error) {
	if limit <= 0 || limit > 20 {
		limit = 10
	}
	return s.cache.Suggest(ctx, q, limit)
}

// WarmIndex loads every model from Postgres into the Redis autocomplete index.
func (s *Service) WarmIndex(ctx context.Context) error {
	models, err := s.repo.AllModels(ctx)
	if err != nil || len(models) == 0 {
		return err
	}
	return s.cache.Index(ctx, models...)
}
