package middleware

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type responseWriter struct {
	gin.ResponseWriter
	status int
	size   int
}

func (w *responseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *responseWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.size += n
	return n, err
}


const (
	HeaderTraceID       = "X-Trace-ID"
	HeaderRequestID     = "X-Request-ID"
	HeaderCorrelationID = "X-Correlation-ID"

	ginKeyTraceID = "trace_id"
)


type contextKey string

const ctxKeyTraceID contextKey = "trace_id"

func getTraceID(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyTraceID).(string); ok {
		return v
	}
	return ""
}


 

func LoggerMiddleware(log *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		rw := &responseWriter{ResponseWriter: c.Writer, status: http.StatusOK}
		c.Writer = rw

		c.Next()
		status := c.Writer.Status()
		duration := time.Since(start)

		entry := log.WithFields(logrus.Fields{
			"method":        c.Request.Method,
			"path":          c.Request.URL.Path,
			"status":        status,
			"size":          rw.size,
			"duration_ms":   duration.Milliseconds(),
			"client_ip":     c.ClientIP(),
			"user_agent":    c.Request.UserAgent(),
			"trace_id":      getTraceID(c.Request.Context()),
			"error_message": c.Errors.ByType(gin.ErrorTypePrivate).String(),
		})

		if rw.status >= 500 {
			entry.Error("server_error")
		} else if rw.status >= 400 {
			entry.Warn("client_error")
		} else {
			entry.Info("request_processed")
		}
	}
}

func TraceMiddleware(_ *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := c.GetHeader(HeaderTraceID)
		if traceID == "" {
			traceID = c.GetHeader(HeaderRequestID)
		}
		if traceID == "" {
			traceID = c.GetHeader(HeaderCorrelationID)
		}
		if traceID == "" {
			traceID = uuid.New().String()
		}

		ctx := context.WithValue(c.Request.Context(), ctxKeyTraceID, traceID)
		c.Request = c.Request.WithContext(ctx)

		c.Header(HeaderTraceID, traceID)
		c.Set(ginKeyTraceID, traceID)

		c.Next()
	}
}

func RecoveryMiddleware(log *logrus.Logger) gin.HandlerFunc {
    return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
        traceID := getTraceID(c.Request.Context())
        panicMsg := fmt.Sprint(recovered)
        stack := string(debug.Stack())

        lower := strings.ToLower(panicMsg)
        isBroken := strings.Contains(lower, "broken pipe") || strings.Contains(lower, "connection reset by peer")

        entry := log.WithFields(logrus.Fields{
            "method":     c.Request.Method,
            "path":       c.Request.URL.Path,
            "client_ip":  c.ClientIP(),
            "user_agent": c.Request.UserAgent(),
            "trace_id":   traceID,
            "panic":      panicMsg,
        })
        if !isBroken {
            entry = entry.WithField("stack", stack)
        }

        if isBroken {
            entry.Warn("client_broken_pipe")
            c.Abort() 
            return
        }

        c.Header(HeaderTraceID, traceID)
        entry.Error("panic_recovered")
        c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
            "error":    "internal_server_error",
            "trace_id": traceID,
        })
    })
}