# LinkTracker

Telegram-бот, который отслеживает изменения на веб-страницах и информирует пользователя о них.
## Запуск

1. **Секреты в `.env`**:
   - `APP_TELEGRAM_TOKEN` — от [@BotFather](https://t.me/BotFather)
   - `POSTGRES_PASSWORD`
   - `REDIS_PASSWORD`
   - `VALKEY_PASSWORD`
   - `HUGGINGFACE_TOKEN` https://huggingface.co/settings/tokens

2. **Инфраструктура (Docker Compose)**
   Postgres: `make compose-db` → `make compose-migrate`.
   Kafka KRaft (3 брокера) + топики (`link.raw-updates`, `link.processed-updates`, `failed-links`, `*-dlq`) + Kafka UI + **Schema Registry** + Redis: `make compose-kafka`.
   Valkey-кластер (3 ноды): `make compose-valkey`.
   Всё разом (Postgres + миграции + Redis + Valkey + Kafka + регистрация Avro + запуск bot/scrapper/agent): `make god`.

3. **Schema Registry** после поднятия кластера: схемы подтягиваются **при старте** scrapper/bot (REST `POST /subjects/.../versions`). Для ручной регистрации: `make avro-registrate` (по умолчанию `SCHEMA_REGISTRY_URL=http://localhost:18081`).

4. **Порты**: бот HTTP — `cmd/bot/config.yaml` (`bot_port`, по умолчанию 8081), scrapper — `cmd/scrapper/config.yaml` (`port`, 8080), AI Agent health-чек — `cmd/agent/config.yaml` (`port`, 8082).

5. **Запуск приложений**: `make run-all` либо по одному сервису:
   - `make run-bot`
   - `make run-scrapper`
   - `make run-agent`
   Либо `make god` для запуска всего





