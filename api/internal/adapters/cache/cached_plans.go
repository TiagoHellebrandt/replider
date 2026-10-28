package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/replider/api/internal/core/domain"
)

const (
	planByIDKey  = "plan:%s"
	planListKey  = "plans:all"
	planCacheTTL = 1 * time.Hour
)

// CachedPlanRepository wraps a PlanRepository and caches results in Redis.
// Falls through to the underlying repo on cache miss or Redis failure.
type CachedPlanRepository struct {
	inner domain.PlanRepository
	cache *RedisCache
}

func NewCachedPlanRepository(inner domain.PlanRepository, cache *RedisCache) *CachedPlanRepository {
	return &CachedPlanRepository{inner: inner, cache: cache}
}

func (r *CachedPlanRepository) GetByID(ctx context.Context, id string) (*domain.Plan, error) {
	key := fmt.Sprintf(planByIDKey, id)

	var plan domain.Plan
	if err := r.cache.Get(ctx, key, &plan); err == nil {
		return &plan, nil
	}

	result, err := r.inner.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if result != nil {
		_ = r.cache.Set(ctx, key, result, planCacheTTL)
	}
	return result, nil
}

// GetByStripePriceID passes through to the inner repo uncached. It runs
// only on billing webhooks (low volume), and caching a price→plan miss
// could mask a just-fixed stripe_price_id for up to the TTL.
func (r *CachedPlanRepository) GetByStripePriceID(ctx context.Context, priceID string) (*domain.Plan, error) {
	return r.inner.GetByStripePriceID(ctx, priceID)
}

func (r *CachedPlanRepository) ListAll(ctx context.Context) ([]*domain.Plan, error) {
	var plans []*domain.Plan
	if err := r.cache.Get(ctx, planListKey, &plans); err == nil && plans != nil {
		return plans, nil
	}

	plans, err := r.inner.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	_ = r.cache.Set(ctx, planListKey, plans, planCacheTTL)
	return plans, nil
}
