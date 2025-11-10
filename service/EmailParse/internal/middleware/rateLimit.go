package middleware

import (
	"net/http"
	"strconv"
	"time"

	ratelimit "github.com/JGLTechnologies/gin-rate-limit"
	"github.com/Zifeldev/emailback/service/EmailParse/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

func RateLimitMiddleware(cfg config.RateLimitConfig, log *logrus.Logger, rdb *redis.Client) gin.HandlerFunc {
	if !cfg.Enabled || cfg.Interval <= 0 || cfg.Max <= 0 {
		return func(c *gin.Context) { c.Next() }
	}

	var store ratelimit.Store
	if rdb != nil {
		store = ratelimit.RedisStore(&ratelimit.RedisOptions{
			RedisClient: rdb,
			Rate:        cfg.Interval,
			Limit:       uint(cfg.Max),
		})
		log.WithFields(logrus.Fields{
			"backend": "redis",
			"rate":    cfg.Interval.String(),
			"limit":   cfg.Max,
		}).Info("rate limit enabled")
	} else {
		store = ratelimit.InMemoryStore(&ratelimit.InMemoryOptions{
			Rate:  cfg.Interval,
			Limit: uint(cfg.Max),
		})
		log.WithFields(logrus.Fields{
			"backend": "memory",
			"rate":    cfg.Interval.String(),
			"limit":   cfg.Max,
		}).Warn("redis not configured; using in-memory rate limit")
	}

	return ratelimit.RateLimiter(store, &ratelimit.Options{
		KeyFunc: func(c *gin.Context) string {
			ip := c.ClientIP()
			if ip == "" {
				ip = "unknown"
			}
			return ip
		},
		ErrorHandler: func(c *gin.Context, info ratelimit.Info) {
			retry := time.Until(info.ResetTime).Round(time.Second)
			if retry < 0 {
				retry = 0
			}
			c.Header("Retry-After", strconv.FormatInt(int64(retry/time.Second), 10))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":      "rate limit exceeded",
				"limit":      info.Limit,
				"reset_unix": info.ResetTime.Unix(),
			})
		},
	})
}
