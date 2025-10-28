package controllers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DBHealth struct{ pool *pgxpool.Pool }

func NewDBHealth(pool *pgxpool.Pool) *DBHealth { return &DBHealth{pool: pool} }

// Handle
// @Summary     Database health check
// @Tags        health
// @Produce     json
// @Success     200  {object}  map[string]string
// @Failure     503  {object}  map[string]string
// @Router      /healthz [get]
func (h *DBHealth) Handle(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 1*time.Second)
	defer cancel()
	if err := h.pool.Ping(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", "db": "down"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "db": "up"})
}

// HealthHandler
// @Summary     Liveness probe
// @Tags        health
// @Produce     json
// @Success     200  {object}  map[string]string
// @Router      /health [get]
func HealthHandler(c *gin.Context) {
	c.JSON(200, gin.H{
		"status": "ok",
	})
}
