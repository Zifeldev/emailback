package repository

import (
	"context"
	stdsql "database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// fakeRows implements pgx.Rows minimally for our tests.
type fakeRows struct {
	scans []func(dest ...any) error
	idx   int
	err   error
}

func (r *fakeRows) Next() bool {
	if r.idx < len(r.scans) {
		r.idx++
		return true
	}
	return false
}
func (r *fakeRows) Scan(dest ...any) error {
	if r.idx == 0 || r.idx > len(r.scans) {
		return stdsql.ErrNoRows
	}
	return r.scans[r.idx-1](dest...)
}
func (r *fakeRows) Values() ([]any, error)                       { return nil, nil }
func (r *fakeRows) RawValues() [][]byte                          { return nil }
func (r *fakeRows) Err() error                                   { return r.err }
func (r *fakeRows) Close()                                       {}
func (r *fakeRows) CommandTag() pgconn.CommandTag                { return pgconn.NewCommandTag("") }
func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeRows) Conn() *pgx.Conn                              { return nil }

type mockPoolQuery struct {
	rows pgx.Rows
	qErr error
}

func (m *mockPoolQuery) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag(""), nil
}
func (m *mockPoolQuery) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return m.rows, m.qErr
}
func (m *mockPoolQuery) QueryRow(ctx context.Context, _ string, args ...interface{}) pgx.Row {
	return mockRow{scan: func(dest ...any) error { return stdsql.ErrNoRows }}
}

func TestPostgresEmailRepo_GetAll_Success(t *testing.T) {
	now := time.Now().UTC()
	m1 := map[string]interface{}{"a": 1}
	h1 := map[string]string{"H": "v"}
	m1j, _ := json.Marshal(m1)
	h1j, _ := json.Marshal(h1)

	row1 := func(dest ...any) error {
		*(dest[0].(*string)) = "id1"
		*(dest[1].(*string)) = "m1"
		*(dest[2].(*string)) = "from1"
		*(dest[3].(*[]string)) = []string{"to1"}
		*(dest[4].(*string)) = "sub1"
		dt := dest[5].(*stdsql.NullTime)
		*dt = stdsql.NullTime{Time: now, Valid: true}
		*(dest[6].(*string)) = "text1"
		*(dest[7].(*string)) = "html1"
		*(dest[8].(*string)) = "en"
		cf := dest[9].(*stdsql.NullFloat64)
		*cf = stdsql.NullFloat64{Float64: 0.5, Valid: true}
		*(dest[10].(*[]byte)) = m1j
		*(dest[11].(*[]byte)) = h1j
		*(dest[12].(*time.Time)) = now
		*(dest[13].(*int)) = 10
		return nil
	}
	row2 := func(dest ...any) error {
		*(dest[0].(*string)) = "id2"
		*(dest[1].(*string)) = "m2"
		*(dest[2].(*string)) = "from2"
		*(dest[3].(*[]string)) = []string{"to2a", "to2b"}
		*(dest[4].(*string)) = "sub2"
		dt := dest[5].(*stdsql.NullTime)
		*dt = stdsql.NullTime{Valid: false}
		*(dest[6].(*string)) = "text2"
		*(dest[7].(*string)) = ""
		*(dest[8].(*string)) = "ru"
		cf := dest[9].(*stdsql.NullFloat64)
		*cf = stdsql.NullFloat64{Valid: false}
		*(dest[10].(*[]byte)) = nil
		*(dest[11].(*[]byte)) = nil
		*(dest[12].(*time.Time)) = now
		*(dest[13].(*int)) = 20
		return nil
	}

	rows := &fakeRows{scans: []func(dest ...any) error{row1, row2}}
	mp := &mockPoolQuery{rows: rows}
	repo := &PostgresEmailRepo{pool: mp}

	got, err := repo.GetAll(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("getall: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
	if got[0].ID != "id1" || got[1].ID != "id2" {
		t.Fatalf("ids: %v, %v", got[0].ID, got[1].ID)
	}
	if got[0].Metrics["a"].(float64) != 1 {
		t.Fatalf("metrics decode failed: %v", got[0].Metrics)
	}
	if got[0].Date == nil || got[1].Date != nil {
		t.Fatalf("date handling wrong")
	}
}

func TestPostgresEmailRepo_GetAll_ScanError(t *testing.T) {
	rows := &fakeRows{scans: []func(dest ...any) error{func(dest ...any) error { return stdsql.ErrNoRows }}}
	mp := &mockPoolQuery{rows: rows}
	repo := &PostgresEmailRepo{pool: mp}
	_, err := repo.GetAll(context.Background(), 10, 0)
	if err == nil {
		t.Fatalf("expected error from scan")
	}
}
