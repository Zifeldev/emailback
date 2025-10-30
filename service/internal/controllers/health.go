package controllers

import (
	"context"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/Zifeldev/emailback/service/internal/db"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"github.com/gin-gonic/gin"
)

type HealthController struct {
	DBPool     db.Pinger
	Redis      *redis.Client
	Logger     *logrus.Entry
	StartTime  time.Time
	ServiceVer string
}


func NewHealthController(pool db.Pinger, redis *redis.Client, logger *logrus.Entry, start time.Time, version string) *HealthController {
	return &HealthController{
		DBPool:     pool,
		Redis:      redis,
		Logger:     logger,
		StartTime:  start,
		ServiceVer: version,
	}
}


type HealthResponse struct {
	Status       string                 `json:"status" example:"ok"`
	Timestamp    string                 `json:"timestamp" example:"2025-10-30T10:15:00Z"`
	ServiceName  string                 `json:"service_name" example:"emailback"`
	Version      string                 `json:"version" example:"v1.0.0"`
	Hostname     string                 `json:"hostname" example:"emailback-app-1"`
	Uptime       string                 `json:"uptime" example:"5m42s"`
	GoVersion    string                 `json:"go_version" example:"go1.23.2"`
	NumGoroutine int                    `json:"num_goroutine" example:"18"`
	Checks       map[string]interface{} `json:"checks"`
	Memory       map[string]interface{} `json:"memory"`
}

// Handle runs all health checks and returns the system status.
//
// @Summary      Service health check
// @Description  Returns detailed information about the EmailBack API service state, including database, Redis, memory usage, and uptime.
// @Tags         health
// @Produce      json
// @Success      200 {object} HealthResponse "Service is healthy"
// @Failure      503 {object} HealthResponse "Service is degraded"
// @Router       /health [get]
func (h *HealthController) Handle(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	status := HealthResponse{
		Status:       "ok",
		Timestamp:    time.Now().UTC().Format(time.RFC3339Nano),
		ServiceName:  "emailback",
		Version:      h.ServiceVer,
		Hostname:     getHostname(),
		Uptime:       time.Since(h.StartTime).String(),
		GoVersion:    runtime.Version(),
		NumGoroutine: runtime.NumGoroutine(),
		Checks:       make(map[string]interface{}),
	}

	// --- PostgreSQL Check ---
	dbCheck := make(map[string]interface{})
	start := time.Now()
	if err := h.DBPool.Ping(ctx); err != nil {
		dbCheck["status"] = "fail"
		dbCheck["error"] = err.Error()
		status.Status = "degraded"
	} else {
		dbCheck["status"] = "ok"
	}
	dbCheck["latency_ms"] = time.Since(start).Milliseconds()
	status.Checks["postgres"] = dbCheck

	// --- Redis Check ---
	if h.Redis != nil {
		redisCheck := make(map[string]interface{})
		start := time.Now()
		if err := h.Redis.Ping(ctx).Err(); err != nil {
			redisCheck["status"] = "fail"
			redisCheck["error"] = err.Error()
			status.Status = "degraded"
		} else {
			redisCheck["status"] = "ok"
		}
		redisCheck["latency_ms"] = time.Since(start).Milliseconds()
		status.Checks["redis"] = redisCheck
	}

	// --- Memory Info ---
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	status.Memory = map[string]interface{}{
		"alloc_mb":       float64(memStats.Alloc) / 1024 / 1024,
		"total_alloc_mb": float64(memStats.TotalAlloc) / 1024 / 1024,
		"sys_mb":         float64(memStats.Sys) / 1024 / 1024,
		"num_gc":         memStats.NumGC,
	}

	code := http.StatusOK
	if status.Status == "degraded" {
		code = http.StatusServiceUnavailable
	}
	c.JSON(code, status)
}

func getHostname() string {
	name, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return name
}
