# Auth Service

JWT-based authentication microservice with access and refresh tokens.

## Features

- User registration with email/password
- Login with JWT access + refresh tokens
- Token refresh mechanism
- Token revocation (logout)
- PostgreSQL for user & token storage
- Redis for session caching (optional)
- Health checks
- Rate limiting support

## Architecture

```
service/
├── Auth/              # Auth microservice (Port 8081)
│   ├── cmd/
│   │   └── main.go
│   ├── internal/
│   │   ├── config/
│   │   ├── controllers/   # auth, health
│   │   ├── db/
│   │   ├── logger/
│   │   ├── middleware/    # JWT validation
│   │   ├── models/
│   │   ├── repository/    # user, token repos
│   │   └── service/       # auth service
│   └── go.mod
│
└── EmailParse/        # Email parser microservice (Port 8080)
    └── ... (existing structure)
```

## Database Schema

### users
- id (BIGSERIAL PK)
- email (VARCHAR UNIQUE)
- password_hash (VARCHAR)
- created_at, updated_at (TIMESTAMP)

### refresh_tokens
- id (BIGSERIAL PK)
- user_id (BIGINT FK -> users.id)
- token (VARCHAR UNIQUE)
- expires_at (TIMESTAMP)
- created_at (TIMESTAMP)
- revoked (BOOLEAN)

## API Endpoints

### Public Endpoints

**POST /auth/register**
```json
{
  "email": "user@example.com",
  "password": "password123"
}
```
Response:
```json
{
  "access_token": "eyJhbGc...",
  "refresh_token": "random_base64...",
  "expires_in": 900
}
```

**POST /auth/login**
```json
{
  "email": "user@example.com",
  "password": "password123"
}
```

**POST /auth/refresh**
```json
{
  "refresh_token": "previous_refresh_token"
}
```

**POST /auth/logout**
```json
{
  "refresh_token": "token_to_revoke"
}
```

**GET /health** - Health check

### Protected Endpoints

**GET /api/me** - Get current user info (requires Authorization header)

## Environment Variables

```bash
# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=auth
DB_PASSWORD=changeme
DB_NAME=auth
DB_SSLMODE=disable
DB_MAX_CONNS=25
DB_MIN_CONNS=5
DB_QUERY_TIMEOUT=5s

# HTTP
HTTP_HOST=:8081
SHUTDOWN_TIMEOUT=10s
REQUEST_TIMEOUT=30s

# Logger
LOG_LEVEL=info

# Redis (optional)
REDIS_ENABLED=true
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
REDIS_DB=1
REDIS_PREFIX=auth:
REDIS_TTL=24h

# JWT
JWT_ACCESS_SECRET=your_access_secret_min_32_chars
JWT_REFRESH_SECRET=your_refresh_secret_min_32_chars
JWT_ACCESS_EXPIRATION=15m
JWT_REFRESH_EXPIRATION=7d
JWT_ISSUER=emailback-auth

# Rate Limiting
RATE_LIMIT_ENABLED=false
RATE_LIMIT_INTERVAL=1m
RATE_LIMIT_MAX=100
```

## Quick Start

```bash
cd service/Auth
go mod tidy
go run cmd/main.go
```

## Integration with EmailParse Service

### Option 1: JWT Middleware in EmailParse

Add JWT validation middleware to EmailParse to protect endpoints:

```go
// In EmailParse service
import "github.com/Zifeldev/emailback/service/Auth/internal/middleware"

// Add to protected routes
protected := r.Group("/api/emails")
protected.Use(middleware.JWTAuth(authService))
{
    protected.GET("", emailController.GetAll)
    protected.POST("", emailController.ParseAndSave)
}
```

### Option 2: API Gateway

Use reverse proxy (nginx, traefik) to route:
- `/auth/*` -> Auth Service (8081)
- `/api/*` -> EmailParse Service (8080)

Gateway validates JWT before forwarding to EmailParse.

### Option 3: Service-to-Service

EmailParse validates JWT by calling Auth service:

```go
// In EmailParse middleware
func ValidateToken(authServiceURL string) gin.HandlerFunc {
    return func(c *gin.Context) {
        token := c.GetHeader("Authorization")
        // Call Auth service /api/validate endpoint
        // If valid, set user context
    }
}
```

## Docker Deployment

See `deployments/docker-compose-auth.yml` for complete setup with PostgreSQL and Redis.

## Security Best Practices

1. **Secrets**: Use strong random secrets (min 32 chars) for JWT_ACCESS_SECRET and JWT_REFRESH_SECRET
2. **HTTPS**: Always use HTTPS in production
3. **Token Expiration**: Keep access tokens short-lived (15m), refresh tokens longer (7d)
4. **Refresh Token Rotation**: Old refresh token is revoked when new pair is issued
5. **Password Hashing**: bcrypt with default cost (10)
6. **Rate Limiting**: Enable rate limiting to prevent brute force attacks

## Testing

```bash
# Register
curl -X POST http://localhost:8081/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"password123"}'

# Login
curl -X POST http://localhost:8081/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"password123"}'

# Access protected endpoint
curl -X GET http://localhost:8081/api/me \
  -H "Authorization: Bearer <access_token>"

# Refresh token
curl -X POST http://localhost:8081/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{"refresh_token":"<refresh_token>"}'
```

## Monitoring

- Health check: `GET /health`
- Metrics endpoint: Can be added with Prometheus (similar to EmailParse)
- Logs: JSON structured logs via logrus

## Next Steps

1. Run migrations: `migrate -path db/auth_migrations -database "postgres://..." up`
2. Generate JWT secrets: `openssl rand -base64 32`
3. Configure `.env` file
4. Start service: `go run cmd/main.go`
5. Integrate with EmailParse service using one of the options above
