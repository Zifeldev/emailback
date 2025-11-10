package controllers

import (
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

type HealthController struct {
	pool      *pgxpool.Pool
	redis     *redis.Client
	log       *logrus.Entry
	startTime time.Time
	version   string
}

func NewHealthController(pool *pgxpool.Pool, redis *redis.Client, log *logrus.Entry, startTime time.Time, version string) *HealthController {
	return &HealthController{
		pool:      pool,
		redis:     redis,
		log:       log,
		startTime: startTime,
		version:   version,
	}
}

// @Summary Health check
// @Tags health
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /health [get]
func (h *HealthController) Health(c *gin.Context) {
	ctx := c.Request.Context()

	// Check PostgreSQL
	pgLatency := time.Duration(0)
	pgStatus := "ok"
	start := time.Now()
	if err := h.pool.Ping(ctx); err != nil {
		pgStatus = "error"
		h.log.WithError(err).Error("postgres health check failed")
	} else {
		pgLatency = time.Since(start)
	}

	// Check Redis
	redisStatus := "ok"
	if h.redis != nil {
		if err := h.redis.Ping(ctx).Err(); err != nil {
			redisStatus = "error"
			h.log.WithError(err).Error("redis health check failed")
		}
	} else {
		redisStatus = "disabled"
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	c.JSON(http.StatusOK, gin.H{
		"status":        "ok",
		"timestamp":     time.Now().UTC(),
		"service_name":  "auth-service",
		"version":       h.version,
		"uptime":        time.Since(h.startTime).String(),
		"go_version":    runtime.Version(),
		"num_goroutine": runtime.NumGoroutine(),
		"checks": gin.H{
			"postgres": gin.H{
				"status":     pgStatus,
				"latency_ms": pgLatency.Milliseconds(),
			},
			"redis": gin.H{
				"status": redisStatus,
			},
		},
		"memory": gin.H{
			"alloc_mb":       float64(m.Alloc) / 1024 / 1024,
			"total_alloc_mb": float64(m.TotalAlloc) / 1024 / 1024,
			"sys_mb":         float64(m.Sys) / 1024 / 1024,
			"num_gc":         m.NumGC,
		},
	})
}
