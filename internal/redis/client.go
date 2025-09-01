package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	saiTypes "github.com/saiset-co/sai-service/types"
	"github.com/saiset-co/sai-storage/types"
)

type Client struct {
	client *redis.Client
	config types.RedisConfig
}

func NewClient(config types.RedisConfig) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", config.Host, config.Port),
		Password:     config.Password,
		DB:           config.DB,
		DialTimeout:  time.Duration(config.Timeout) * time.Second,
		ReadTimeout:  time.Duration(config.Timeout) * time.Second,
		WriteTimeout: time.Duration(config.Timeout) * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.Timeout)*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, saiTypes.WrapError(err, "failed to connect to Redis")
	}

	return &Client{
		client: rdb,
		config: config,
	}, nil
}

func (c *Client) Get(ctx context.Context, key string) (string, error) {
	result, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", saiTypes.NewError("key not found")
		}
		return "", saiTypes.WrapError(err, "failed to get value")
	}
	return result, nil
}

func (c *Client) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	err := c.client.Set(ctx, key, value, ttl).Err()
	if err != nil {
		return saiTypes.WrapError(err, "failed to set value")
	}
	return nil
}

func (c *Client) Del(ctx context.Context, keys ...string) error {
	err := c.client.Del(ctx, keys...).Err()
	if err != nil {
		return saiTypes.WrapError(err, "failed to delete keys")
	}
	return nil
}

func (c *Client) Keys(ctx context.Context, pattern string) ([]string, error) {
	keys, err := c.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, saiTypes.WrapError(err, "failed to get keys")
	}
	return keys, nil
}

func (c *Client) Exists(ctx context.Context, keys ...string) (int64, error) {
	count, err := c.client.Exists(ctx, keys...).Result()
	if err != nil {
		return 0, saiTypes.WrapError(err, "failed to check existence")
	}
	return count, nil
}

func (c *Client) HSet(ctx context.Context, key string, values ...interface{}) error {
	err := c.client.HSet(ctx, key, values...).Err()
	if err != nil {
		return saiTypes.WrapError(err, "failed to set hash")
	}
	return nil
}

func (c *Client) HGet(ctx context.Context, key, field string) (string, error) {
	result, err := c.client.HGet(ctx, key, field).Result()
	if err != nil {
		if err == redis.Nil {
			return "", saiTypes.NewError("field not found")
		}
		return "", saiTypes.WrapError(err, "failed to get hash field")
	}
	return result, nil
}

func (c *Client) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	result, err := c.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, saiTypes.WrapError(err, "failed to get all hash fields")
	}
	return result, nil
}

func (c *Client) HDel(ctx context.Context, key string, fields ...string) error {
	err := c.client.HDel(ctx, key, fields...).Err()
	if err != nil {
		return saiTypes.WrapError(err, "failed to delete hash fields")
	}
	return nil
}

func (c *Client) Pipeline() redis.Pipeliner {
	return c.client.Pipeline()
}

func (c *Client) Ping(ctx context.Context) error {
	err := c.client.Ping(ctx).Err()
	if err != nil {
		return saiTypes.WrapError(err, "failed to ping Redis")
	}
	return nil
}

func (c *Client) Close() error {
	err := c.client.Close()
	if err != nil {
		return saiTypes.WrapError(err, "failed to close Redis connection")
	}
	return nil
}