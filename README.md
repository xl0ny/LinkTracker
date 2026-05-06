# LinkTracker

Telegram-бот, который отслеживает изменения на веб-страницах и информирует пользователя о них.

## Запуск

1. **Секреты в `.env`**:
   - `APP_TELEGRAM_TOKEN` — от [@BotFather](https://t.me/BotFather)
   - `POSTGRES_PASSWORD`
   - `REDIS_PASSWORD`

2. **Инфраструктура (Docker Compose)**  
   Postgres: `make compose-db` → `make compose-migrate`.  
   Kafka KRaft (3 брокера) + топики + Kafka UI + **Schema Registry** + Redis: `make compose-kafka`.

3. **Schema Registry** после поднятия кластера: схемы подтягиваются **при старте** scrapper/bot (REST `POST /subjects/.../versions`). Для ручной регистрации: `make avro-registrate` (по умолчанию `SCHEMA_REGISTRY_URL=http://localhost:18081`).

4. **Порты**: бот HTTP `cmd/bot/config.yaml` (`bot_port`), скраппер `cmd/scrapper/config.yaml` (`port`). **Schema Registry**: `http://localhost:18081` (не занимать `:8081` ботом).

5. **YAML**: `cmd/bot/config.yaml`, `cmd/scrapper/config.yaml` — брокеры Kafka, топики, `schema_registry_url`, `kafka.enabled`, у скраппера `producer.mode` (`direct` | `outbox`).

6. Запуск приложений: `make run-all` или `make run-bot` / `make run-scrapper`.

## ДЗ 5 допы (все сделаны кроме кросс-ревью)

- ✓ Ретраи в консьюмере при ошибках обработки.
- ✓ **Transactional Outbox**.
- ✓ **Avro**.

**Также:** идемпотентность консьюмера по `eventId` через **Redis**.
