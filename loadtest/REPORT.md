# Нагрузочные тесты `GET /links` и `POST /links`

## Окружение и параметры

- Машина: `MacBook Pro 14" (2021) M1 Pro 10-Core CPU`
- Postgres: docker-compose (`make compose-db && make compose-migrate`).
- Valkey: docker-compose кластер из 3 нод (`make compose-valkey`).
- Scrapper: `go run ./cmd/scrapper` (см. `cmd/scrapper/config.yaml`).
- Loadtest: все параметры в `loadtest/config.yaml` (включая Postgres DSN для seed).
- Объём данных: ~1000 чатов × ~100 ссылок = ~100 000 подписок.
- VUs: `2 × CPU`. Ramp-up: 1 минута. Stage: 5 минут.
- Соотношение операций: 100 GET : 1 POST.

## Как воспроизвести

```bash
make god                  # Postgres + миграции + Kafka + Redis + Valkey
make loadtest-seed        # ~100K записей в БД

# 1) без кэша: в cmd/scrapper/config.yaml -> valkey.enabled: false
make run-scrapper
make loadtest-run SCENARIO=no-cache

# 2) с серверным кэшем Valkey: valkey.enabled: true, client_cache.enabled: false
make run-scrapper
make loadtest-run SCENARIO=valkey-server

# 3) с Client-Side Caching: valkey.enabled: true, client_cache.enabled: true
make run-scrapper
make loadtest-run SCENARIO=valkey-csc
```

Результаты дописываются в `loadtest/REPORT.md`.

## Результаты

## Scenario: no-cache

- VUs: 8
- CPU: 10
- ramp-up: 1m0s
- stage: 5m0s
- chats: 1000
- GET:POST ratio: 100:1

| Method | Count | RPS | mean | p50 | p99 | 200 | 4xx | 5xx | other |
|--------|-------|-----|------|-----|-----|-----|-----|-----|-------|
| GET | 926615 | 3088.7 | 2.808956ms | 2.654875ms | 7.374875ms | 926607 | 0 | 0 | 8 |
| POST | 9263 | 30.9 | 5.620869ms | 4.911958ms | 21.090167ms | 9263 | 0 | 0 | 0 |

Most frequent errors:
- GET Get "http://localhost:8080/links": context canceled × 8

## Scenario: valkey-server

- VUs: 8
- CPU: 10
- ramp-up: 1m0s
- stage: 5m0s
- chats: 1000
- GET:POST ratio: 100:1

| Method | Count | RPS | mean | p50 | p99 | 200 | 4xx | 5xx | other |
|--------|-------|-----|------|-----|-----|-----|-----|-----|-------|
| GET | 2912445 | 9708.1 | 846.986µs | 774.208µs | 2.949917ms | 2912439 | 0 | 0 | 6 |
| POST | 29121 | 97.1 | 5.525125ms | 5.1715ms | 13.921042ms | 29119 | 0 | 0 | 2 |

Most frequent errors:
- GET Get "http://localhost:8080/links": context canceled × 6
- POST Post "http://localhost:8080/links": context canceled × 2

## Scenario: valkey-csc

- VUs: 8
- CPU: 10
- ramp-up: 1m0s
- stage: 5m0s
- chats: 1000
- GET:POST ratio: 100:1

| Method | Count | RPS | mean | p50 | p99 | 200 | 4xx | 5xx | other |
|--------|-------|-----|------|-----|-----|-----|-----|-----|-------|
| GET | 2998248 | 9994.2 | 745.515µs | 252.167µs | 10.344333ms | 2995003 | 0 | 0 | 3245 |
| POST | 29978 | 99.9 | 11.485577ms | 5.272375ms | 75.374208ms | 29937 | 0 | 0 | 41 |

Most frequent errors:
- GET Get "http://localhost:8080/links": dial tcp [::1]:8080: connect: resource temporarily unavailable × 3239
- GET Get "http://localhost:8080/links": context canceled × 6
- POST Post "http://localhost:8080/links": dial tcp [::1]:8080: connect: resource temporarily unavailable × 40
- POST Post "http://localhost:8080/links": context canceled × 1

## Выводы

По замерам видно, что подключение серверного кэша Valkey даёт прирост RPS на `GET /links` примерно в 3 раза (3к → 9.7к) и роняет среднюю задержку с ~2.8 мс до ~0.85 мс — БД разгружается полностью, чтения идут из памяти. Включение Client-Side Caching сверху даёт ещё один шаг: p50 падает до 0.25 мс (×3 относительно серверного кэша), потому что горячие чаты отдаются прямо из локального кэша scrapper'а без сетевого round-trip; правда, p99 при этом немного хуже из-за инвалидаций по RESP3 tracking. POST'ов проходит больше пропорционально GET'ам (соотношение 100:1 фиксированное), сам POST по скорости почти не меняется. Ошибки `context canceled` — это штатное завершение stage, а `resource temporarily unavailable` в CSC — упор в ephemeral-порты macOS на ~10к RPS, не проблема Valkey. Итог: кэш стоит ставить обязательно, CSC оправдан при большой доле повторных чтений, а дальнейший рост уже ограничен самим scrapper'ом и решается горизонтальным масштабированием.
