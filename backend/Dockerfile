FROM golang:1.24-bookworm AS builder
RUN apt-get update && apt-get install -y --no-install-recommends gcc libc6-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
# Cache bust: 2026-02-10-v3-salesforce-fix
COPY . .
RUN CGO_ENABLED=1 go build -o api ./cmd/api
RUN CGO_ENABLED=1 go build -o migrate ./cmd/migrate

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    fontconfig \
    fonts-dejavu-core \
    fonts-freefont-ttf \
    wkhtmltopdf \
    && rm -rf /var/lib/apt/lists/*
COPY --from=builder /app/api /app/api
COPY --from=builder /app/migrate /app/migrate
COPY --from=builder /app/static /app/static
WORKDIR /app
EXPOSE 8080
# Run migrations then start API — semicolon ensures API starts even if migrations
# fail (e.g. Turso plan limits). The app handles missing schema gracefully.
CMD /app/migrate; /app/api
