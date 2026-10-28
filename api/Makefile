-include .env
export

MIGRATIONS_PATH=db/migrations
DB_URL=postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSLMODE)

.PHONY: migrate-create migrate-up migrate-down build dev docker-dev swagger swagger-v3 test test-ngrok test-e2e test-stripe test-all seed-stripe seed-stripe-docker

migrate-create:
	migrate create -ext sql -dir $(MIGRATIONS_PATH) -seq $(name)

migrate-up:
	migrate -path $(MIGRATIONS_PATH) -database "$(DB_URL)" up

migrate-down:
	migrate -path $(MIGRATIONS_PATH) -database "$(DB_URL)" down 1

test:
	go test $$(go list ./... | grep -v /e2e)

test-ngrok:
	go test -v -run TestDetectNgrokURL ./cmd/api/

test-e2e:
	docker compose -f docker-compose.test.yml up -d --wait
	go test ./internal/e2e/ -v -count=1 -timeout 120s; \
	EXIT_CODE=$$?; \
	docker compose -f docker-compose.test.yml down -v; \
	exit $$EXIT_CODE

test-stripe:
	@./scripts/test-stripe-e2e.sh

test-all: test test-e2e

build:
	go build -o bin/api ./cmd/api

dev:
	go run cmd/api/main.go

docker-dev:
	docker compose -f docker-compose.dev.yml up --build --remove-orphans
SWAG=$(shell go env GOPATH)/bin/swag
# swag v2 emits the OpenAPI 3.1 spec (the `--v3.1` flag does not exist in
# swag v1, which is what $(SWAG) / `make swagger` uses to embed the legacy
# Swagger 2.0 doc into the binary via gin-swagger). Pinned so CI and local
# generation stay byte-stable. Invoked via `go run …@version` so it never
# clobbers the v1 binary at $(SWAG).
SWAG_V3_VERSION=v2.0.0-rc5

swagger:
	$(SWAG) init -g cmd/api/main.go -o docs --parseDependency

# US-194: generate the published OpenAPI 3.1 spec served by docs.replider.com
# (Redoc). Output lands under nginx/docs/openapi/v1/ because nginx/ is the
# only tree shipped to the VPS (api/docs/ is gitignored and lives only inside
# the API image). The generated docs.go is removed — we publish the JSON/YAML
# spec as static data, so no swag/v2 module dependency is pulled into the build.
swagger-v3:
	go run github.com/swaggo/swag/v2/cmd/swag@$(SWAG_V3_VERSION) init -g cmd/api/main.go -o ../nginx/docs/openapi/v1 --parseDependency --v3.1
	rm -f ../nginx/docs/openapi/v1/docs.go

# Creates Stripe Products + monthly Prices for any plan whose
# stripe_price_id is still empty, then UPDATE-s the row with the
# newly-minted price_id. Idempotent — skips plans that already have
# a price. Uses STRIPE_SECRET_KEY + DB_* env vars from .env.
seed-stripe:
	go run ./cmd/seed_stripe_prices

# Same as above but runs inside the dev API container so it picks up
# the container's env (DB_HOST=replider-db resolvable, etc.) without
# needing to duplicate those on the host.
seed-stripe-docker:
	docker exec replider_api_dev go run ./cmd/seed_stripe_prices
