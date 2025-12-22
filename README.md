# Emailback — краткий гайд / quick start

Русский
-----

Небольшой сервис для парсинга писем и асинхронной обработки AI-операций.

Коротко — как запустить локально (Docker Compose):

- Убедитесь, что в корне проекта есть файл `.env` с переменными окружения (DB, JWT, HF_TOKEN и др.).
- Запустить стек:

```bash
docker compose up -d --build postgres redis migrate-email migrate-auth app aiworker prometheus grafana
```

- Сервисы:
  - API (app): http://localhost:8080
  - AI worker metrics: http://localhost:9092/metrics (host 9092 -> контейнер 8080)
  - Prometheus: http://localhost:9090
  - Grafana: http://localhost:3000 (admin/admin)

- Важные метрики (Prometheus):
  - emailback_emails_processed_total — общее число успешно обработанных писем
  - emailback_emails_failed_total — общее число неуспешных обработок
  - emailback_email_processing_seconds_* — histogram для времени обработки

- Ошибки парсинга EML:
  - При некорректном EML API возвращает HTTP 400 (Bad Request). В логах видно сообщение вроде:
    "Failed to ReadParts: malformed MIME header initial line: ..." — это означает, что входные данные не соответствуют ожидаемому EML/MIME формату.
  - Рекомендуется проверять вход и при необходимости очищать/валидировать тело письма прежде чем отправлять в `/parse`.


English
-------

Small service for parsing emails and offloading heavy AI tasks to a background worker.

Quick start (Docker Compose):

- Make sure you have a `.env` file in project root with DB, JWT and HF_TOKEN variables.
- Start the stack:

```bash
docker compose up -d --build postgres redis migrate-email migrate-auth app aiworker prometheus grafana
```

- Services:
  - API (app): http://localhost:8080
  - AI worker metrics: http://localhost:9092/metrics (host 9092 -> container 8080)
  - Prometheus: http://localhost:9090
  - Grafana: http://localhost:3000 (admin/admin)

- Key Prometheus metrics:
  - emailback_emails_processed_total — total processed emails
  - emailback_emails_failed_total — total failed
  - emailback_email_processing_seconds_* — histogram for processing durations

- EML parsing errors:
  - On malformed EML the API returns HTTP 400. Logs show:
    "Failed to ReadParts: malformed MIME header initial line: ..." — meaning the submitted EML is invalid for the parser.
  - Validate or sanitize input before POST /parse to avoid 400 responses.

Troubleshooting
---------------
- Prometheus targets: http://localhost:9090 → Status → Targets — check `app:8080` and `aiworker:8080` (scraped via host ports 8080/9092 mapping).
- If Grafana dashboard panels are empty, ensure the dashboard JSON uses the actual metric names (`emailback_*`) and Prometheus has those series.
- To monitor Redis queue length add `redis_exporter` to docker-compose and scrape it from Prometheus.

