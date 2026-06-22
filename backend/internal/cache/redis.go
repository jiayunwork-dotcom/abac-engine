package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"abac-engine/internal/models"

	"github.com/go-redis/redis/v8"
)

type DecisionCache struct {
	client *redis.Client
	ttl    time.Duration
}

func NewDecisionCache(client *redis.Client) *DecisionCache {
	return &DecisionCache{
		client: client,
		ttl:    10 * time.Second,
	}
}

func (dc *DecisionCache) cacheKey(req *models.AccessRequest) string {
	data, _ := json.Marshal(req)
	hash := sha256.Sum256(data)
	return fmt.Sprintf("abac:decision:%s", hex.EncodeToString(hash[:]))
}

func (dc *DecisionCache) Get(ctx context.Context, req *models.AccessRequest) (*models.DecisionResult, error) {
	key := dc.cacheKey(req)
	val, err := dc.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var result models.DecisionResult
	if err := json.Unmarshal([]byte(val), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (dc *DecisionCache) Set(ctx context.Context, req *models.AccessRequest, result *models.DecisionResult) error {
	key := dc.cacheKey(req)
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return dc.client.Set(ctx, key, data, dc.ttl).Err()
}

type RateLimiter struct {
	client *redis.Client
}

func NewRateLimiter(client *redis.Client) *RateLimiter {
	return &RateLimiter{client: client}
}

func (rl *RateLimiter) Allow(ctx context.Context, tenantID string, maxRPS int) (bool, error) {
	now := time.Now().Unix()
	key := fmt.Sprintf("abac:quota:rps:%s:%d", tenantID, now)

	count, err := rl.client.Incr(ctx, key).Result()
	if err != nil {
		return false, err
	}
	if count == 1 {
		rl.client.Expire(ctx, key, 2*time.Second)
	}
	if count > int64(maxRPS) {
		return false, nil
	}
	return true, nil
}

func (rl *RateLimiter) CheckPolicyQuota(ctx context.Context, tenantID string, maxPolicies int) (bool, int, error) {
	key := fmt.Sprintf("abac:quota:policies:%s", tenantID)
	cached, err := rl.client.Get(ctx, key).Int()
	if err == nil {
		return cached < maxPolicies, cached, nil
	}
	return true, 0, nil
}

func (rl *RateLimiter) SetPolicyCount(ctx context.Context, tenantID string, count int) error {
	key := fmt.Sprintf("abac:quota:policies:%s", tenantID)
	return rl.client.Set(ctx, key, count, 5*time.Minute).Err()
}
