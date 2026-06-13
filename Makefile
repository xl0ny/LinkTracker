COVERAGE_FILE ?= coverage.out
SCHEMA_REGISTRY_URL ?= http://localhost:18081

# Get all directories in cmd/ as available modules
MODULES := $(notdir $(wildcard cmd/*))

# Help target - display usage information
GOLANGCI_LINT = $(shell go env GOPATH)/bin/golangci-lint

.PHONY: help
help:
	@echo "Available commands:"
	@echo "  \033[36mmake build\033[0m - Build all modules ($(MODULES))"
	@$(foreach mod,$(MODULES),echo "  \033[36mmake build_$(mod)\033[0m - Build $(mod) module";)
	@echo "  \033[36mmake test\033[0m - Run all tests"
	@echo "  \033[36mmake test-integration\033[0m - DB integration tests (Docker / Testcontainers)"
	@echo "  \033[36mmake run-all\033[0m - Run bot, scrapper and ai-agent together (Ctrl+C stops all)"
	@echo "  \033[36mmake run-bot\033[0m / \033[36mrun-scrapper\033[0m / \033[36mrun-agent\033[0m - Run a single service"
	@echo "  \033[36mmake compose-db\033[0m - Postgres via Docker Compose (локальная разработка по ДЗ)"
	@echo "  \033[36mmake compose-migrate\033[0m - Применить SQL-миграции к compose-Postgres (отдельный шаг по ДЗ)"
	@echo "  \033[36mmake compose-kafka\033[0m - Kafka KRaft (3 брокера) + Kafka UI + Schema Registry + создать топики"
	@echo "  \033[36mmake compose-kafka-up\033[0m - Только 3 брокера Kafka (без UI и без создания топиков)"
	@echo "  \033[36mmake compose-kafka-broker BROKER=1\033[0m - Поднять только один брокер (1, 2 или 3)"
	@echo "  \033[36mmake compose-kafka-init\033[0m - Создать/проверить топики (link.raw-updates, link.processed-updates, failed-links, *-dlq)"
	@echo "  \033[36mmake compose-kafka-ui\033[0m - Запустить Kafka UI (http://localhost:8085)"
	@echo "  \033[36mmake compose-kafka-down\033[0m - Остановить Kafka кластер и UI (volume'ы НЕ удаляются)"
	@echo "  \033[36mmake compose-kafka-purge\033[0m - Полная очистка кластера: контейнеры + volume'ы (потеря данных)"
	@echo "  \033[36mmake compose-kafka-logs\033[0m - tail -f логов всех брокеров"
	@echo "  \033[36mmake compose-kafka-topics\033[0m - Список топиков в кластере"
	@echo "  \033[36mmake avro-registrate\033[0m - Register Avro schemas in Schema Registry"
	@echo "  \033[36mmake compose-valkey\033[0m - Поднять Valkey кластер (3 ноды)"
	@echo "  \033[36mmake compose-valkey-down\033[0m / \033[36mcompose-valkey-purge\033[0m - Остановить / удалить кластер"
	@echo "  \033[36mmake loadtest-seed\033[0m - Засеять Postgres данными для нагрузочных тестов"
	@echo "  \033[36mmake loadtest-run SCENARIO=name\033[0m - Прогнать нагрузочный тест и дописать отчёт"
	@echo "  \033[36mmake compose-observability\033[0m - Prometheus (9090), Grafana (3000), Pushgateway (9091)"
	@echo "  \033[36mmake docker-build-apps\033[0m - Build bot, scrapper and agent Docker images"
	@echo "  \033[36mmake compose-apps\033[0m - Build and run bot, scrapper and agent containers"
	@echo "  \033[36mmake compose-apps-down\033[0m / \033[36mcompose-apps-logs\033[0m - Stop / tail app containers"
	@echo "  \033[36mmake docker-up-all\033[0m - Docker-only deploy: infra + Avro schemas + app containers"
	@echo "  \033[36mmake docker-down-all\033[0m - Stop Docker app containers and compose infrastructure"
	@echo "  \033[36mmake lint\033[0m - Run golangci-lint"

.PHONY: build
build:
	@echo "Building all modules: $(MODULES)"
	@mkdir -p bin
	@$(foreach mod,$(MODULES),echo "Building module: $(mod)"; go build -o ./bin/$(mod) ./cmd/$(mod);)

# Convenience targets for building individual modules
.PHONY: $(addprefix build_,$(MODULES))
$(addprefix build_,$(MODULES)):
	@modulename=$(subst build_,,$@); \
	echo "Building module: $$modulename"; \
	mkdir -p bin; \
	go build -o ./bin/$$modulename ./cmd/$$modulename

# test: run all tests
#   -race       : detect data races
#   -count=1    : disable test cache (run tests every time)
#   -coverpkg   : collect coverage for all packages in the module
#   -coverprofile : write coverage to file
.PHONY: test
test:
	@go test -coverpkg='gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/...' --race -count=1 -coverprofile='$(COVERAGE_FILE)' ./...
	@go tool cover -func='$(COVERAGE_FILE)' | grep ^total | tr -s '\t'

.PHONY: test-integration
test-integration:
	@go test -tags=integration -count=1 -v ./internal/scrapper/infrastructure/db/... ./internal/bot/transport/kafka/... ./internal/agent/infrastructure/kafka/... ./internal/scrapper/infrastructure/valkey/...

.PHONY: lint
lint:
	@command -v $(GOLANGCI_LINT) >/dev/null 2>&1 || (echo "golangci-lint v2 not found. Run: go install github.com/golangci/golangci-lint/v2@latest" && exit 1)
	@$(GOLANGCI_LINT) run -c .golangci.yml --timeout=5m


.PHONY: god
god:
	@echo "\033[36mgod:\033[0m Postgres..."
	@$(MAKE) compose-db
	@echo "\033[36mgod:\033[0m migrations..."
	@$(MAKE) compose-migrate
	@echo "\033[36mgod:\033[0m Redis..."
	@docker compose up -d redis
	@echo "\033[36mgod:\033[0m Valkey cluster (3 nodes)..."
	@$(MAKE) compose-valkey
	@echo "\033[36mgod:\033[0m Kafka + Schema Registry + topics"
	@$(MAKE) compose-kafka
	@echo "\033[36mgod:\033[0m ждём Schema Registry ($(SCHEMA_REGISTRY_URL))..."
	@i=0; until curl -fsS "$(SCHEMA_REGISTRY_URL)/subjects" >/dev/null 2>&1; do \
		i=$$((i+1)); test $$i -le 120 || (echo "god: schema-registry недоступен, выход" && exit 1); \
		sleep 1; \
	done
	@echo "\033[36mgod:\033[0m Avro в Schema Registry..."
	@$(MAKE) avro-registrate
	@echo "\033[36mgod:\033[0m бот + скраппер + ai-agent"
	@$(MAKE) run-all

.PHONY: run-bot
run-bot:
	@echo "Running bot"
	@go run ./cmd/bot/main.go

.PHONY: run-scrapper
run-scrapper:
	@echo "Running scrapper"
	@go run ./cmd/scrapper/main.go

.PHONY: run-agent
run-agent:
	@echo "Running ai-agent"
	@go run ./cmd/agent/main.go

.PHONY: run-all
run-all:
	@echo "Running bot, scrapper and agent (Ctrl+C stops all)"
	@trap 'kill $$BOT $$SCRAPPER $$AGENT 2>/dev/null; exit 130' INT TERM; \
	go run ./cmd/bot/main.go & BOT=$$!; \
	go run ./cmd/scrapper/main.go & SCRAPPER=$$!; \
	go run ./cmd/agent/main.go & AGENT=$$!; \
	wait $$BOT $$SCRAPPER $$AGENT

.PHONY: generate-api
generate-api:
	@go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest -generate types,chi-server -package api -o internal/bot/transport/http/api/openapi_gen.go internal/bot/api/contract.yaml
	@go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest -generate types,client     -package botclient -o internal/scrapper/infrastructure/botclient/openapi_client_gen.go internal/bot/api/contract.yaml
	@go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest -generate types,chi-server -package api -o internal/scrapper/transport/http/api/openapi_gen.go internal/scrapper/api/contract.yaml
	@go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest -generate types,client     -package scrapperclient -o internal/bot/infrastructure/scrapperclient/openapi_client_gen.go internal/scrapper/api/contract.yaml
	@echo "API generated"

.PHONY: run-db-migration
run-db-migration:
	@go run ./migrations/migrator/main.go

.PHONY: compose-observability
compose-observability:
	@echo "Prometheus http://localhost:9090  Grafana http://localhost:3000 (admin/admin)  Pushgateway :9091"
	@docker compose up -d prometheus grafana pushgateway

.PHONY: docker-build-apps
docker-build-apps:
	@docker build -f deploy/docker/Dockerfile.scrapper -t link-tracker-scrapper:latest .
	@docker build -f deploy/docker/Dockerfile.bot -t link-tracker-bot:latest .
	@docker build -f deploy/docker/Dockerfile.agent -t link-tracker-agent:latest .

.PHONY: compose-apps
compose-apps:
	@docker compose up -d --build bot-app scrapper-app agent-app

.PHONY: compose-apps-down
compose-apps-down:
	@docker compose stop bot-app scrapper-app agent-app || true
	@docker compose rm -f bot-app scrapper-app agent-app || true

.PHONY: compose-apps-logs
compose-apps-logs:
	@docker compose logs -f bot-app scrapper-app agent-app

.PHONY: compose-db
compose-db:
	@docker compose up -d db

.PHONY: compose-migrate
compose-migrate:
	@docker compose --profile migrate run --rm migrator

KAFKA_BROKERS := kafka-broker-1 kafka-broker-2 kafka-broker-3
BROKER ?= 1

.PHONY: compose-kafka
compose-kafka:
	@echo "Starting Kafka KRaft cluster (3 brokers) + UI + init topics"
	@docker compose up -d $(KAFKA_BROKERS) kafka-ui schema-registry
	@docker compose run --rm kafka-init

.PHONY: compose-kafka-up
compose-kafka-up:
	@echo "Starting Kafka KRaft cluster (3 brokers)"
	@docker compose up -d $(KAFKA_BROKERS)

.PHONY: compose-kafka-broker
compose-kafka-broker:
	@echo "Starting kafka-broker-$(BROKER)"
	@docker compose up -d kafka-broker-$(BROKER)

.PHONY: compose-kafka-init
compose-kafka-init:
	@echo "Creating/ensuring Kafka topics"
	@docker compose run --rm kafka-init

.PHONY: compose-kafka-ui
compose-kafka-ui:
	@echo "Starting Kafka UI on http://localhost:8085"
	@docker compose up -d kafka-ui

.PHONY: compose-kafka-down
compose-kafka-down:
	@echo "Stopping Kafka cluster and UI (volumes preserved)"
	@docker compose stop $(KAFKA_BROKERS) kafka-ui schema-registry kafka-init || true
	@docker compose rm -f $(KAFKA_BROKERS) kafka-ui schema-registry kafka-init || true

.PHONY: compose-kafka-purge
compose-kafka-purge:
	@echo "Purging Kafka cluster (containers + volumes)"
	@docker compose stop $(KAFKA_BROKERS) kafka-ui schema-registry kafka-init || true
	@docker compose rm -f $(KAFKA_BROKERS) kafka-ui schema-registry kafka-init || true
	@docker volume rm -f link-tracker_kafka1-data link-tracker_kafka2-data link-tracker_kafka3-data || true

.PHONY: compose-kafka-logs
compose-kafka-logs:
	@docker compose logs -f $(KAFKA_BROKERS)

.PHONY: compose-kafka-topics
compose-kafka-topics:
	@docker compose exec kafka-broker-1 kafka-topics --bootstrap-server kafka-broker-1:9094 --list

.PHONY: run-gorm-models-generation
run-gorm-models-generation:
	@go run ./internal/scrapper/infrastructure/db/orm/generate

.PHONY: avro-registrate avro-register
avro-registrate:
	@command -v curl >/dev/null 2>&1 || (echo "curl is required" && exit 1)
	@command -v python3 >/dev/null 2>&1 || (echo "python3 is required" && exit 1)
	@echo "Registering Avro schemas in Schema Registry: $(SCHEMA_REGISTRY_URL)"
	@curl -fsS -X POST "$(SCHEMA_REGISTRY_URL)/subjects/link-raw-update-event-value/versions" \
		-H "Content-Type: application/vnd.schemaregistry.v1+json" \
		--data "$$(python3 -c 'import json, pathlib; print(json.dumps({"schema": pathlib.Path("schemas/avro/link_raw_update_event.avsc").read_text()}))')"
	@echo
	@curl -fsS -X POST "$(SCHEMA_REGISTRY_URL)/subjects/link-processed-update-event-value/versions" \
		-H "Content-Type: application/vnd.schemaregistry.v1+json" \
		--data "$$(python3 -c 'import json, pathlib; print(json.dumps({"schema": pathlib.Path("schemas/avro/link_processed_update_event.avsc").read_text()}))')"
	@echo
	@curl -fsS -X POST "$(SCHEMA_REGISTRY_URL)/subjects/failed-links-event-value/versions" \
		-H "Content-Type: application/vnd.schemaregistry.v1+json" \
		--data "$$(python3 -c 'import json, pathlib; print(json.dumps({"schema": pathlib.Path("schemas/avro/failed_links_event.avsc").read_text()}))')"
	@echo
	@echo "Avro schemas registered successfully"

avro-register: avro-registrate

.PHONY: docker-up-all
docker-up-all:
	@echo "\033[36mdocker-up-all:\033[0m Postgres..."
	@$(MAKE) compose-db
	@echo "\033[36mdocker-up-all:\033[0m migrations..."
	@$(MAKE) compose-migrate
	@echo "\033[36mdocker-up-all:\033[0m Redis..."
	@docker compose up -d redis
	@echo "\033[36mdocker-up-all:\033[0m Valkey cluster..."
	@$(MAKE) compose-valkey
	@echo "\033[36mdocker-up-all:\033[0m Kafka + Schema Registry + topics..."
	@$(MAKE) compose-kafka
	@echo "\033[36mdocker-up-all:\033[0m waiting for Schema Registry ($(SCHEMA_REGISTRY_URL))..."
	@i=0; until curl -fsS "$(SCHEMA_REGISTRY_URL)/subjects" >/dev/null 2>&1; do \
		i=$$((i+1)); test $$i -le 120 || (echo "docker-up-all: schema-registry unavailable" && exit 1); \
		sleep 1; \
	done
	@echo "\033[36mdocker-up-all:\033[0m Avro schemas..."
	@$(MAKE) avro-registrate
	@echo "\033[36mdocker-up-all:\033[0m observability..."
	@$(MAKE) compose-observability
	@echo "\033[36mdocker-up-all:\033[0m bot + scrapper + agent containers..."
	@$(MAKE) compose-apps

.PHONY: docker-down-all
docker-down-all:
	@$(MAKE) compose-apps-down
	@docker compose stop prometheus grafana pushgateway redis valkey-cluster db kafka-1 kafka-2 kafka-3 kafka-ui schema-registry || true

VALKEY_SERVICE := valkey-cluster

.PHONY: compose-valkey
compose-valkey:
	@echo "Starting Valkey cluster (valkey/valkey:8-alpine, ports 17000-17002)"
	@docker compose up -d $(VALKEY_SERVICE)

.PHONY: compose-valkey-down
compose-valkey-down:
	@echo "Stopping Valkey cluster"
	@docker compose stop $(VALKEY_SERVICE) || true
	@docker compose rm -f $(VALKEY_SERVICE) || true

.PHONY: compose-valkey-purge
compose-valkey-purge:
	@echo "Purging Valkey cluster"
	@docker compose stop $(VALKEY_SERVICE) || true
	@docker compose rm -f $(VALKEY_SERVICE) || true

SCENARIO ?= unknown
OUT      ?=

.PHONY: loadtest-seed
loadtest-seed:
	@echo "Seeding Postgres (see loadtest/config.yaml)"
	@go run ./loadtest/seed

.PHONY: loadtest-run
loadtest-run:
	@echo "Running load test [scenario=$(SCENARIO)] (see loadtest/config.yaml)"
	@go run ./loadtest/run -scenario "$(SCENARIO)" $(if $(OUT),-out "$(OUT)",)
