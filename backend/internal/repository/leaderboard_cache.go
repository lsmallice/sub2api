package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const leaderboardOverviewCachePrefix = "leaderboard:overview:v1:"

type leaderboardCache struct {
	rdb *redis.Client
}

func NewLeaderboardCache(rdb *redis.Client) service.LeaderboardCache {
	return &leaderboardCache{rdb: rdb}
}

func (c *leaderboardCache) GetOverview(ctx context.Context, key string) (string, error) {
	if c == nil || c.rdb == nil {
		return "", redis.Nil
	}
	value, err := c.rdb.Get(ctx, leaderboardOverviewCachePrefix+key).Result()
	if errors.Is(err, redis.Nil) {
		return "", err
	}
	return value, err
}

func (c *leaderboardCache) SetOverview(ctx context.Context, key string, payload string, ttl time.Duration) error {
	if c == nil || c.rdb == nil || ttl <= 0 {
		return nil
	}
	return c.rdb.Set(ctx, leaderboardOverviewCachePrefix+key, payload, ttl).Err()
}

func (c *leaderboardCache) DeleteOverview(ctx context.Context, key string) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Del(ctx, leaderboardOverviewCachePrefix+key).Err()
}

func (c *leaderboardCache) DeleteAll(ctx context.Context) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	var cursor uint64
	for {
		keys, next, err := c.rdb.Scan(ctx, cursor, leaderboardOverviewCachePrefix+"*", 200).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := c.rdb.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			return nil
		}
	}
}
