package config

import (
	"os"

	"github.com/caarlos0/env/v11"
)

// Config holds all service configuration
type Config struct {
	// Server
	Port string `env:"PORT" envDefault:"8080"`

	// Redis
	RedisAddr       string `env:"REDIS_ADDR" envDefault:"localhost:6379"`
	RedisPassword   string `env:"REDIS_PASSWORD" envDefault:""`
	RedisDB         int    `env:"REDIS_DB" envDefault:"0"`
	RedisMaxRetries int    `env:"REDIS_MAX_RETRIES" envDefault:"3"`

	// Logging
	LogLevel  string `env:"LOG_LEVEL" envDefault:"info"`
	LogFormat string `env:"LOG_FORMAT" envDefault:"json"`
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}

	// Allow overriding config via env vars
	if p := os.Getenv("PORT"); p != "" {
		cfg.Port = p
	}
	if r := os.Getenv("REDIS_ADDR"); r != "" {
		cfg.RedisAddr = r
	}
	if pw := os.Getenv("REDIS_PASSWORD"); pw != "" {
		cfg.RedisPassword = pw
	}
	if db := os.Getenv("REDIS_DB"); db != "" {
		// parsed already, but envDefault handles it
		_ = db
	}
	if ll := os.Getenv("LOG_LEVEL"); ll != "" {
		cfg.LogLevel = ll
	}
	if lf := os.Getenv("LOG_FORMAT"); lf != "" {
		cfg.LogFormat = lf
	}

	return cfg, nil
}
