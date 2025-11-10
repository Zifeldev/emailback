# Email Service Deployment

Docker Compose configuration for Email Parser microservice with AI-powered summarization.

## Files

- `docker-compose.yml` - Base configuration
- `docker-compose.dev.yml` - Development overrides
- `docker-compose.prod.yml` - Production configuration
- `.env` - Development environment variables
- `.env.prod` - Production environment variables
- `.env.example` - Template with all available options

## Environment Variables

All credentials and configuration are loaded from `.env` files - no hardcoded defaults in compose files.

### Required Variables

```properties
DB_USER=emailback
DB_PASSWORD=changeme
DB_NAME=emailback
HF_TOKEN=hf_your_token_here
AI_SUM_MODELS={"en":"model1","ru":"model2"}
```

See `.env.example` for complete list of available variables.

## Development

Uses base config + dev overrides with exposed ports:

```bash
docker compose up -d
```

**Features:**
- Exposed ports: 8080 (HTTP), 5432 (PostgreSQL), 6379 (Redis)
- Debug logging
- Password in `.env` file

## Production

Uses separate compose file with Docker secrets:

```bash
# Setup secrets
echo "strong_password" > ../secrets/db_password
chmod 600 ../secrets/db_password

# Deploy
docker compose -f docker-compose.prod.yml --env-file .env.prod up -d
```

**Features:**
- No exposed ports except 8080
- Warn-level logging
- Password in secrets file
- Auto-restart enabled
- Redis persistence enabled
- Rate limiting enabled

## Multilingual AI Support

Configure language-specific summarization models via `AI_SUM_MODELS` JSON:

```json
{
  "en": "sshleifer/distilbart-cnn-12-6",
  "ru": "RussianNLP/FRED-T5-Summarizer",
  "de": "Einmalumdiewelt/T5-Base_GNAD",
}
```

Language is auto-detected, and appropriate model is selected.

## Commands

```bash
# View logs
docker logs emailback-app-1 -f

# Check status
docker compose ps

# Restart service
docker compose restart app

# Clean rebuild
docker compose down -v && docker compose up -d
```

## Security Notes

- Never commit `.env` or `.env.prod` files
- Use strong passwords in production
- Store sensitive tokens in secrets
- Keep HF_TOKEN private

## Documentation

See [QUICKSTART.md](./QUICKSTART.md) for step-by-step setup guide.
