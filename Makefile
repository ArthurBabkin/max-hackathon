# Команды разработки «Траектории». Всё, что проверяет CI, запускается здесь же.
#
# Go — ровно 1.23: максимальный Go-рантайм Yandex Cloud Functions — golang123.
# Более новый тулчейн соберёт модуль (в go.mod стоит go 1.23), но пропустит
# функции стандартной библиотеки из 1.24+, которых в облаке нет. Поэтому,
# если установлен go1.23.12 (go install golang.org/dl/go1.23.12@latest &&
# go1.23.12 download), берём его.
GO ?= $(shell command -v go1.23.12 2>/dev/null || ls $(HOME)/go/bin/go1.23.12 2>/dev/null || echo go)

COMPOSE := docker compose -f infra/docker-compose.yml
# Отдельная база для тестов: харнесс packages/db/dbtest пересоздаёт в ней схему,
# поэтому гонять тесты против базы с демо-данными нельзя.
TEST_DATABASE_URL ?= postgres://traektoria:traektoria@localhost:5432/postgres?sslmode=disable

.PHONY: help fmt fmt-check vet test test-db build check seed up down db-up web-dev bot-poll emu

help: ## список команд
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-12s %s\n", $$1, $$2}'

fmt: ## отформатировать Go-код
	gofmt -w apps packages

fmt-check: ## проверить форматирование (как в CI)
	@out=$$(gofmt -l apps packages); if [ -n "$$out" ]; then echo "не отформатировано:"; echo "$$out"; exit 1; fi

vet: ## go vet, включая stdversion: ловит функции stdlib новее go 1.23
	$(GO) vet ./...

test: ## юнит-тесты без базы (тесты с базой пропускаются)
	$(GO) test ./...

test-db: db-up ## все тесты, включая интеграционные с Postgres
	TEST_DATABASE_URL='$(TEST_DATABASE_URL)' $(GO) test -count=1 ./...
	python3 -m unittest discover -s datasets/parser -p 'test_*.py'

build: ## собрать все четыре бинаря для локального стенда
	@for app in api bot reminders notifier; do \
		$(GO) build -o /dev/null ./apps/$$app/cmd/local || exit 1; \
		$(GO) build -o /dev/null ./apps/$$app/cmd/function || exit 1; \
	done; echo "сборка ок"

check: fmt-check vet build test ## всё, что проверяет CI для Go

seed: ## пересобрать SQL-сид контента из datasets/
	python3 datasets/parser/build_seed.py

db-up: ## поднять только Postgres
	$(COMPOSE) up -d --wait postgres

up: ## весь локальный стенд
	$(COMPOSE) up --build

down: ## остановить стенд (данные остаются; снести — make down ARGS=-v)
	$(COMPOSE) down $(ARGS)

web-dev: ## мини-приложение на живом API (не на моках)
	VITE_USE_MOCKS=off VITE_DEV_FAKE_WEBAPP=true npm --prefix apps/web run dev

bot-poll: db-up ## бот в живом MAX через long polling (только для разработки)
	set -a; . ./.env; set +a; \
	DATABASE_URL="$${DATABASE_URL:-postgres://traektoria:traektoria@localhost:5432/traektoria?sslmode=disable}" \
	BOT_MODE=poll $(GO) run ./apps/bot/cmd/local

emu: ## эмулятор MAX с ботом: чат на http://localhost:9000, без аккаунта в MAX
	GO='$(GO)' apps/bot/cmd/emu/run.sh
