# syntax=docker/dockerfile:1
# The public demo: the server with the web app embedded, serving the frozen
# Nifty 50 snapshot read-only. Build the snapshot first: make demo-snapshot.
#
#   docker build -t kanso .
#   docker run --rm -p 8080:8080 kanso

# 1. The web app.
FROM node:26.13.1-slim AS web
RUN npm install -g pnpm@12.14.0
WORKDIR /src/web
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

# 2. The Go binary, static (no cgo), with the web app embedded.
FROM golang:1.27.4 AS server
ENV CGO_ENABLED=0
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
COPY --from=web /src/web/dist/ internal/webui/dist/
RUN go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# 3. Runtime: distroless static, non-root, nothing writable needed.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=server /out/server /app/server
COPY deploy/demo.db /app/demo.db
USER nonroot:nonroot
# Cloud Run sets PORT; the server expands it (there is no shell).
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/app/server", "-db", "/app/demo.db", "-demo", "-addr", ":${PORT}"]
