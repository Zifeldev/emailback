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

type JWTConfig struct {
	AccessSecret      string
	RefreshSecret     string
	AccessExpiration  time.Duration
	RefreshExpiration time.Duration
	Issuer            string
	FirstAdminEmail   string 
}

type RateLimitConfig struct {
	Enabled  bool
	Interval time.Duration
	Max      int
}

type Config struct {
	Database  DatabaseConfig
	HTTP      HTTPConfig
	Logger    LoggerConfig
	Redis     RedisConfig
	JWT       JWTConfig
	RateLimit RateLimitConfig
}

func Load(ctx context.Context) (*Config, error) {
	cfg := &Config{}

	// Database
	port, err := strconv.Atoi(getEnv("DB_PORT", "5432"))
	if err != nil {
		return nil, fmt.Errorf("invalid DB_PORT: %w", err)
	}

	maxConns, err := strconv.ParseInt(getEnv("DB_MAX_CONNS", "25"), 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid DB_MAX_CONNS: %w", err)
	}

	minConns, err := strconv.ParseInt(getEnv("DB_MIN_CONNS", "5"), 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid DB_MIN_CONNS: %w", err)
	}

	queryTimeout, err := time.ParseDuration(getEnv("DB_QUERY_TIMEOUT", "5s"))
	if err != nil {
		return nil, fmt.Errorf("invalid DB_QUERY_TIMEOUT: %w", err)
	}

	cfg.Database = DatabaseConfig{
		Host:              getEnv("DB_HOST", "localhost"),
		Port:              port,
		User:              getEnv("DB_USER", "auth"),
		Password:          getEnv("DB_PASSWORD", ""),
		Name:              getEnv("DB_NAME", "auth"),
		SSLMode:           getEnv("DB_SSLMODE", "disable"),
		MaxConns:          int32(maxConns),
		MinConns:          int32(minConns),
		MaxConnLifetime:   time.Hour,
		MaxConnIdleTime:   30 * time.Minute,
		HealthCheckPeriod: time.Minute,
		QueryTimeout:      queryTimeout,
	}

	// HTTP
	shutdownTimeout, err := time.ParseDuration(getEnv("SHUTDOWN_TIMEOUT", "10s"))
	if err != nil {
		return nil, fmt.Errorf("invalid SHUTDOWN_TIMEOUT: %w", err)
	}

	requestTimeout, err := time.ParseDuration(getEnv("REQUEST_TIMEOUT", "30s"))
	if err != nil {
		return nil, fmt.Errorf("invalid REQUEST_TIMEOUT: %w", err)
	}

	cfg.HTTP = HTTPConfig{
		Host:            getEnv("HTTP_HOST", ":8081"),
		ShutdownTimeout: shutdownTimeout,
		RequestTimeout:  requestTimeout,
	}

	// Logger
	cfg.Logger = LoggerConfig{
		Level: getEnv("LOG_LEVEL", "info"),
	}

	// Redis
	redisDB, err := strconv.Atoi(getEnv("REDIS_DB", "1"))
	if err != nil {
		return nil, fmt.Errorf("invalid REDIS_DB: %w", err)
	}

	redisTTL, err := time.ParseDuration(getEnv("REDIS_TTL", "24h"))
	if err != nil {
		return nil, fmt.Errorf("invalid REDIS_TTL: %w", err)
	}

	cfg.Redis = RedisConfig{
		Enabled:  getEnv("REDIS_ENABLED", "true") == "true",
		Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
		Password: getEnv("REDIS_PASSWORD", ""),
		DB:       redisDB,
		Prefix:   getEnv("REDIS_PREFIX", "auth:"),
		TTL:      redisTTL,
	}

	// JWT
	accessExpiration, err := time.ParseDuration(getEnv("JWT_ACCESS_EXPIRATION", "15m"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_ACCESS_EXPIRATION: %w", err)
	}

	refreshExpiration, err := time.ParseDuration(getEnv("JWT_REFRESH_EXPIRATION", "24h"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_REFRESH_EXPIRATION: %w", err)
	}

	accessSecret := getEnv("JWT_ACCESS_SECRET", "")
	if accessSecret == "" {
		return nil, errors.New("JWT_ACCESS_SECRET is required")
	}

	refreshSecret := getEnv("JWT_REFRESH_SECRET", "")
	if refreshSecret == "" {
		return nil, errors.New("JWT_REFRESH_SECRET is required")
	}

	cfg.JWT = JWTConfig{
		AccessSecret:      accessSecret,
		RefreshSecret:     refreshSecret,
		AccessExpiration:  accessExpiration,
		RefreshExpiration: refreshExpiration,
		Issuer:            getEnv("JWT_ISSUER", "emailback-auth"),
		FirstAdminEmail:   getEnv("FIRST_ADMIN_EMAIL", ""),
	}

	// Rate Limit
	rateLimitInterval, err := time.ParseDuration(getEnv("RATE_LIMIT_INTERVAL", "1m"))
	if err != nil {
		return nil, fmt.Errorf("invalid RATE_LIMIT_INTERVAL: %w", err)
	}

	rateLimitMax, err := strconv.Atoi(getEnv("RATE_LIMIT_MAX", "100"))
	if err != nil {
		return nil, fmt.Errorf("invalid RATE_LIMIT_MAX: %w", err)
	}

	cfg.RateLimit = RateLimitConfig{
		Enabled:  getEnv("RATE_LIMIT_ENABLED", "false") == "true",
		Interval: rateLimitInterval,
		Max:      rateLimitMax,
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
