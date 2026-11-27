# Nexus — единая точка входа для локальной разработки.
# `make check` — единая проверка кода, одинаковая на машинах всех разработчиков.

BACKEND_DIR          := backend
GOLANGCI_LINT_VERSION := v2.15.0
GOLANGCI_LINT        := $(CURDIR)/$(BACKEND_DIR)/bin/golangci-lint
COMPOSE              ?= docker compose
COMPOSE_FILE         := compose.yaml

.DEFAULT_GOAL := help
.PHONY: help up down run test vet fmt lint check tools

help: ## Показать список целей
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ {printf "  \033[36m%-8s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

up: ## Поднять окружение (docker compose up -d)
	@test -f $(COMPOSE_FILE) || { echo "no compose.yaml file yet"; exit 1; }
	$(COMPOSE) up -d

down: ## Остановить окружение (docker compose down)
	@test -f $(COMPOSE_FILE) || { echo "no compose.yaml file yet"; exit 1; }
	$(COMPOSE) down

run: ## Запустить сервер локально
	cd $(BACKEND_DIR) && go run ./cmd/server

test: ## Тесты с race-детектором
	cd $(BACKEND_DIR) && go test -race -count=1 ./...

vet: ## go vet
	cd $(BACKEND_DIR) && go vet ./...

fmt: tools ## Отформатировать код (gofumpt + goimports)
	cd $(BACKEND_DIR) && "$(GOLANGCI_LINT)" fmt ./...

lint: tools ## golangci-lint
	cd $(BACKEND_DIR) && "$(GOLANGCI_LINT)" run ./...

check: vet lint test ## Всё, что проверяет CI: vet → lint → test -race

tools: ## Установить golangci-lint нужной версии в backend/bin
	@"$(GOLANGCI_LINT)" version 2>/dev/null | grep -q "version $(GOLANGCI_LINT_VERSION:v%=%) " || { \
		echo "installing golangci-lint $(GOLANGCI_LINT_VERSION)"; \
		cd $(BACKEND_DIR) && GOBIN="$(CURDIR)/$(BACKEND_DIR)/bin" go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION); \
	}
