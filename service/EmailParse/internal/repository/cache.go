package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

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
func (c *CacheEmailRepo) firstKeysSet() string {
	return fmt.Sprintf("%semails:first:keys", c.prefix)
}

func (c *CacheEmailRepo) SaveEmail(ctx context.Context, email *EmailEntity) error {
	if err := c.underlying.SaveEmail(ctx, email); err != nil {
		return err
	}
	if c.rdb != nil {
		_ = c.rdb.Del(ctx, c.cacheKeyByID(email.ID)).Err()
		keys, _ := c.rdb.SMembers(ctx, c.firstKeysSet()).Result()
		if len(keys) > 0 {
			_ = c.rdb.Del(ctx, keys...).Err()
		}
	}
	return nil
}

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

func (c *CacheEmailRepo) GetAll(ctx context.Context, limit, offset int) ([]*EmailEntity, error) {
	if c.rdb == nil || offset != 0 || limit <= 0 {
		return c.underlying.GetAll(ctx, limit, offset)
	}
	key := fmt.Sprintf("%semails:first:%d", c.prefix, limit)

	if data, err := c.rdb.Get(ctx, key).Bytes(); err == nil && len(data) > 0 {
		var list []*EmailEntity
		if jErr := json.Unmarshal(data, &list); jErr == nil {
			return list, nil
		}
		_ = c.rdb.Del(ctx, key).Err()
	}

	list, err := c.underlying.GetAll(ctx, limit, offset)
	if err != nil {
		return nil, err
	}

	if b, mErr := json.Marshal(list); mErr == nil {
		pipe := c.rdb.Pipeline()
		pipe.Set(ctx, key, b, c.ttl)
		pipe.SAdd(ctx, c.firstKeysSet(), key)
		_, _ = pipe.Exec(ctx)
	}
	return list, nil
}

// GetByUserID возвращает письма пользователя. Кеширование списков по пользователю
// опущено для простоты — делегируем вызов базовому репозиторию.
func (c *CacheEmailRepo) GetByUserID(ctx context.Context, userID string, limit, offset int) ([]*EmailEntity, error) {
	return c.underlying.GetByUserID(ctx, userID, limit, offset)
}

// UpdateAIFields updates AI-generated fields and invalidates cache for the email id.
func (c *CacheEmailRepo) UpdateAIFields(ctx context.Context, id string, summary *string, aiSumModel *string, priority *string, priorityScore *float64, aiClsModel *string, aiUpdatedAt *time.Time) error {
	if err := c.underlying.UpdateAIFields(ctx, id, summary, aiSumModel, priority, priorityScore, aiClsModel, aiUpdatedAt); err != nil {
		return err
	}
	if c.rdb != nil {
		_ = c.rdb.Del(ctx, c.cacheKeyByID(id)).Err()
	}
	return nil
}
