# Observability

Метрики отдаются в формате Prometheus (NFR-1). Длительности унифицированы через histogram с лейблами `scope` и `scope_type` (NFR-2).

## Эндпоинты

| Сервис   | URL                      | Порт (по умолчанию) |
|----------|--------------------------|---------------------|
| Scrapper | http://localhost:8080/metrics | 8080 (`cmd/scrapper/config.yaml`) |
| Bot      | http://localhost:8011/metrics | 8011 (`metrics_port` в `cmd/bot/config.yaml`) |

Bot API (Swagger, `/updates`) остаётся на `bot_port` (8081).

## Метрики Scrapper

| Метрика | Тип | Лейблы |
|---------|-----|--------|
| `links_on_track_total` | Gauge | `tracked_source` (github, stackoverflow) |
| `request_duration_ms_total` | Histogram | `scope` (database, external_source, kafka, llm_agent), `scope_type` |
| `api_requests_total` | Counter | `source` |
| `http_requests_total` / `http_request_duration_seconds` | Counter / Histogram | RED, `app=scrapper` |

Инструментированы: PostgreSQL (`metricsrepo`), GitHub/StackOverflow (`checker`), Kafka producer/outbox (`scope=kafka`), публикация в `link.raw-updates` дополнительно пишет `scope=llm_agent`, `scope_type=huggingface`, HTTP API (RED + `api_requests_total`, `source=bot` от клиента бота).

## Метрики Bot

| Метрика | Тип | Лейблы |
|---------|-----|--------|
| `command_requests_total` | Counter | `command` |
| `command_duration_ms_total` | Histogram | `scope` (scrapper_sync_api, scrapper_async_api), `scope_type` |
| `sent_notification_total` | Counter | — |
| `telegram_requests_total` | Counter | `request_type` (command, plain_message, callback, other) |

## Pushgateway (опционально)

В `cmd/*/config.yaml` → `metrics.pushgateway`: `enabled: true`, `url`, `job`, `interval`.

## Стек мониторинга

```bash
make compose-observability
```

- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000 (логин `admin` / `admin`)
- Pushgateway: http://localhost:9091

Приложения запускаются локально (`make run-scrapper`, `make run-bot`). Prometheus скрейпит `host.docker.internal:8080` и `:8011`.

Дашборды: папка **Link Tracker** — «RED & Runtime», «Business Metrics». PromQL — в `example_pql.txt`.

## Docker-образы

```bash
make docker-build-apps
```

## Скриншоты для MR

1. Поднять `make compose-observability`, запустить bot + scrapper.
2. Prometheus → Graph: запросы из `example_pql.txt`.
3. Grafana → дашборды, variable `app`.
4. `curl localhost:8080/metrics` и `curl localhost:8011/metrics`.
