package service

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	blacklistKeyPrefix = "blacklist:token:"
)

type TokenBlacklistService struct {
	redis  *redis.Client
	prefix string
}

func NewTokenBlacklistService(redisClient *redis.Client, prefix string) *TokenBlacklistService {
	return &TokenBlacklistService{
		redis:  redisClient,
		prefix: prefix,
	}
}


func (s *TokenBlacklistService) BlacklistToken(ctx context.Context, jti string, userID int64, ttl time.Duration, reason string) error {
	if jti == "" {
		return fmt.Errorf("jti cannot be empty")
	}

	key := s.getKey(jti)

	value := fmt.Sprintf(`{"user_id":%d,"reason":"%s","blacklisted_at":"%s"}`,
		userID, reason, time.Now().Format(time.RFC3339))

	err := s.redis.Set(ctx, key, value, ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to blacklist token: %w", err)
	}

	return nil
}

func (s *TokenBlacklistService) IsBlacklisted(ctx context.Context, jti string) (bool, error) {
	if jti == "" {
		return false, nil
	}

	key := s.getKey(jti)

	exists, err := s.redis.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check blacklist: %w", err)
	}

	return exists > 0, nil
}

func (s *TokenBlacklistService) RemoveToken(ctx context.Context, jti string) error {
	if jti == "" {
		return nil
	}

	key := s.getKey(jti)
	return s.redis.Del(ctx, key).Err()
}


func (s *TokenBlacklistService) BlacklistAllUserTokens(ctx context.Context, userID int64, ttl time.Duration) error {
	key := fmt.Sprintf("%suser:%d", s.prefix, userID)
	err := s.redis.Set(ctx, key, time.Now().Unix(), ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to blacklist user tokens: %w", err)
	}
	return nil
}

func (s *TokenBlacklistService) IsUserBlacklisted(ctx context.Context, userID int64) (bool, error) {
	key := fmt.Sprintf("%suser:%d", s.prefix, userID)
	exists, err := s.redis.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check user blacklist: %w", err)
	}
	return exists > 0, nil
}

func (s *TokenBlacklistService) GetBlacklistInfo(ctx context.Context, jti string) (string, error) {
	key := s.getKey(jti)
	value, err := s.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get blacklist info: %w", err)
	}
	return value, nil
}

func (s *TokenBlacklistService) getKey(jti string) string {
	return fmt.Sprintf("%s%s%s", s.prefix, blacklistKeyPrefix, jti)
}

func (s *TokenBlacklistService) CountBlacklistedTokens(ctx context.Context) (int64, error) {
	pattern := fmt.Sprintf("%s%s*", s.prefix, blacklistKeyPrefix)

	var cursor uint64
	var count int64

	for {
		keys, nextCursor, err := s.redis.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return 0, err
		}

		count += int64(len(keys))
		cursor = nextCursor

		if cursor == 0 {
			break
		}
	}

	return count, nil
}
