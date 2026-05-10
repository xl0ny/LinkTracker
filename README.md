# LinkTracker

Telegram-бот, который отслеживает изменения на веб-страницах и информирует пользователя о них.

## Запуск

1. `.env`: `APP_TELEGRAM_TOKEN` (от [@BotFather](https://t.me/BotFather)).
2. БД: `make compose-db`, затем `make compose-migrate` (или `make run-db-migration` с `cmd/migrator/config.yaml` и теми же `POSTGRES_*`).
3. Для скраппера в env: `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB`, `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_SSL_MODE`; опционально `APP_SCRAPPER_ACCESS_TYPE` (`SQL` / `ORM`, по умолчанию SQL).
4. В `cmd/bot/config.yaml` — `scrapper_url` на HTTP скраппера (порт как у `APP_SCRAPPER_PORT`).
5. `make run-all` — бот и скраппер; по отдельности: `make run-bot`, `make run-scrapper`.

## Сборка и тесты

- `make build` — бинарники в `bin/`
- `make test` — юнит-тесты; `make test-integration` — БД в Docker (Testcontainers)

Для отладочных логов — задать `logging.mode: "DEBUG"` в `config.yaml`.

---

## Бонус к ДЗ 4 (многопоточная обработка ссылок)

<span style="color:#d32f2f; font-weight:700">! ДОП. ЗАДАНИЕ СДЕЛАНО</span>

Реализовано по смыслу бонусного NFR: **параллельная обработка** подписок в батчах из БД (`batch.size` в `cmd/scrapper/config.yaml`, диапазон 50–500), **постоянный пул воркеров** (`scheduler.workers`), ошибка по одной ссылке **не роняет** остальные, пользователь получает **отчёт** по ссылкам с ошибками (`NotifyFailedLinks`). Число потоков задаётся в конфиге, не хардкод.

Планировщик: пакетная выгрузка ссылок из репозитория → задачи в канал → воркеры обрабатывают параллельно
