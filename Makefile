BINARY     := api
CMD        := ./cmd/api
MIGRATIONS := migrations
MODULE     := github.com/fairyhunter13/community-waste-collection-system
DB_URL     ?= $(DATABASE_URL)

.PHONY: all run build clean \
        lint fmt vet \
        test test-unit test-integration test-e2e bench coverage \
        migrate-up migrate-down migrate-force migrate-version migrate-create \
        docker-up docker-down docker-logs docker-clean \
        seed

all: lint test build

run:
	go run $(CMD)

build:
	CGO_ENABLED=0 go build -ldflags="-w -s" -o bin/$(BINARY) $(CMD)

clean:
	rm -rf bin/ coverage.out coverage.html

lint:
	golangci-lint run ./...

fmt:
	goimports -w -local $(MODULE) .

vet:
	go vet ./...

test: test-unit

test-unit:
	go test -race -count=1 -coverprofile=coverage.out \
	    -coverpkg=./internal/... \
	    ./internal/... -v

test-integration:
	go test -race -count=1 -tags=integration \
	    ./internal/repository/... ./internal/service/... \
	    -timeout 120s -v

test-e2e:
	go test -race -count=1 -tags=e2e \
	    ./test/e2e/... \
	    -timeout 180s -v

bench:
	go test -bench=. -benchmem -run='^$$' -tags=integration \
	    ./internal/... \
	    -timeout 120s

coverage:
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

migrate-up:
	migrate -path $(MIGRATIONS) -database "$(DB_URL)" up

migrate-down:
	migrate -path $(MIGRATIONS) -database "$(DB_URL)" down

migrate-force:
	migrate -path $(MIGRATIONS) -database "$(DB_URL)" force $(VERSION)

migrate-version:
	migrate -path $(MIGRATIONS) -database "$(DB_URL)" version

migrate-create:
	migrate create -ext sql -dir $(MIGRATIONS) -seq $(NAME)

docker-up:
	docker compose -f deployments/docker-compose.yml up --build -d

docker-down:
	docker compose -f deployments/docker-compose.yml down

docker-logs:
	docker compose -f deployments/docker-compose.yml logs -f app

docker-clean:
	docker compose -f deployments/docker-compose.yml down -v --remove-orphans

seed:
	psql "$(DB_URL)" -f scripts/seed.sql
