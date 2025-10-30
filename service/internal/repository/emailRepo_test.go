package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type mockPool struct {
	execSQL  string
	execArgs []interface{}
	execErr  error
	row      pgx.Row
}

func (m *mockPool) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	m.execSQL = sql
	m.execArgs = args
	if m.execErr != nil {
		return pgconn.NewCommandTag(""), m.execErr
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}
func (m *mockPool) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	panic("not used in tests")
}
func (m *mockPool) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return m.row
}

type mockRow struct{ scan func(dest ...any) error }

func (r mockRow) Scan(dest ...any) error { return r.scan(dest...) }

func TestPostgresEmailRepo_SaveEmail_Args(t *testing.T) {
	mp := &mockPool{}
	repo := &PostgresEmailRepo{pool: mp}
	now := time.Now().UTC()
	e := &EmailEntity{
		ID: "id1", MessageID: "m1", From: "a", To: []string{"b", "c"}, Subject: "sub",
		Date: &now, Text: "text", HTML: "<p>h</p>", Language: "en", Confidence: 0.9,
		Metrics: map[string]interface{}{"w": 1}, Headers: map[string]string{"X": "y"},
		CreatedAt: now, RawSize: 42,
	}
	if err := repo.SaveEmail(context.Background(), e); err != nil {
		t.Fatalf("save: %v", err)
	}
	if len(mp.execArgs) != 14 {
		t.Fatalf("expected 14 args, got %d", len(mp.execArgs))
	}
	if mp.execArgs[0] != "id1" || mp.execArgs[1] != "m1" || mp.execArgs[2] != "a" {
		t.Fatalf("unexpected args prefix: %v", mp.execArgs[:3])
	}
}

func TestPostgresEmailRepo_GetByID_Success(t *testing.T) {
	now := time.Now().UTC()
	metrics := map[string]interface{}{"raw_size": 10}
	headers := map[string]string{"h": "v"}
	metricsJSON, _ := json.Marshal(metrics)
	headersJSON, _ := json.Marshal(headers)

	row := mockRow{scan: func(dest ...any) error {
		// order matches selectByID
		*(dest[0].(*string)) = "id2"
		*(dest[1].(*string)) = "m2"
		*(dest[2].(*string)) = "from@a"
		*(dest[3].(*[]string)) = []string{"to@b"}
		*(dest[4].(*string)) = "subject"
		dt := dest[5].(*sql.NullTime)
		*dt = sql.NullTime{Time: now, Valid: true}
		*(dest[6].(*string)) = "text"
		*(dest[7].(*string)) = "html"
		*(dest[8].(*string)) = "ru"
		cf := dest[9].(*sql.NullFloat64)
		*cf = sql.NullFloat64{Float64: 0.7, Valid: true}
		*(dest[10].(*[]byte)) = metricsJSON
		*(dest[11].(*[]byte)) = headersJSON
		*(dest[12].(*time.Time)) = now
		*(dest[13].(*int)) = 55
		return nil
	}}

	mp := &mockPool{row: row}
	repo := &PostgresEmailRepo{pool: mp}
	got, err := repo.GetByID(context.Background(), "id2")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != "id2" || got.MessageID != "m2" || got.Confidence != 0.7 || got.Language != "ru" {
		t.Fatalf("bad entity: %+v", got)
	}
	if got.Date == nil || !got.Date.Equal(now) {
		t.Fatalf("bad date: %v", got.Date)
	}
	if got.RawSize != 55 {
		t.Fatalf("rawsize: %d", got.RawSize)
	}
	if got.Metrics["raw_size"].(float64) != 10 {
		t.Fatalf("metrics: %v", got.Metrics)
	}
	if !reflect.DeepEqual(got.Headers["h"], "v") {
		t.Fatalf("headers: %v", got.Headers)
	}
}

func TestPostgresEmailRepo_GetByID_NotFound(t *testing.T) {
	row := mockRow{scan: func(dest ...any) error { return pgx.ErrNoRows }}
	mp := &mockPool{row: row}
	repo := &PostgresEmailRepo{pool: mp}
	got, err := repo.GetByID(context.Background(), "nope")
	if err == nil || err != ErrEmailNotFound || got != nil {
		t.Fatalf("expected not found, got %v %v", got, err)
	}
}
