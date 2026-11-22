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

# Regenerate the vectorbt oracle's fixture and golden files (spec §8) from
# an ingested database that has every BENCH_SYMBOLS member:
# make benchmark KANSO_DB=kanso.db. Needs uv; the Go tests never do.
BENCH_SYMBOLS = RELIANCE,TCS,HDFCBANK,INFY,ICICIBANK,ITC,LT,SBIN,BHARTIARTL,HINDUNILVR,DIVISLAB,BAJFINANCE,HEROMOTOCO,^NSEI
BENCH_TO = 2026-11-20
benchmark:
	@test -n "$(KANSO_DB)" || { echo "make benchmark: set KANSO_DB=path/to/kanso.db"; exit 1; }
	go run ./cmd/export-fixture -db $(KANSO_DB) -symbols '$(BENCH_SYMBOLS)' -from 2005-01-01 -to $(BENCH_TO) -out testdata/benchmark/bars.parquet
	uv run --project tools/vbt-benchmark python -I tools/vbt-benchmark/benchmark.py testdata/benchmark/bars.parquet testdata/benchmark/systems testdata/benchmark

# Ingest the demo's frozen Nifty 50 (deploy/nifty50.csv) plus ^NSEI from Yahoo
# into deploy/demo.db, then vacuum it into one rollback-journal file for the image.
demo-snapshot:
	rm -f deploy/demo.db deploy/demo.db-wal deploy/demo.db-shm
	go run ./cmd/ingest -db deploy/demo.db -universe NIFTY50 -constituents deploy/nifty50.csv -compact

e2e:
	@echo "make $@: not implemented yet (later V1 ticket)"; exit 1
