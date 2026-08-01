# syntax=docker/dockerfile:1

# ---------- build ----------
# Only ./cmd/web is compiled, which uses Postgres (lib/pq, pure Go) + Redis —
# no go-sqlite3 in that import graph, so CGO can stay off for a tiny static binary.
# (The SQLite seeder cmd/data/seed.go is NOT built here.)
FROM golang:1.26-alpine AS build
WORKDIR /src
ENV CGO_ENABLED=0 GOOS=linux
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/web ./cmd/web

# ---------- runtime ----------
FROM alpine:3.20
RUN apk add --no-cache ca-certificates openssl
WORKDIR /app
COPY --from=build /out/web /app/web
COPY docker/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh
EXPOSE 8080
# entrypoint generates JWT keys into $KEY_DIR if they are absent, then execs the server
ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["/app/web"]
