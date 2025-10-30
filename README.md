

### start (Docker Compose)
Dev:
1) cd deployments
2) docker compose up -d postgres redis
3) docker compose up migrate
4) docker compose up -d app
5) curl http://localhost:8080/health

Prod (example):
1) Ensure `secrets/db_password` contains only the DB password
2) docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d postgres redis
3) docker compose -f docker-compose.yml -f docker-compose.prod.yml up migrate
4) docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d app

### Environment variables

Database (required):
- DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME, DB_SSLMODE (usually "disable")
- DB_MAX_CONNS, DB_MIN_CONNS
- DB_MAX_CONN_LIFETIME (e.g. 30m), DB_MAX_CONN_IDLE_TIME (e.g. 5m)
- DB_HEALTH_CHECK_PERIOD (e.g. 1m), DB_QUERY_TIMEOUT (e.g. 450ms)

HTTP:
- HTTP_HOST (default :8080)
- HTTP_SHUTDOWN_TIMEOUT (default 10s)
- HTTP_REQUEST_TIMEOUT (per-request timeout; default 500ms if unset)

Logger:
- LOGGER_LEVEL (info|debug|warn|error)

Strict mode:
- STRICT=true enables strict env validation (panics on missing required values)

Compose app service:

- POST /parse — body: raw RFC822, returns parsed entity
- POST /parse/batch — JSON [{ raw: "..." }], concurrent parsing with per-item timeout
- GET /emails/{id}
- GET /emails?limit&offset
- GET /health — checks Postgres (+Redis if enabled)
- GET /swagger/index.html
- GET /metrics (Prometheus)

### Tests & coverage
- Run all tests with coverage summary:
```
go test ./... -cover -coverprofile=coverage.out && go tool cover -func=coverage.out | tail -n 1
```

### Notes
- DB migrations are applied via the `migrate` one-shot service in compose.
- Ensure `secrets/db_password` contains only the password (no quotes/newlines).