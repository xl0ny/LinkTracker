COVERAGE_FILE ?= coverage.out
SCHEMA_REGISTRY_URL ?= http://localhost:8081

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
	@echo "  \033[36mmake run-all\033[0m - Run bot and scrapper together (Ctrl+C stops both)"
	@echo "  \033[36mmake compose-db\033[0m - Postgres via Docker Compose (локальная разработка по ДЗ)"
	@echo "  \033[36mmake compose-migrate\033[0m - Применить SQL-миграции к compose-Postgres (отдельный шаг по ДЗ)"
	@echo "  \033[36mmake avro-registrate\033[0m - Register Avro schemas in Schema Registry"
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
	@go test -tags=integration -count=1 -v ./internal/scrapper/infrastructure/db/...

.PHONY: lint
lint:
	@command -v $(GOLANGCI_LINT) >/dev/null 2>&1 || (echo "golangci-lint v2 not found. Run: go install github.com/golangci/golangci-lint/v2@latest" && exit 1)
	@$(GOLANGCI_LINT) run -c .golangci.yml --timeout=5m

.PHONY: run-bot
run-bot:
	@echo "Running bot"
	@go run ./cmd/bot/main.go

.PHONY: run-scrapper
run-scrapper:
	@echo "Running scrapper"
	@go run ./cmd/scrapper/main.go

.PHONY: run-all
run-all:
	@echo "Running bot and scrapper (Ctrl+C stops both)"
	@trap 'kill $$BOT $$SCRAPPER 2>/dev/null; exit 130' INT TERM; \
	go run ./cmd/bot/main.go & BOT=$$!; \
	go run ./cmd/scrapper/main.go & SCRAPPER=$$!; \
	wait $$BOT $$SCRAPPER

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

.PHONY: compose-db
compose-db:
	@docker compose up -d db

.PHONY: compose-migrate
compose-migrate:
	@docker compose --profile migrate run --rm migrator

.PHONY: run-gorm-models-generation
run-gorm-models-generation:
	@go run ./internal/scrapper/infrastructure/db/orm/generate

.PHONY: avro-registrate avro-register
avro-registrate:
	@command -v curl >/dev/null 2>&1 || (echo "curl is required" && exit 1)
	@command -v python3 >/dev/null 2>&1 || (echo "python3 is required" && exit 1)
	@echo "Registering Avro schemas in Schema Registry: $(SCHEMA_REGISTRY_URL)"
	@curl -fsS -X POST "$(SCHEMA_REGISTRY_URL)/subjects/link-update-event-value/versions" \
		-H "Content-Type: application/vnd.schemaregistry.v1+json" \
		--data "$$(python3 -c 'import json, pathlib; print(json.dumps({"schema": pathlib.Path("schemas/avro/link_update_event.avsc").read_text()}))')"
	@echo
	@curl -fsS -X POST "$(SCHEMA_REGISTRY_URL)/subjects/failed-links-event-value/versions" \
		-H "Content-Type: application/vnd.schemaregistry.v1+json" \
		--data "$$(python3 -c 'import json, pathlib; print(json.dumps({"schema": pathlib.Path("schemas/avro/failed_links_event.avsc").read_text()}))')"
	@echo
	@echo "Avro schemas registered successfully"

avro-register: avro-registrate