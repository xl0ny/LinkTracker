# LinkTracker

Telegram-бот для отслеживания обновлений по ссылкам и отправки уведомлений в чат.

[Открыть бота в Telegram](https://t.me/teducationlinktracerkbot)

Проект состоит из трех сервисов:

- `bot` - принимает команды Telegram и отправляет уведомления;
- `scrapper` - хранит подписки и проверяет обновления;
- `agent` - обрабатывает найденные обновления через AI.

## Стек

Go, PostgreSQL, Kafka, Schema Registry, Redis, Valkey, Prometheus, Grafana, Docker Compose.

## Требования

- Go
- Docker и Docker Compose
- Telegram bot token от [@BotFather](https://t.me/BotFather)

## Настройка

Создайте `.env` в корне проекта:

```env
APP_TELEGRAM_TOKEN=
POSTGRES_PASSWORD=
REDIS_PASSWORD=
VALKEY_PASSWORD=
HUGGINGFACE_TOKEN=
```

## Запуск

Полный локальный запуск:

```bash
make god
```

Команда поднимает инфраструктуру, применяет миграции, регистрирует Avro-схемы и запускает `bot`, `scrapper`, `agent`.
Мониторинг в `make god` не входит.

Запуск по частям:

```bash
make compose-db
make compose-migrate
make compose-kafka
make compose-valkey
make avro-registrate
make run-all
```

Отдельные сервисы:

```bash
make run-bot
make run-scrapper
make run-agent
```

## Тесты

```bash
make test
make test-integration
```

Интеграционные тесты используют Testcontainers, поэтому для них нужен запущенный Docker.

## Порты

| Сервис | Порт |
| --- | --- |
| Scrapper API | `8080` |
| Bot API | `8081` |
| Agent health | `8082` |
| Bot metrics | `8011` |
| Kafka UI | `8085` |
| Schema Registry | `18081` |
| Prometheus | `9090` |
| Grafana | `3000` |

Мониторинг запускается отдельно:

```bash
make compose-observability
```

Подробнее: [OBSERVABILITY.md](OBSERVABILITY.md).

## Структура

```text
cmd/          точки входа сервисов
internal/     бизнес-логика
pkg/          общие пакеты
migrations/   миграции PostgreSQL
schemas/      Avro-схемы Kafka-сообщений
```
