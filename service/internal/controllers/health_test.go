package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

type mockDB struct {
	fail bool
}

func (m *mockDB) Ping(ctx context.Context) error {
	if m.fail {
		return context.DeadlineExceeded
	}
	return nil
}

func TestHealthHandler_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dbMock := &mockDB{fail: false}
	ctrl := NewHealthController(
		dbMock,
		nil,
		logrus.NewEntry(logrus.New()),
		time.Now().Add(-1*time.Hour),
		"v1.0.0",
	)

	router := gin.Default()
	router.GET("/health", ctrl.Handle)

	req, _ := http.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp HealthResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "ok", resp.Status)
	assert.Contains(t, resp.Checks, "postgres")
}

func TestHealthHandler_Degraded(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dbMock := &mockDB{fail: true}

	ctrl := NewHealthController(
		dbMock,
		nil,
		logrus.NewEntry(logrus.New()),
		time.Now().Add(-30*time.Minute),
		"v1.0.0",
	)

	router := gin.Default()
	router.GET("/health", ctrl.Handle)

	req, _ := http.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var resp HealthResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "degraded", resp.Status)
}
