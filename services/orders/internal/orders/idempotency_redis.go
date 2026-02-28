package orders

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisIdempotencyStore struct {
	client *redis.Client
	prefix string
}

func NewRedisIdempotencyStore(client *redis.Client, prefix string) *RedisIdempotencyStore {
	return &RedisIdempotencyStore{
		client: client,
		prefix: prefix,
	}
}

func (s *RedisIdempotencyStore) Reserve(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return s.client.SetNX(ctx, s.prefix+key, "1", ttl).Result()
}
