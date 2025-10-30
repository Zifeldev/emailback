package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type DatabaseConfig struct {
	Host              string
	Port              int
	User              string
	Password          string
	Name              string
	SSLMode           string
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
	QueryTimeout      time.Duration
}

type HTTPConfig struct {
	Host            string
	ShutdownTimeout time.Duration
	RequestTimeout  time.Duration
}

type LoggerConfig struct {
	Level string
}

type RedisConfig struct {
	Enabled  bool
	Addr     string
	Password string
	DB       int
	Prefix   string
	TTL      time.Duration
}

type Config struct {
	Strict   bool
	Database DatabaseConfig
	HTTP     HTTPConfig
	Logger   LoggerConfig
	Redis    RedisConfig
}

func MustLoad(_ context.Context) Config {
	var cfg Config

	cfg.Strict = getEnvBool("STRICT", false)
	cfg.HTTP = HTTPConfig{
		Host:            getEnv("HTTP_HOST", ":8080"),
		ShutdownTimeout: getEnvDuration("HTTP_SHUTDOWN_TIMEOUT", 10*time.Second),
	}
	if cfg.Strict {
		cfg.Database = DatabaseConfig{
			Host:              mustEnv("DB_HOST"),
			Port:              mustEnvInt("DB_PORT"),
			User:              mustEnv("DB_USER"),
			Password:          mustEnv("DB_PASSWORD"),
			Name:              mustEnv("DB_NAME"),
			SSLMode:           mustEnv("DB_SSLMODE"),
			MaxConns:          int32(mustEnvInt("DB_MAX_CONNS")),
			MinConns:          int32(mustEnvInt("DB_MIN_CONNS")),
			MaxConnLifetime:   getEnvDuration("DB_MAX_CONN_LIFETIME", 30*time.Minute),
			MaxConnIdleTime:   getEnvDuration("DB_MAX_CONN_IDLE_TIME", 5*time.Minute),
			HealthCheckPeriod: getEnvDuration("DB_HEALTH_CHECK_PERIOD", time.Minute),
			QueryTimeout:      getEnvDuration("DB_QUERY_TIMEOUT", 5*time.Second),
		}
		// Redis in strict mode: parse if enabled, require address
		rcEnabled := getEnvBool("REDIS_ENABLED", false)
		if rcEnabled {
			cfg.Redis = RedisConfig{
				Enabled:  true,
				Addr:     mustEnv("REDIS_ADDR"),
				Password: getEnv("REDIS_PASSWORD", ""),
				DB:       mustEnvInt("REDIS_DB"),
				Prefix:   getEnv("REDIS_PREFIX", "emailback:"),
				TTL:      getEnvDuration("REDIS_TTL", 5*time.Minute),
			}
		} else {
			cfg.Redis = RedisConfig{Enabled: false}
		}
	} else {
		cfg.Database = DatabaseConfig{
			Host:              getEnv("DB_HOST", "localhost"),
			Port:              getEnvInt("DB_PORT", 5432),
			User:              getEnv("DB_USER", "postgres"),
			Password:          getEnv("DB_PASSWORD", "123"),
			Name:              getEnv("DB_NAME", "emaildb"),
			SSLMode:           getEnv("DB_SSLMODE", "disable"),
			MaxConns:          int32(getEnvInt("DB_MAX_CONNS", 20)),
			MinConns:          int32(getEnvInt("DB_MIN_CONNS", 2)),
			MaxConnLifetime:   getEnvDuration("DB_MAX_CONN_LIFETIME", 30*time.Minute),
			MaxConnIdleTime:   getEnvDuration("DB_MAX_CONN_IDLE_TIME", 5*time.Minute),
			HealthCheckPeriod: getEnvDuration("DB_HEALTH_CHECK_PERIOD", time.Minute),
			QueryTimeout:      getEnvDuration("DB_QUERY_TIMEOUT", 5*time.Second),
		}
		cfg.Redis = RedisConfig{
			Enabled:  getEnvBool("REDIS_ENABLED", false),
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
			Prefix:   getEnv("REDIS_PREFIX", "emailback:"),
			TTL:      getEnvDuration("REDIS_TTL", 5*time.Minute),
		}
	}
	cfg.Logger = LoggerConfig{
		Level: getEnv("LOGGER_LEVEL", "info"),
	}
	return cfg
}

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}
func getEnvBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		b, err := strconv.ParseBool(v)
		if err == nil {
			return b
		}
	}
	return def
}
func getEnvInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
func getEnvDuration(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func mustEnv(key string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	panic(errors.New("missing required env: " + key))
}
func mustEnvInt(key string) int {
	v := mustEnv(key)
	n, err := strconv.Atoi(v)
	if err != nil {
		panic(fmt.Errorf("env %s must be int: %w", key, err))
	}
	return n
}
