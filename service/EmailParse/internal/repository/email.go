package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/Zifeldev/emailback/service/EmailParse/internal/db"
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
	// UserID links this email to a user (nullable)
	UserID        *int64     `db:"user_id" json:"user_id,omitempty"`
	CreatedAt     time.Time  `db:"created_at" json:"created_at"`
	RawSize       int        `db:"raw_size" json:"raw_size"`
	Summary       *string    `db:"summary,omitempty" json:"summary,omitempty"`
	Priority      *string    `db:"priority,omitempty" json:"priority,omitempty"`
	PriorityScore *float64   `db:"priority_score,omitempty" json:"priority_score,omitempty"`
	AISumModel    *string    `db:"ai_sum_model,omitempty" json:"ai_sum_model,omitempty"`
	AIClsModel    *string    `db:"ai_cls_model,omitempty" json:"ai_cls_model,omitempty"`
	AIUpdatedAt   *time.Time `db:"ai_updated_at,omitempty" json:"ai_updated_at,omitempty"`
}

type EmailRepository interface {
	SaveEmail(ctx context.Context, email *EmailEntity) error
	GetByID(ctx context.Context, id string) (*EmailEntity, error)
	GetAll(ctx context.Context, limit, offset int) ([]*EmailEntity, error)
	GetByUserID(ctx context.Context, userID string, limit, offset int) ([]*EmailEntity, error)
	// UpdateAIFields updates AI-generated fields for an email (summary, models, priority, timestamp)
	UpdateAIFields(ctx context.Context, id string, summary *string, aiSumModel *string, priority *string, priorityScore *float64, aiClsModel *string, aiUpdatedAt *time.Time) error
}

const selectByUserID = `
SELECT id, message_id, from_addr, to_addrs, subject, date,
	   body_text, body_html, language, language_confidence,
	   metrics, headers, created_at, raw_size, summary, priority, priority_score, ai_sum_model, ai_cls_model, ai_updated_at
FROM emails
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3
`

