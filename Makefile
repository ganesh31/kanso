.PHONY: test test-web lint sqlc api check-api benchmark web e2e demo-snapshot build embed-web run

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

# Build the web app, embed it (internal/webui/dist) and build the Go binaries into bin/.
build: web embed-web
	go build -trimpath -o bin/ ./cmd/...

# Copy the built web app into the Go embed directory, keeping its placeholder.
embed-web:
	find internal/webui/dist -mindepth 1 ! -name .gitkeep -delete
	cp -R web/dist/. internal/webui/dist/

# Serve the API and the embedded web app on 127.0.0.1:8080 from an ingested database: make run DB=kanso.db
run: build
	./bin/server -db $(DB)

# Regenerate internal/data/db from the SQL (sqlc v1.31.1:
# CGO_ENABLED=1 go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1).
sqlc:
	sqlc generate -f internal/data/sqlc.yaml

# Ingest the demo's frozen Nifty 50 (deploy/nifty50.csv) plus ^NSEI from Yahoo
# into deploy/demo.db, then vacuum it into one rollback-journal file for the image.
demo-snapshot:
	rm -f deploy/demo.db deploy/demo.db-wal deploy/demo.db-shm
	go run ./cmd/ingest -db deploy/demo.db -universe NIFTY50 -constituents deploy/nifty50.csv -compact

benchmark e2e:
	@echo "make $@: not implemented yet (later V1 ticket)"; exit 1
