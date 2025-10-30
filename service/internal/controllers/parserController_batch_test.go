package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Zifeldev/emailback/service/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type batchRepo struct{ memRepo }

type okParser struct{}

func (okParser) Parse(_ context.Context, _ []byte) (*repository.EmailEntity, error) {
	now := time.Now().UTC()
	return &repository.EmailEntity{ID: "id-x", MessageID: "m-x", CreatedAt: now}, nil
}

type errParser struct{}

func (errParser) Parse(_ context.Context, _ []byte) (*repository.EmailEntity, error) {
	return nil, errors.New("boom")
}

func TestBatchParseAndSave_SuccessAndError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	pc := NewParserController(okParser{}, newMemRepo(), logrus.New().WithField("t", "test"))
	r := gin.New()
	r.POST("/parse/batch", pc.BatchParseAndSave)

	payload := []BatchEmailInput{{Raw: "From: a\nTo: b\n\nhello"}, {Raw: "From: c\nTo: d\n\nhi"}}
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/parse/batch?max_workers=2&item_timeout=1s", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestBatchParseAndSave_ParseErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	pc := NewParserController(errParser{}, newMemRepo(), logrus.New().WithField("t", "test"))
	r := gin.New()
	r.POST("/parse/batch", pc.BatchParseAndSave)

	payload := []BatchEmailInput{{Raw: "x"}}
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/parse/batch", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with error items, got %d", w.Code)
	}
}