func (r *PostgresEmailRepo) GetByUserID(ctx context.Context, userID string, limit, offset int) ([]*EmailEntity, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx, selectByUserID, userID, limit, offset)
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
		var summaryNT, priorityNT sql.NullString
		var priorityScoreNF sql.NullFloat64
		var aiSumNT, aiClsNT sql.NullString
		var aiUpdatedNT sql.NullTime

		if err := rows.Scan(
			&e.ID, &e.MessageID, &e.From, &e.To, &e.Subject, &dateNT,
			&e.Text, &e.HTML, &e.Language, &confNF,
			&metricsJSON, &headersJSON, &e.CreatedAt, &e.RawSize,
			&summaryNT, &priorityNT, &priorityScoreNF, &aiSumNT, &aiClsNT, &aiUpdatedNT,
		); err != nil {
			return nil, err
		}
		if dateNT.Valid {
			e.Date = &dateNT.Time
		}
		if confNF.Valid {
			e.Confidence = confNF.Float64
		}
		if summaryNT.Valid {
			s := summaryNT.String
			e.Summary = &s
		}
		if priorityNT.Valid {
			p := priorityNT.String
			e.Priority = &p
		}
		if priorityScoreNF.Valid {
			sc := priorityScoreNF.Float64
			e.PriorityScore = &sc
		}
		if aiSumNT.Valid {
			m := aiSumNT.String
			e.AISumModel = &m
		}
		if aiClsNT.Valid {
			m := aiClsNT.String
			e.AIClsModel = &m
		}
		if aiUpdatedNT.Valid {
			t := aiUpdatedNT.Time
			e.AIUpdatedAt = &t
		}
		if len(metricsJSON) > 0 {
			if err := json.Unmarshal(metricsJSON, &e.Metrics); err != nil {
				return nil, err
			}
		}
		if len(headersJSON) > 0 {
			if err := json.Unmarshal(headersJSON, &e.Headers); err != nil {
				return nil, err
			}
		}
		out = append(out, &e)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return out, nil
}

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
	language, language_confidence, metrics, headers, user_id, created_at, raw_size, summary, priority, priority_score, ai_sum_model, ai_cls_model, ai_updated_at
) VALUES (
	$1,$2,$3,$4,$5,$6,$7,$8,
	$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21
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
	raw_size = EXCLUDED.raw_size,
	summary = EXCLUDED.summary,
	priority = EXCLUDED.priority,
	priority_score = EXCLUDED.priority_score,
	ai_sum_model = EXCLUDED.ai_sum_model,
	ai_cls_model = EXCLUDED.ai_cls_model,
	ai_updated_at = EXCLUDED.ai_updated_at
`

const selectByID = `
SELECT id, message_id, from_addr, to_addrs, subject, date,
       body_text, body_html, language, language_confidence,
       metrics, headers, created_at, raw_size, summary, priority, priority_score, ai_sum_model, ai_cls_model, ai_updated_at
FROM emails WHERE id = $1
`

const selectAll = `
SELECT id, message_id, from_addr, to_addrs, subject, date,
       body_text, body_html, language, language_confidence,
       metrics, headers, created_at, raw_size, summary, priority, priority_score, ai_sum_model, ai_cls_model, ai_updated_at
FROM emails
ORDER BY created_at DESC
LIMIT $1 OFFSET $2
`

func (r *PostgresEmailRepo) SaveEmail(ctx context.Context, email *EmailEntity) error {
	if email.To == nil {
		email.To = []string{}
	}
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
		metricsJSON, headersJSON, email.UserID, createdAt, email.RawSize,
		email.Summary, email.Priority, email.PriorityScore, email.AISumModel, email.AIClsModel, email.AIUpdatedAt,
	)
	return err
}

func (r *PostgresEmailRepo) GetByID(ctx context.Context, id string) (*EmailEntity, error) {
	row := r.pool.QueryRow(ctx, selectByID, id)

	var email EmailEntity
	var metricsJSON, headersJSON []byte
	var dateNT sql.NullTime
	var confNF sql.NullFloat64

	var summaryNT, priorityNT sql.NullString
	var priorityScoreNF sql.NullFloat64
	var aiSumNT, aiClsNT sql.NullString
	var aiUpdatedNT sql.NullTime
	err := row.Scan(
		&email.ID, &email.MessageID, &email.From, &email.To, &email.Subject, &dateNT,
		&email.Text, &email.HTML, &email.Language, &confNF,
		&metricsJSON, &headersJSON, &email.CreatedAt, &email.RawSize,
		&summaryNT, &priorityNT, &priorityScoreNF, &aiSumNT, &aiClsNT, &aiUpdatedNT,
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
	if summaryNT.Valid {
		s := summaryNT.String
		email.Summary = &s
	}
	if priorityNT.Valid {
		p := priorityNT.String
		email.Priority = &p
	}
	if priorityScoreNF.Valid {
		sc := priorityScoreNF.Float64
		email.PriorityScore = &sc
	}
	if aiSumNT.Valid {
		m := aiSumNT.String
		email.AISumModel = &m
	}
	if aiClsNT.Valid {
		m := aiClsNT.String
		email.AIClsModel = &m
	}
	if aiUpdatedNT.Valid {
		t := aiUpdatedNT.Time
		email.AIUpdatedAt = &t
	}
	if len(metricsJSON) > 0 {
		if err := json.Unmarshal(metricsJSON, &email.Metrics); err != nil {
			return nil, err
		}
	}
	if len(headersJSON) > 0 {
		if err := json.Unmarshal(headersJSON, &email.Headers); err != nil {
			return nil, err
		}
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
		var summaryNT, priorityNT sql.NullString
		var priorityScoreNF sql.NullFloat64
		var aiSumNT, aiClsNT sql.NullString
		var aiUpdatedNT sql.NullTime

		if err := rows.Scan(
			&e.ID, &e.MessageID, &e.From, &e.To, &e.Subject, &dateNT,
			&e.Text, &e.HTML, &e.Language, &confNF,
			&metricsJSON, &headersJSON, &e.CreatedAt, &e.RawSize,
			&summaryNT, &priorityNT, &priorityScoreNF, &aiSumNT, &aiClsNT, &aiUpdatedNT,
		); err != nil {
			return nil, err
		}
		if dateNT.Valid {
			e.Date = &dateNT.Time
		}
		if confNF.Valid {
			e.Confidence = confNF.Float64
		}
		if summaryNT.Valid {
			s := summaryNT.String
			e.Summary = &s
		}
		if priorityNT.Valid {
			p := priorityNT.String
			e.Priority = &p
		}
		if priorityScoreNF.Valid {
			sc := priorityScoreNF.Float64
			e.PriorityScore = &sc
		}
		if aiSumNT.Valid {
			m := aiSumNT.String
			e.AISumModel = &m
		}
		if aiClsNT.Valid {
			m := aiClsNT.String
			e.AIClsModel = &m
		}
		if aiUpdatedNT.Valid {
			t := aiUpdatedNT.Time
			e.AIUpdatedAt = &t
		}
		if len(metricsJSON) > 0 {
			if err := json.Unmarshal(metricsJSON, &e.Metrics); err != nil {
				return nil, err
			}
		}
		if len(headersJSON) > 0 {
			if err := json.Unmarshal(headersJSON, &e.Headers); err != nil {
				return nil, err
			}
		}
		out = append(out, &e)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return out, nil
}

const updateAIFields = `
UPDATE emails SET
	summary = $2,
	ai_sum_model = $3,
	priority = $4,
	priority_score = $5,
	ai_cls_model = $6,
	ai_updated_at = $7
WHERE id = $1
`

func (r *PostgresEmailRepo) UpdateAIFields(ctx context.Context, id string, summary *string, aiSumModel *string, priority *string, priorityScore *float64, aiClsModel *string, aiUpdatedAt *time.Time) error {
	_, err := r.pool.Exec(ctx, updateAIFields,
		id,
		summary,
		aiSumModel,
		priority,
		priorityScore,
		aiClsModel,
		aiUpdatedAt,
	)
	return err
}
