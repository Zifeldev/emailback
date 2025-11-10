package config

import (
	"context"
	"os"
	"testing"
	"time"
)

func withEnv(k, v string, f func()) {
	old, had := os.LookupEnv(k)
	_ = os.Setenv(k, v)
	defer func() {
		if had {
			_ = os.Setenv(k, old)
		} else {
			_ = os.Unsetenv(k)
		}
	}()
	f()
}

func TestMustLoad_Default(t *testing.T) {
	withEnv("STRICT", "false", func() {
		cfg := MustLoad(context.Background())
		if cfg.HTTP.Host == "" {
			t.Fatalf("expected default HTTP host set")
		}
		if cfg.Database.Port == 0 {
			t.Fatalf("expected default DB port set")
		}
		if cfg.Logger.Level == "" {
			t.Fatalf("expected default logger level set")
		}
	})
}

func TestMustLoad_Strict_OK(t *testing.T) {
	// Provide all required envs so MustLoad does not panic
	envs := map[string]string{
		"STRICT":                 "true",
		"DB_HOST":                "localhost",
		"DB_PORT":                "5432",
		"DB_USER":                "user",
		"DB_PASSWORD":            "pass",
		"DB_NAME":                "db",
		"DB_SSLMODE":             "disable",
		"DB_MAX_CONNS":           "10",
		"DB_MIN_CONNS":           "1",
		"DB_MAX_CONN_LIFETIME":   "30m",
		"DB_MAX_CONN_IDLE_TIME":  "5m",
		"DB_HEALTH_CHECK_PERIOD": "1m",
		"HTTP_SHUTDOWN_TIMEOUT":  "5s",
	}
	// set all
	restore := make(map[string]*string)
	for k, v := range envs {
		if old, ok := os.LookupEnv(k); ok {
			restore[k] = &old
		} else {
			restore[k] = nil
		}
		_ = os.Setenv(k, v)
	}
	defer func() {
		for k, v := range restore {
			if v == nil {
				_ = os.Unsetenv(k)
			} else {
				_ = os.Setenv(k, *v)
			}
		}
	}()

	cfg := MustLoad(context.Background())
	if !cfg.Strict {
		t.Fatalf("expected strict mode")
	}
	if cfg.Database.Host != "localhost" || cfg.Database.Port != 5432 {
		t.Fatalf("db config mismatch")
	}
	if cfg.HTTP.ShutdownTimeout != 5*time.Second {
		t.Fatalf("http timeout mismatch: %v", cfg.HTTP.ShutdownTimeout)
	}
}

func TestGetEnvHelpers_DefaultsAndInvalid(t *testing.T) {
	// getEnv uses default when unset
	if v := getEnv("UNKNOWN_ENV_X", "def"); v != "def" {
		t.Fatalf("getEnv default failed: %q", v)
	}
	// getEnvInt falls back to default on invalid
	_ = os.Setenv("ENV_INT_X", "notint")
	if v := getEnvInt("ENV_INT_X", 42); v != 42 {
		t.Fatalf("getEnvInt default on invalid: %d", v)
	}
	// getEnvDuration falls back to default on invalid
	_ = os.Setenv("ENV_DUR_X", "zzz")
	if d := getEnvDuration("ENV_DUR_X", time.Second); d != time.Second {
		t.Fatalf("getEnvDuration default on invalid: %v", d)
	}
}

func TestMustEnv_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("mustEnv expected panic")
		}
	}()
	_ = os.Unsetenv("NEEDED")
	_ = mustEnv("NEEDED")
}

func TestMustEnvInt_PanicsOnNonInt(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("mustEnvInt expected panic")
		}
	}()
	_ = os.Setenv("MUST_INT", "nope")
	_ = mustEnvInt("MUST_INT")
}
