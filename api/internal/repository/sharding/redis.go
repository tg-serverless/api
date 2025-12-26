package sharding

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"serverless-api/api/internal/model"
	"strconv"
)

type redisRepository struct {
	client       *redis.Client
	defaultCount uint32
}

func NewRedisRepository(client *redis.Client, defaultCount uint32) model.ShardingRepository {
	if defaultCount == 0 {
		defaultCount = 1
	}

	return &redisRepository{
		client:       client,
		defaultCount: defaultCount,
	}
}

func (r *redisRepository) SetCount(ctx context.Context, botID string, count uint32) error {
	key := r.makeKey(botID)
	val := strconv.FormatUint(uint64(count), 10)

	if err := r.client.Set(ctx, key, val, 0).Err(); err != nil {
		return fmt.Errorf("failed to set shard count: %w", err)
	}

	return nil
}

func (r *redisRepository) makeKey(botID string) string {
	return fmt.Sprintf("config:bots:%s:shards", botID)
}
