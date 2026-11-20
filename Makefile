.PHONY: test test-web lint sqlc api check-api benchmark web e2e demo-snapshot build run

export CGO_ENABLED = 0

# Go tests (no Python, network or database needed).
test:
	go test ./...

# Web unit tests (Vitest).
test-web:
	cd web && pnpm exec vitest run

lint:
	golangci-lint run
	test -z "$$(gofumpt -l .)"

# Regenerate the OpenAPI spec from the Go handlers and the TypeScript schema from it.
api:
	go run ./cmd/server -dump-openapi > api/openapi.json
	pnpm --dir web run gen:api

# Fails if the committed spec or TS schema is stale.
check-api: api
	git diff --exit-code api/openapi.json web/src/api/schema.d.ts

web:
	cd web && pnpm install --frozen-lockfile && pnpm build

build:
	go build -trimpath -o bin/server ./cmd/server

# Serve the API and built web app on :8080 from a CSV of daily bars: make run CSV=bars.csv
run: build
	./bin/server -csv $(CSV)

sqlc benchmark e2e demo-snapshot:
	@echo "make $@: not implemented yet (later V1 ticket)"; exit 1
