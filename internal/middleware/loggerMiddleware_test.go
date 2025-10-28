package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func newTestLogger() *logrus.Logger {
	l := logrus.New()
	l.SetLevel(logrus.DebugLevel)
	l.SetFormatter(&logrus.TextFormatter{DisableTimestamp: true})
	return l
}

func TestTraceMiddleware_SetsHeaderAndContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	lg := newTestLogger()
	r.Use(TraceMiddleware(lg))
	r.GET("/ping", func(c *gin.Context) {
		if getTraceID(c.Request.Context()) == "" {
			t.Fatalf("trace id not found in request context")
		}
		c.String(200, "pong")
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ping", nil)
	r.ServeHTTP(w, req)
	if w.Header().Get(HeaderTraceID) == "" {
		t.Fatalf("trace id header not present in response")
	}
}

func TestLoggerMiddleware_Logs200(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	lg := newTestLogger()
	r.Use(LoggerMiddleware(lg))
	r.GET("/ok", func(c *gin.Context) { c.String(200, "ok") })
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ok", nil)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("unexpected status: %d", w.Code)
	}
}

func TestRecoveryMiddleware_Recovers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	lg := newTestLogger()
	r.Use(RecoveryMiddleware(lg))
	r.GET("/panic", func(c *gin.Context) { panic(errors.New("boom")) })
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/panic", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestLoggerMiddleware_Logs400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	lg := newTestLogger()
	r.Use(LoggerMiddleware(lg))
	r.GET("/bad", func(c *gin.Context) { c.AbortWithStatus(400) })
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/bad", nil)
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestLoggerMiddleware_Logs500(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	lg := newTestLogger()
	r.Use(LoggerMiddleware(lg))
	r.GET("/boom", func(c *gin.Context) { c.AbortWithStatus(500) })
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/boom", nil)
	r.ServeHTTP(w, req)
	if w.Code != 500 {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestRecoveryMiddleware_BrokenPipe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	lg := newTestLogger()
	r.Use(RecoveryMiddleware(lg))
	r.GET("/bp", func(c *gin.Context) { panic(errors.New("write: broken pipe")) })
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/bp", nil)
	r.ServeHTTP(w, req)
	if w.Code == 0 {
		t.Fatalf("no status code written")
	}
}
