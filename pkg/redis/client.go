package redis

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// Client wraps the Redis client with convenience methods
type Client struct {
	client *redis.Client
}

// Config holds Redis connection configuration
type Config struct {
	Addr       string
	Password   string
	DB         int
	MaxRetries int
}

// NewClient creates a new Redis client
func NewClient(cfg Config) (*Client, error) {
	opts := &redis.Options{
		Addr:       cfg.Addr,
		Password:   cfg.Password,
		DB:         cfg.DB,
		MaxRetries: cfg.MaxRetries,
	}

	client := redis.NewClient(opts)
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &Client{client: client}, nil
}

// Close closes the Redis connection
func (c *Client) Close() error {
	return c.client.Close()
}

// Client returns the underlying go-redis client
func (c *Client) Client() *redis.Client {
	return c.client
}

// IsConnected checks if the Redis connection is alive
func (c *Client) IsConnected(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// Do executes a raw command
func (c *Client) Do(ctx context.Context, args ...interface{}) interface{} {
	return c.client.Do(ctx, args...).Val()
}
