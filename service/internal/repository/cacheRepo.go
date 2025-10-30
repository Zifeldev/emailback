package repository

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/redis/go-redis/v9"
)

// CacheEmailRepo is a cache-aside decorator for EmailRepository backed by Redis.
type CacheEmailRepo struct {
    underlying EmailRepository
    rdb        *redis.Client
    prefix     string
    ttl        time.Duration
}

func NewCacheEmailRepo(under EmailRepository, rdb *redis.Client, prefix string, ttl time.Duration) *CacheEmailRepo {
    return &CacheEmailRepo{underlying: under, rdb: rdb, prefix: prefix, ttl: ttl}
}

func (c *CacheEmailRepo) cacheKeyByID(id string) string {
    return fmt.Sprintf("%semail:id:%s", c.prefix, id)
}

// SaveEmail delegates to underlying and invalidates relevant cache keys.
func (c *CacheEmailRepo) SaveEmail(ctx context.Context, email *EmailEntity) error {
    if err := c.underlying.SaveEmail(ctx, email); err != nil {
        return err
    }
    // Best-effort invalidation; ignore errors
    _ = c.rdb.Del(ctx, c.cacheKeyByID(email.ID)).Err()
    return nil
}

// GetByID returns entity from cache first; falls back to DB and populates cache.
func (c *CacheEmailRepo) GetByID(ctx context.Context, id string) (*EmailEntity, error) {
    key := c.cacheKeyByID(id)
    if c.rdb != nil {
        if bs, err := c.rdb.Get(ctx, key).Bytes(); err == nil && len(bs) > 0 {
            var e EmailEntity
            if json.Unmarshal(bs, &e) == nil {
                return &e, nil
            }
        }
    }
    e, err := c.underlying.GetByID(ctx, id)
    if err != nil {
        return nil, err
    }
    if c.rdb != nil && e != nil {
        if bs, err := json.Marshal(e); err == nil {
            _ = c.rdb.Set(ctx, key, bs, c.ttl).Err()
        }
    }
    return e, nil
}

// GetAll is not cached by default (pagination + freshness). Delegates to underlying.
func (c *CacheEmailRepo) GetAll(ctx context.Context, limit, offset int) ([]*EmailEntity, error) {
    return c.underlying.GetAll(ctx, limit, offset)
}
