.PHONY: test test-web lint sqlc api check-api benchmark web e2e demo-snapshot build run

export CGO_ENABLED = 0

# Go tests (no Python, network or database needed).
test:
	go test ./...

# Web unit tests (Vitest).
test-web:
	cd web && pnpm exec vp test

lint:
	golangci-lint run
	test -z "$$(gofumpt -l .)"
	cd web && pnpm exec vp check

# Regenerate the OpenAPI spec from the Go handlers and the TypeScript client (orval) from it.
api:
	go run ./cmd/server -dump-openapi > api/openapi.json
	pnpm --dir web run gen:api

# Fails if the committed spec or generated TS client is stale.
check-api: api
	git diff --exit-code api/openapi.json web/src/api/generated.ts

web:
	cd web && pnpm install --frozen-lockfile && pnpm build

build:
	go build -trimpath -o bin/server ./cmd/server

# Serve the API and built web app on :8080 from a CSV of daily bars: make run CSV=bars.csv
run: build
	./bin/server -csv $(CSV)

# Regenerate internal/data/db from the SQL (sqlc v1.31.1:
# CGO_ENABLED=1 go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1).
sqlc:
	sqlc generate -f internal/data/sqlc.yaml

benchmark e2e demo-snapshot:
	@echo "make $@: not implemented yet (later V1 ticket)"; exit 1
