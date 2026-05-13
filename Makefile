COVERAGE_FILE ?= coverage.out

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
	@echo "  \033[36mmake compose-app\033[0m - Собрать образ и поднять scrapper + bot (профиль app; нужен APP_TELEGRAM_TOKEN, до этого make compose-migrate)"
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
	@go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest -generate types,chi-server -package api -o internal/bot/transport/http/api/openapi_gen.go api/contracts/bot.yaml
	@go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest -generate types,client     -package botclient -o internal/scrapper/infrastructure/botclient/openapi_client_gen.go api/contracts/bot.yaml
	@go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest -generate types,chi-server -package api -o internal/scrapper/transport/http/api/openapi_gen.go api/contracts/scrapper.yaml
	@go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest -generate types,client     -package scrapperclient -o internal/bot/infrastructure/scrapperclient/openapi_client_gen.go api/contracts/scrapper.yaml
	@echo "API generated"

.PHONY: generate-mocks
generate-mocks:
	@go run go.uber.org/mock/mockgen@latest -destination=internal/bot/application/command_mock.go -package=application gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application Command
	@echo "mocks generated"

.PHONY: run-db-migration
run-db-migration:
	@go run ./migrations/migrator

.PHONY: compose-db
compose-db:
	@docker compose up -d db

.PHONY: compose-app
compose-app:
	@docker compose --profile app up -d --build

.PHONY: run-gorm-models-generation
run-gorm-models-generation:
	@go run ./internal/scrapper/infrastructure/db/orm/generate
