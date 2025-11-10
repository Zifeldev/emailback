package middleware

import (
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func CORS() gin.HandlerFunc {
    originsCSV := os.Getenv("CORS_ORIGINS")
    var origins []string
    for _, o := range strings.Split(originsCSV, ",") {
        o = strings.TrimSpace(o)
        if o != "" {
            origins = append(origins, o)
        }
    }

    cfg := cors.Config{
        AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
        AllowHeaders:     []string{"Authorization", "Content-Type", "Accept", "X-Request-ID"},
        ExposeHeaders:    []string{"Content-Length"},
        AllowCredentials: true,
        MaxAge:           12 * time.Hour,
    }

    if len(origins) == 0 {
        cfg.AllowAllOrigins = true
    } else {
        cfg.AllowOrigins = origins
    }

    return cors.New(cfg)
}