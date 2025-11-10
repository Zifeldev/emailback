package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
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

type RateLimitConfig struct {
	Enabled  bool
	Interval time.Duration
	Max      int
}

type AIConfig struct {
	Enabled             bool          `env:"AI_ENABLED" envDefault:"true"`
	HFToken             string        `env:"HF_TOKEN"`
	SummarizationModels string        `env:"AI_SUM_MODELS"`
	ClassificationModel string        `env:"AI_CLS_MODEL" envDefault:"facebook/bart-large-mnli"`
	Timeout             time.Duration `env:"AI_TIMEOUT" envDefault:"1m"`
}

func (a *AIConfig) ParseSumModels() (map[string]string, error) {
	defaultModels := map[string]string{
		"en": "sshleifer/distilbart-cnn-12-6",
		"ru": "IlyaGusev/mbart_ru_sum_gazeta",
		"de": "ml6team/mt5-small-german-finetune-mlsum",
	}

	if a.SummarizationModels == "" {
		return defaultModels, nil
	}

	var models map[string]string
	if err := json.Unmarshal([]byte(a.SummarizationModels), &models); err != nil {
		return nil, fmt.Errorf("failed to parse AI_SUM_MODELS: %w", err)
	}

	for lang, model := range defaultModels {
		if _, exists := models[lang]; !exists {
			models[lang] = model
		}
	}

	return models, nil
}

type JWTConfig struct {
	AccessSecret string
}

type Config struct {
	Strict    bool
	Database  DatabaseConfig
	HTTP      HTTPConfig
	Logger    LoggerConfig
	Redis     RedisConfig
	RateLimit RateLimitConfig
	AI        AIConfig
	JWT       JWTConfig
}

func MustLoad(_ context.Context) Config {
	var cfg Config

	cfg.Strict = getEnvBool("STRICT", false)
	cfg.HTTP = HTTPConfig{
		Host:            getEnv("HTTP_HOST", getEnv("HTTP_ADDR", ":8080")),
		ShutdownTimeout: getEnvDuration("HTTP_SHUTDOWN_TIMEOUT", 10*time.Second),
		RequestTimeout:  getEnvDuration("REQUEST_TIMEOUT", 5*time.Second),
	}
	if cfg.Strict {
		dbPass := os.Getenv("DB_PASSWORD")
		if dbPass == "" {
			if path, ok := os.LookupEnv("DB_PASSWORD_FILE"); ok && path != "" {
				if b, err := os.ReadFile(path); err == nil {
					dbPass = strings.TrimSpace(string(b))
				} else {
					panic(fmt.Errorf("failed to read DB_PASSWORD_FILE: %w", err))
				}
			} else {
				panic(errors.New("missing required env: DB_PASSWORD or DB_PASSWORD_FILE"))
			}
		}

		cfg.Database = DatabaseConfig{
			Host:              mustEnv("DB_HOST"),
			Port:              mustEnvInt("DB_PORT"),
			User:              mustEnv("DB_USER"),
			Password:          dbPass,
			Name:              mustEnv("DB_NAME"),
			SSLMode:           mustEnv("DB_SSLMODE"),
			MaxConns:          int32(mustEnvInt("DB_MAX_CONNS")),
			MinConns:          int32(mustEnvInt("DB_MIN_CONNS")),
			MaxConnLifetime:   getEnvDuration("DB_MAX_CONN_LIFETIME", 30*time.Minute),
			MaxConnIdleTime:   getEnvDuration("DB_MAX_CONN_IDLE_TIME", 5*time.Minute),
			HealthCheckPeriod: getEnvDuration("DB_HEALTH_CHECK_PERIOD", time.Minute),
			QueryTimeout:      getEnvDuration("DB_QUERY_TIMEOUT", 5*time.Second),
		}

		if rlEnabled := getEnvBool("RATE_LIMIT_ENABLED", false); rlEnabled {
			cfg.RateLimit = RateLimitConfig{
				Enabled:  true,
				Interval: getEnvDuration("RATE_LIMIT_INTERVAL", time.Minute),
				Max:      getEnvInt("RATE_LIMIT_MAX", 100),
			}
		}
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
		defaultPass := "123"
		if v, ok := os.LookupEnv("DB_PASSWORD"); ok && v != "" {
			defaultPass = v
		} else if path, ok := os.LookupEnv("DB_PASSWORD_FILE"); ok && path != "" {
			if b, err := os.ReadFile(path); err == nil {
				defaultPass = strings.TrimSpace(string(b))
			}
		}
		cfg.Database = DatabaseConfig{
			Host:              getEnv("DB_HOST", "localhost"),
			Port:              getEnvInt("DB_PORT", 5432),
			User:              getEnv("DB_USER", "postgres"),
			Password:          defaultPass,
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
		cfg.RateLimit = RateLimitConfig{
			Enabled:  getEnvBool("RATE_LIMIT_ENABLED", false),
			Interval: getEnvDuration("RATE_LIMIT_INTERVAL", time.Minute),
			Max:      getEnvInt("RATE_LIMIT_MAX", 100),
		}
	}
	cfg.Logger = LoggerConfig{
		Level: getEnv("LOGGER_LEVEL", getEnv("LOG_LEVEL", "info")),
	}
	cfg.AI = AIConfig{
		Enabled:             getEnvBool("AI_ENABLED", false),
		HFToken:             getEnv("HF_TOKEN", ""),
		SummarizationModels: getEnv("AI_SUM_MODELS", "facebook/bart-large-cnn"),
		ClassificationModel: getEnv("AI_CLS_MODEL", "facebook/bart-large-mnli"),
		Timeout:             getEnvDuration("AI_TIMEOUT", 10*time.Second),
	}

	cfg.JWT = JWTConfig{
		AccessSecret: getEnv("JWT_ACCESS_SECRET", ""),
	}

	if cfg.RateLimit.Interval <= 0 || cfg.RateLimit.Max <= 0 {
		cfg.RateLimit.Enabled = false
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
