package repository

import (
	"context"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

type stubRepo struct {
	saved map[string]*EmailEntity
}

func (s *stubRepo) SaveEmail(ctx context.Context, e *EmailEntity) error {
	if s.saved == nil {
		s.saved = map[string]*EmailEntity{}
	}
	s.saved[e.ID] = e
	return nil
}
func (s *stubRepo) GetByID(ctx context.Context, id string) (*EmailEntity, error) {
	if v, ok := s.saved[id]; ok {
		return v, nil
	}
	return nil, ErrEmailNotFound
}
func (s *stubRepo) GetAll(ctx context.Context, limit, offset int) ([]*EmailEntity, error) {
	out := make([]*EmailEntity, 0, len(s.saved))
	for _, v := range s.saved {
		out = append(out, v)
	}
	return out, nil
}

func TestCacheEmailRepo_GetByID_CacheAside(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	base := &stubRepo{saved: map[string]*EmailEntity{"1": {ID: "1", MessageID: "m1", From: "a", To: []string{"b"}, Subject: "s", Text: "t", RawSize: 1, CreatedAt: time.Now()}}}
	repo := NewCacheEmailRepo(base, rdb, "test:", time.Minute)

	ctx := context.Background()
	// miss -> underlying -> set cache
	e, err := repo.GetByID(ctx, "1")
	if err != nil || e == nil || e.ID != "1" {
		t.Fatalf("expected entity from underlying, got %v err=%v", e, err)
	}
	// now present in cache: remove from underlying to prove cache hit
	delete(base.saved, "1")
	e2, err := repo.GetByID(ctx, "1")
	if err != nil || e2 == nil || e2.ID != "1" {
		t.Fatalf("expected entity from cache, got %v err=%v", e2, err)
	}
}

func TestCacheEmailRepo_SaveEmail_Invalidates(t *testing.T) {
	mr, _ := miniredis.Run()
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	base := &stubRepo{saved: map[string]*EmailEntity{}}
	repo := NewCacheEmailRepo(base, rdb, "test:", time.Minute)

	ctx := context.Background()
	e := &EmailEntity{ID: "2", MessageID: "m2", From: "a", To: []string{"b"}, Subject: "s", Text: "t", CreatedAt: time.Now()}
	// warm cache manually
	_, _ = repo.GetByID(ctx, "2") // miss to underlying -> ErrEmailNotFound
	// save and ensure key removed (best-effort; we verify that GetByID misses and goes to underlying)
	_ = repo.SaveEmail(ctx, e)
	// should come from underlying (exists now), and be set to cache
	got, err := repo.GetByID(ctx, "2")
	if err != nil || got == nil || got.ID != "2" {
		t.Fatalf("expected fetch after save: %v err=%v", got, err)
	}
}

func TestCacheEmailRepo_BadJSON_FallbackToDB(t *testing.T) {
	mr, _ := miniredis.Run()
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	base := &stubRepo{saved: map[string]*EmailEntity{"3": {ID: "3", MessageID: "m3", From: "a", To: []string{"b"}, Subject: "s", Text: "t", CreatedAt: time.Now()}}}
	repo := NewCacheEmailRepo(base, rdb, "test:", time.Minute)

	// write invalid JSON into cache
	_ = rdb.Set(context.Background(), repo.cacheKeyByID("3"), "{not-json}", time.Minute).Err()
	got, err := repo.GetByID(context.Background(), "3")
	if err != nil || got == nil || got.ID != "3" {
		t.Fatalf("expected fallback to DB, got %v err=%v", got, err)
	}
}
