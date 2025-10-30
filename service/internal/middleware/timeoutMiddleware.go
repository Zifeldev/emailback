package middleware

import (
    "context"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
)


func TimeoutMiddleware(d time.Duration) gin.HandlerFunc {
    return func(c *gin.Context) {
        if d <= 0 {
            c.Next()
            return
        }
        ctx, cancel := context.WithTimeout(c.Request.Context(), d)
        defer cancel()

        done := make(chan struct{})
        panicChan := make(chan any, 1)

        c.Request = c.Request.WithContext(ctx)

        go func() {
            defer func() {
                if p := recover(); p != nil {
                    panicChan <- p
                }
            }()
            c.Next()
            close(done)
        }()

        select {
        case <-done:
            return
        case p := <-panicChan:
            panic(p)
        case <-ctx.Done():
            c.AbortWithStatusJSON(http.StatusGatewayTimeout, gin.H{"error": "request timeout"})
            return
        }
    }
}