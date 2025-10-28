package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type EmailEntity struct {
	ID         string                 `db:"id" json:"id"`
	MessageID  string                 `db:"message_id" json:"message_id"`
	From       string                 `db:"from" json:"from"`
	To         []string               `db:"to" json:"to"`
	Subject    string                 `db:"subject" json:"subject"`
	Date       *time.Time             `db:"date" json:"date"`
	Text       string                 `db:"text" json:"text"`
	HTML       string                 `db:"html,omitempty" json:"html,omitempty"`
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

type PostgresEmailRepo struct {
	pool *pgxpool.Pool
}

func NewPostgresEmailRepo(pool *pgxpool.Pool) *PostgresEmailRepo {
	return &PostgresEmailRepo{pool: pool}
}

var ErrEmailNotFound = errors.New("email not found")

func (r *PostgresEmailRepo) SaveEmail(ctx context.Context, email *EmailEntity) error {
	metricsJSON, err := json.Marshal(email.Metrics)
	if err != nil {
		return err
	}
	headersJSON, err := json.Marshal(email.Headers)
	if err != nil {
		return err
	}

	_, err = r.pool.Exec(ctx, `
INSERT INTO emails (
  id, message_id, "from", "to", subject, date, text, html,
  language, language_confidence, metrics, headers, created_at, raw_size
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8,
  $9, $10, $11, $12, $13, $14
)
ON CONFLICT (message_id) DO UPDATE SET
  "from" = EXCLUDED."from",
  "to" = EXCLUDED."to",
  subject = EXCLUDED.subject,
  date = EXCLUDED.date,
  text = EXCLUDED.text,
  html = EXCLUDED.html,
  language = EXCLUDED.language,
  language_confidence = EXCLUDED.language_confidence,
  metrics = EXCLUDED.metrics,
  headers = EXCLUDED.headers,
  created_at = EXCLUDED.created_at,
  raw_size = EXCLUDED.raw_size
`, email.ID, email.MessageID, email.From, email.To, email.Subject, email.Date, email.Text, email.HTML,
		email.Language, email.Confidence, metricsJSON, headersJSON, email.CreatedAt, email.RawSize)
	return err
}

func (r *PostgresEmailRepo) GetByID(ctx context.Context, id string) (*EmailEntity, error) {
	row := r.pool.QueryRow(ctx, `
SELECT id, message_id, "from", "to", subject, date, text, html,
       language, language_confidence, metrics, headers, created_at, raw_size
FROM emails
WHERE id = $1
`, id)

	var email EmailEntity
	var metricsJSON []byte
	var headersJSON []byte
	var dateNull time.Time
	var datePtr *time.Time
	var confNull *float64

	err := row.Scan(
		&email.ID, &email.MessageID, &email.From, &email.To, &email.Subject, &dateNull, &email.Text, &email.HTML,
		&email.Language, &confNull, &metricsJSON, &headersJSON, &email.CreatedAt, &email.RawSize,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrEmailNotFound
		}
		return nil, err
	}

	if !dateNull.IsZero() {
		datePtr = &dateNull
	}
	email.Date = datePtr
	if confNull != nil {
		email.Confidence = *confNull
	} else {
		email.Confidence = 0
	}
	if len(metricsJSON) > 0 {
		var m map[string]interface{}
		if err := json.Unmarshal(metricsJSON, &m); err == nil {
			email.Metrics = m
		}
	}
	if len(headersJSON) > 0 {
		var h map[string]string
		if err := json.Unmarshal(headersJSON, &h); err == nil {
			email.Headers = h
		}
	}
	return &email, nil
}

func (r *PostgresEmailRepo) GetAll(ctx context.Context, limit, offset int) ([]*EmailEntity, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx, `
SELECT id, message_id, "from", "to", subject, date, text, html,
       language, language_confidence, metrics, headers, created_at, raw_size
FROM emails
ORDER BY created_at DESC
LIMIT $1 OFFSET $2
`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*EmailEntity, 0)
	for rows.Next() {
		var e EmailEntity
		var metricsJSON []byte
		var headersJSON []byte
		var dateNull time.Time
		var datePtr *time.Time
		var confNull *float64

		if err := rows.Scan(
			&e.ID, &e.MessageID, &e.From, &e.To, &e.Subject, &dateNull, &e.Text, &e.HTML,
			&e.Language, &confNull, &metricsJSON, &headersJSON, &e.CreatedAt, &e.RawSize,
		); err != nil {
			return nil, err
		}
		if !dateNull.IsZero() {
			datePtr = &dateNull
		}
		e.Date = datePtr
		if confNull != nil {
			e.Confidence = *confNull
		}
		if len(metricsJSON) > 0 {
			var m map[string]interface{}
			if err := json.Unmarshal(metricsJSON, &m); err == nil {
				e.Metrics = m
			}
		}
		if len(headersJSON) > 0 {
			var h map[string]string
			if err := json.Unmarshal(headersJSON, &h); err == nil {
				e.Headers = h
			}
		}
		out = append(out, &e)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return out, nil
}
