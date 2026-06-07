BINARY     := api
CMD        := ./cmd/api
MIGRATIONS := migrations
MODULE     := github.com/fairyhunter13/community-waste-collection-system
DB_URL     ?= $(DATABASE_URL)
BASE_URL   ?= http://localhost:8080
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
LDFLAGS    := -w -s \
              -X main.version=$(VERSION) \
              -X main.commit=$(COMMIT) \
              -X main.buildDate=$(BUILD_DATE)

.DEFAULT_GOAL := help

.PHONY: help all run build clean \
        lint fmt vet mocks \
        test test-unit test-integration test-e2e bench perf coverage \
        load load-average \
        migrate-up migrate-down migrate-force migrate-version migrate-create \
        docker-up docker-down docker-logs docker-clean \
        seed compliance-check

## help: show this help (default target)
help:
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} \
	     /^## [a-zA-Z_-]+:/ { gsub("## ", ""); split($$0, a, ":"); printf "  \033[36m%-20s\033[0m %s\n", a[1], substr($$0, index($$0, ":") + 1) }' \
	     $(MAKEFILE_LIST)

## all: run lint, tests, build
all: lint test build

## run: run the API locally with `go run`
run:
	go run $(CMD)

## build: produce ./bin/api with version metadata embedded
build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o bin/$(BINARY) $(CMD)

## mocks: regenerate mockery mocks for domain interfaces
mocks:
	go generate ./...

## clean: remove build artefacts and coverage profiles
clean:
	rm -rf bin/ coverage.out coverage-integration.out coverage.html

## lint: run golangci-lint v2.12.2
lint:
	golangci-lint run ./...

## fmt: run goimports
fmt:
	goimports -w -local $(MODULE) .

## vet: go vet
vet:
	go vet ./...

## test: alias for test-unit
test: test-unit

## test-unit: race-enabled unit tests with coverage profile
test-unit:
	go test -race -count=1 -coverprofile=coverage.out \
	    -coverpkg=$$(go list ./internal/... | grep -Ev '/mocks$$|/repository$$|/observability$$' | paste -sd, -) \
	    ./internal/... -v

## test-integration: testcontainers-backed integration tests
test-integration:
	go test -race -count=1 -tags=integration \
	    -coverprofile=coverage-integration.out \
	    -coverpkg=$$(go list ./internal/... | grep -Ev '/mocks$$|/observability$$' | paste -sd, -) \
	    ./internal/repository/... ./internal/service/... \
	    -timeout 120s -v

## test-e2e: full-stack E2E tests against docker-compose
test-e2e:
	go test -race -count=1 -tags=e2e \
	    ./test/e2e/... \
	    -timeout 180s -v

## bench: micro-benchmarks (integration tag)
bench:
	go test -race -bench=. -benchmem -run='^$$' -tags=integration \
	    ./internal/... \
	    -timeout 120s

## perf: full-stack HTTP perf tests (perf tag)
perf:
	go test -race -bench=. -benchmem -run='^$$' -tags=perf \
	    ./test/perf/... \
	    -timeout 300s

.PHONY: load
## load: Run k6 smoke load test against the running stack
load:
	k6 run test/load/smoke.js --env BASE_URL=$(BASE_URL)

.PHONY: load-average
## load-average: Run k6 average-load test against the running stack
load-average:
	k6 run test/load/average.js --env BASE_URL=$(BASE_URL)

## coverage: render coverage.out as HTML
coverage:
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## coverage-all: run unit + integration with merged profile
coverage-all:
	go test -race -count=1 -tags=integration \
	    -coverprofile=coverage.out \
	    -coverpkg=./internal/... \
	    ./internal/... -timeout 120s
	go tool cover -func=coverage.out | tail -1

## migrate-up: apply all pending migrations
migrate-up:
	migrate -path $(MIGRATIONS) -database "$(DB_URL)" up

## migrate-down: roll back the last migration
migrate-down:
	migrate -path $(MIGRATIONS) -database "$(DB_URL)" down

## migrate-force: force migration state to VERSION
migrate-force:
	migrate -path $(MIGRATIONS) -database "$(DB_URL)" force $(VERSION)

## migrate-version: print current migration version
migrate-version:
	migrate -path $(MIGRATIONS) -database "$(DB_URL)" version

## migrate-create: scaffold a new migration pair (NAME=…)
migrate-create:
	migrate create -ext sql -dir $(MIGRATIONS) -seq $(NAME)

## docker-up: boot the full docker-compose stack (-d)
docker-up:
	docker compose -f deployments/docker-compose.yml up --build -d

## docker-down: stop the docker-compose stack
docker-down:
	docker compose -f deployments/docker-compose.yml down

## docker-logs: tail app container logs
docker-logs:
	docker compose -f deployments/docker-compose.yml logs -f app

## docker-clean: stop + remove volumes + orphans
docker-clean:
	docker compose -f deployments/docker-compose.yml down -v --remove-orphans

## seed: load scripts/seed.sql into the configured DB
seed:
	psql "$(DB_URL)" -f scripts/seed.sql

.PHONY: compliance-check
## compliance-check: Run compliance check for forbidden identifiers
compliance-check:
	@bash scripts/compliance_check.sh
