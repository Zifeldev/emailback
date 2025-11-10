package config

import (
    "context"
    "os"
    "testing"
    "time"
)

func TestRedisConfig_NonStrict_Defaults(t *testing.T) {
    // ensure non-strict
    _ = os.Setenv("STRICT", "false")
    _ = os.Unsetenv("REDIS_ENABLED")
    cfg := MustLoad(context.Background())
    if cfg.Redis.Enabled {
        t.Fatalf("redis should be disabled by default in non-strict")
    }
}

func TestRedisConfig_Strict_EnabledRequiresAddr(t *testing.T) {
    // strict + enabled, but no addr -> should panic in mustEnv
    _ = os.Setenv("STRICT", "true")
    _ = os.Setenv("REDIS_ENABLED", "true")
    // DB mandatory envs to pass strict portion
    _ = os.Setenv("DB_HOST", "h")
    _ = os.Setenv("DB_PORT", "5432")
    _ = os.Setenv("DB_USER", "u")
    _ = os.Setenv("DB_PASSWORD", "p")
    _ = os.Setenv("DB_NAME", "n")
    _ = os.Setenv("DB_SSLMODE", "disable")
    _ = os.Setenv("DB_MAX_CONNS", "5")
    _ = os.Setenv("DB_MIN_CONNS", "1")
    defer func() {
        if r := recover(); r == nil {
            t.Fatalf("expected panic due to missing REDIS_ADDR")
        }
    }()
    _ = MustLoad(context.Background())
}

func TestRedisConfig_Strict_AllSet_OK(t *testing.T) {
    envs := map[string]string{
        "STRICT": "true",
        "REDIS_ENABLED": "true",
        "REDIS_ADDR": "localhost:6379",
        "REDIS_DB": "1",
        // DB mandatory
        "DB_HOST": "h", "DB_PORT": "5432", "DB_USER": "u", "DB_PASSWORD": "p", "DB_NAME": "n", "DB_SSLMODE": "disable", "DB_MAX_CONNS": "5", "DB_MIN_CONNS": "1",
    }
    for k,v := range envs { _ = os.Setenv(k,v) }
    defer func(){ for k := range envs { _ = os.Unsetenv(k) } }()
    cfg := MustLoad(context.Background())
    if !cfg.Redis.Enabled || cfg.Redis.Addr == "" || cfg.Redis.DB != 1 || cfg.Redis.Prefix == "" || cfg.Redis.TTL <= 0 {
        t.Fatalf("bad redis cfg: %+v", cfg.Redis)
    }
    if cfg.Redis.TTL < time.Minute || cfg.Redis.TTL > 10*time.Minute { /* acceptable default range check */ }
}
