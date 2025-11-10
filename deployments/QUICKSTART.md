# Quick Start

## Setup

1. **Copy environment template:**
   ```bash
   cp .env.example .env
   ```

2. **Edit `.env`:**
   - Set your `HF_TOKEN` (get it at https://huggingface.co/settings/tokens)
   - Change `DB_PASSWORD` if needed

3. **Start services:**
   ```bash
   docker compose up -d
   ```

4. **Check status:**
   ```bash
   docker compose ps
   curl http://localhost:8080/health
   ```

## Usage

### Development
```bash
# Start
docker compose up -d

# Logs
docker compose logs -f app

# Stop
docker compose down

# Rebuild
docker compose down && docker compose build app && docker compose up -d
```

### Production
```bash
# Create secrets
mkdir -p ../secrets
echo "strong_password" > ../secrets/db_password
chmod 600 ../secrets/db_password

# Start
docker compose -f docker-compose.prod.yml --env-file .env.prod up -d
```

## Endpoints

- API: http://localhost:8080
- Health: http://localhost:8080/health
- Metrics: http://localhost:8080/metrics

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
