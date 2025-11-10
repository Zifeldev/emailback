# Quick Start

## Setup

1. **Copy environment template:**
   ```bash
   cp .env.example .env
   ```

2. **Edit `.env`:**
   - Set your `HF_TOKEN` (get it at https://huggingface.co/settings/tokens)
   - Change `JWT_ACCESS_SECRET` and `JWT_REFRESH_SECRET` (use strong random strings!)
   - Update database passwords if needed

3. **Start all services:**
   ```bash
   docker compose up -d
   ```

   This starts:
   - **Auth Service** (8081) - JWT authentication
   - **Email Service** (8080) - Email parsing with AI
   - 2x PostgreSQL (5432, 5433)
   - 2x Redis (6379, 6380)

4. **Check status:**
   ```bash
   docker compose ps
   curl http://localhost:8081/health  # Auth
   curl http://localhost:8080/health  # Email
   ```

## Usage

### Authentication Flow

```bash
# 1. Register user
curl -X POST http://localhost:8081/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"test@example.com","password":"test123"}'

# 2. Login
curl -X POST http://localhost:8081/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"test@example.com","password":"test123"}'

# Response: { "access_token": "...", "refresh_token": "..." }

# 3. Use access token for Email API
TOKEN="your_access_token_here"
curl http://localhost:8080/api/emails \
  -H "Authorization: Bearer $TOKEN"
```

### Development Commands

```bash
# View logs
docker compose logs -f auth-service
docker compose logs -f email-service

# Restart service
docker compose restart email-service

# Stop all
docker compose down

# Rebuild
docker compose down && docker compose build && docker compose up -d
```

## Services

| Service | Port | Description |
|---------|------|-------------|
| Auth    | 8081 | JWT authentication (register/login/refresh) |
| Email   | 8080 | Email parsing with AI (requires JWT) |
| Auth DB | 5433 | PostgreSQL for auth data |
| Email DB| 5432 | PostgreSQL for email data |

## Endpoints

### Auth Service (8081)

- `POST /api/auth/register` - Create account
- `POST /api/auth/login` - Get JWT tokens
- `POST /api/auth/refresh` - Refresh access token
- `POST /api/auth/logout` - Invalidate tokens
- `GET /health` - Health check

### Email Service (8080)

All require `Authorization: Bearer <token>`:

- `POST /api/parse` - Parse email
- `GET /api/emails` - List emails
- `GET /health` - Health check (public)

## Multilingual AI Models

Configure in `.env` via `AI_SUM_MODELS`:
```json
{
  "en": "sshleifer/distilbart-cnn-12-6",
  "ru": "RussianNLP/FRED-T5-Summarizer",
  "de": "Einmalumdiewelt/T5-Base_GNAD"
}
```

## Troubleshooting

**Check logs:**
```bash
docker logs emailback-app-1
docker logs emailback-postgres-1
```

**Reset database:**
```bash
docker compose down -v
docker compose up -d
```
