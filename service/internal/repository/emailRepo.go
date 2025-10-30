package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/Zifeldev/emailback/service/internal/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type EmailEntity struct {
	ID         string                 `db:"id" json:"id"`
	MessageID  string                 `db:"message_id" json:"message_id"`
	From       string                 `db:"from_addr" json:"from"`
	To         []string               `db:"to_addrs" json:"to"`
	Subject    string                 `db:"subject" json:"subject"`
	Date       *time.Time             `db:"date" json:"date"`
	Text       string                 `db:"body_text" json:"text"`
	HTML       string                 `db:"body_html,omitempty" json:"html,omitempty"`
	Language   string                 `db:"language,omitempty" json:"language,omitempty"`
	Confidence float64                `db:"language_confidence,omitempty" json:"language_confidence,omitempty"`
	Metrics    map[string]interface{} `db:"metrics" json:"metrics"`
	Headers    map[string]string      `db:"headers" json:"headers"`
	CreatedAt  time.Time              `db:"created_at" json:"created_at"`
	RawSize    int                    `db:"raw_size" json:"raw_size"`
}

type EmailRepository interface {
	SaveEmail(ctx context.Context, email *EmailEntity) error
	GetByID(ctx context.Context, id string) (*EmailEntity, error)
	GetAll(ctx context.Context, limit, offset int) ([]*EmailEntity, error)
}

// dbExecutor captures the subset of pool API we use, to enable testing/mocking.
type dbExecutor interface {
	Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}

type PostgresEmailRepo struct {
	pool dbExecutor
}

func NewPostgresEmailRepo(pool *db.TimeoutPool) *PostgresEmailRepo {
	return &PostgresEmailRepo{pool: pool}
}

var ErrEmailNotFound = errors.New("email not found")

const upsertEmail = `
INSERT INTO emails (
  id, message_id, from_addr, to_addrs, subject, date, body_text, body_html,
  language, language_confidence, metrics, headers, created_at, raw_size
) VALUES (
  $1,$2,$3,$4,$5,$6,$7,$8,
  $9,$10,$11,$12,$13,$14
)
ON CONFLICT (message_id) DO UPDATE SET
  from_addr = EXCLUDED.from_addr,
  to_addrs = EXCLUDED.to_addrs,
  subject = EXCLUDED.subject,
  date = EXCLUDED.date,
  body_text = EXCLUDED.body_text,
  body_html = EXCLUDED.body_html,
  language = EXCLUDED.language,
  language_confidence = EXCLUDED.language_confidence,
  metrics = EXCLUDED.metrics,
  headers = EXCLUDED.headers,
  raw_size = EXCLUDED.raw_size
`

const selectByID = `
SELECT id, message_id, from_addr, to_addrs, subject, date,
       body_text, body_html, language, language_confidence,
       metrics, headers, created_at, raw_size
FROM emails WHERE id = $1
`

const selectAll = `
SELECT id, message_id, from_addr, to_addrs, subject, date,
       body_text, body_html, language, language_confidence,
       metrics, headers, created_at, raw_size
FROM emails
ORDER BY created_at DESC
LIMIT $1 OFFSET $2
`

func (r *PostgresEmailRepo) SaveEmail(ctx context.Context, email *EmailEntity) error {
	metricsJSON, err := json.Marshal(email.Metrics)
	if err != nil {
		return err
	}
	headersJSON, err := json.Marshal(email.Headers)
	if err != nil {
		return err
	}
	createdAt := email.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	_, err = r.pool.Exec(ctx, upsertEmail,
		email.ID, email.MessageID, email.From, email.To, email.Subject, email.Date,
		email.Text, email.HTML, email.Language, email.Confidence,
		metricsJSON, headersJSON, createdAt, email.RawSize,
	)
	return err
}

func (r *PostgresEmailRepo) GetByID(ctx context.Context, id string) (*EmailEntity, error) {
	row := r.pool.QueryRow(ctx, selectByID, id)

	var email EmailEntity
	var metricsJSON, headersJSON []byte
	var dateNT sql.NullTime
	var confNF sql.NullFloat64

	err := row.Scan(
		&email.ID, &email.MessageID, &email.From, &email.To, &email.Subject, &dateNT,
		&email.Text, &email.HTML, &email.Language, &confNF,
		&metricsJSON, &headersJSON, &email.CreatedAt, &email.RawSize,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrEmailNotFound
		}
		return nil, err
	}

	if dateNT.Valid {
		email.Date = &dateNT.Time
	}
	if confNF.Valid {
		email.Confidence = confNF.Float64
	}
	if len(metricsJSON) > 0 {
		_ = json.Unmarshal(metricsJSON, &email.Metrics)
	}
	if len(headersJSON) > 0 {
		_ = json.Unmarshal(headersJSON, &email.Headers)
	}
	return &email, nil
}

func (r *PostgresEmailRepo) GetAll(ctx context.Context, limit, offset int) ([]*EmailEntity, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx, selectAll, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*EmailEntity, 0, limit)
	for rows.Next() {
		var e EmailEntity
		var metricsJSON, headersJSON []byte
		var dateNT sql.NullTime
		var confNF sql.NullFloat64

		if err := rows.Scan(
			&e.ID, &e.MessageID, &e.From, &e.To, &e.Subject, &dateNT,
			&e.Text, &e.HTML, &e.Language, &confNF,
			&metricsJSON, &headersJSON, &e.CreatedAt, &e.RawSize,
		); err != nil {
			return nil, err
		}
		if dateNT.Valid {
			e.Date = &dateNT.Time
		}
		if confNF.Valid {
			e.Confidence = confNF.Float64
		}
		if len(metricsJSON) > 0 {
			_ = json.Unmarshal(metricsJSON, &e.Metrics)
		}
		if len(headersJSON) > 0 {
			_ = json.Unmarshal(headersJSON, &e.Headers)
		}
		out = append(out, &e)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return out, nil
}
