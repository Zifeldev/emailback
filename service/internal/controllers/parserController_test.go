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

type mockParser struct {
	ent *repository.EmailEntity
	err error
}

func (m mockParser) Parse(ctx context.Context, rawEmail []byte) (*repository.EmailEntity, error) {
	return m.ent, m.err
}

type memRepo struct {
	byID map[string]*repository.EmailEntity
}

func newMemRepo() *memRepo { return &memRepo{byID: map[string]*repository.EmailEntity{}} }

func (m *memRepo) SaveEmail(ctx context.Context, email *repository.EmailEntity) error {
	m.byID[email.ID] = email
	return nil
}
func (m *memRepo) GetByID(ctx context.Context, id string) (*repository.EmailEntity, error) {
	if v, ok := m.byID[id]; ok {
		return v, nil
	}
	return nil, repository.ErrEmailNotFound
}
func (m *memRepo) GetAll(ctx context.Context, limit, offset int) ([]*repository.EmailEntity, error) {
	out := make([]*repository.EmailEntity, 0, len(m.byID))
	for _, v := range m.byID {
		out = append(out, v)
	}
	if offset > len(out) {
		return []*repository.EmailEntity{}, nil
	}
	end := offset + limit
	if end > len(out) {
		end = len(out)
	}
	return out[offset:end], nil
}

func setupRouter(pc *ParserController) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/parse", pc.ParseAndSave)
	r.GET("/emails/:id", pc.GetByID)
	r.GET("/emails", pc.GetAll)
	return r
}

func TestParserController_ParseAndSave_OK(t *testing.T) {
	now := time.Now().UTC()
	ent := &repository.EmailEntity{ID: "id-1", MessageID: "m1", From: "a@a", To: []string{"b@b"}, Subject: "s", Date: &now, Text: "hi", CreatedAt: now, RawSize: 10}
	pc := NewParserController(mockParser{ent: ent}, newMemRepo(), logrus.New().WithField("t", "test"))
	r := setupRouter(pc)

	body := bytes.NewBufferString("raw eml")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/parse", body)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status code = %d", w.Code)
	}
	var got repository.EmailEntity
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if got.ID != ent.ID || got.MessageID != ent.MessageID {
		t.Errorf("unexpected entity in response")
	}
}

func TestParserController_GetByID_NotFound(t *testing.T) {
	pc := NewParserController(mockParser{}, newMemRepo(), logrus.New().WithField("t", "test"))
	r := setupRouter(pc)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/emails/unknown", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestParserController_GetAll_OK(t *testing.T) {
	repo := newMemRepo()
	now := time.Now().UTC()
	repo.SaveEmail(context.Background(), &repository.EmailEntity{ID: "id-1", MessageID: "m1", CreatedAt: now})
	repo.SaveEmail(context.Background(), &repository.EmailEntity{ID: "id-2", MessageID: "m2", CreatedAt: now})

	pc := NewParserController(mockParser{}, repo, logrus.New().WithField("t", "test"))
	r := setupRouter(pc)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/emails?limit=10&offset=0", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		Count int                      `json:"count"`
		Items []repository.EmailEntity `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if resp.Count != 2 {
		t.Fatalf("expected count=2, got %d", resp.Count)
	}
}

func TestParserController_ParseAndSave_BadBody(t *testing.T) {
	pc := NewParserController(mockParser{ent: &repository.EmailEntity{ID: "x", MessageID: "m", CreatedAt: time.Now()}}, newMemRepo(), logrus.New().WithField("t", "test"))
	r := setupRouter(pc)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/parse", bytes.NewBuffer(nil))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
}

func TestParserController_ParseAndSave_ParseError(t *testing.T) {
	pc := NewParserController(mockParser{err: errors.New("boom")}, newMemRepo(), logrus.New().WithField("t", "test"))
	r := setupRouter(pc)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/parse", bytes.NewBufferString("x"))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

type saveErrRepo struct{ memRepo }

func (r *saveErrRepo) SaveEmail(ctx context.Context, email *repository.EmailEntity) error {
	return errors.New("save_failed")
}

func TestParserController_ParseAndSave_SaveError(t *testing.T) {
	now := time.Now().UTC()
	ent := &repository.EmailEntity{ID: "id-1", MessageID: "m1", From: "a@a", To: []string{"b@b"}, Subject: "s", Date: &now, Text: "hi", CreatedAt: now, RawSize: 10}
	pc := NewParserController(mockParser{ent: ent}, &saveErrRepo{*newMemRepo()}, logrus.New().WithField("t", "test"))
	r := setupRouter(pc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/parse", bytes.NewBufferString("raw"))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

type getErrRepo struct{ memRepo }

func (r *getErrRepo) GetByID(ctx context.Context, id string) (*repository.EmailEntity, error) {
	return nil, errors.New("db")
}

func TestParserController_GetByID_DBError(t *testing.T) {
	pc := NewParserController(mockParser{}, &getErrRepo{*newMemRepo()}, logrus.New().WithField("t", "test"))
	r := setupRouter(pc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/emails/some", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}
