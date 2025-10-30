package db

import (
    "context"
    "time"

    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/pgconn"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

type TimeoutPool struct {
    *pgxpool.Pool
    QueryTimeout time.Duration
}

func (tp *TimeoutPool) Ping(ctx context.Context) error {
    ctx, cancel := context.WithTimeout(ctx, tp.QueryTimeout)
    defer cancel()
    return tp.Pool.Ping(ctx)
}


func (p *TimeoutPool) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
    ctx, cancel := context.WithTimeout(ctx, p.QueryTimeout)
    defer cancel()
    return p.Pool.Exec(ctx, sql, args...)
}

func (p *TimeoutPool) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
    ctx, cancel := context.WithTimeout(ctx, p.QueryTimeout)
    defer cancel()
    return p.Pool.Query(ctx, sql, args...)
}

func (p *TimeoutPool) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
    ctx, cancel := context.WithTimeout(ctx, p.QueryTimeout)
    defer cancel()
    return p.Pool.QueryRow(ctx, sql, args...)
}
