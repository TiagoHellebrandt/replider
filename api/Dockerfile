# ── Stage 1: Builder ──────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates tzdata

# Install golang-migrate CLI early so this layer is cached independently
# of source-code changes.
RUN CGO_ENABLED=0 GOOS=linux \
    go install -tags 'postgres' \
    github.com/golang-migrate/migrate/v4/cmd/migrate@v4.18.1

# Download dependencies (cached layer — only re-runs if go.mod/go.sum change)
COPY go.mod go.sum ./
RUN go mod download

# Build the API binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o /app/replider-api ./cmd/api

# ── Stage 2: Production image ─────────────────────────────────────────────────
FROM alpine:3.20

WORKDIR /app

# ffmpeg is used at runtime to transcode voice notes recorded in the
# browser (AAC/m4a) into OGG-Opus before forwarding to Telegram, so
# the receiver gets a native voice-note bubble with waveform instead
# of a music-player bubble.
RUN apk add --no-cache ffmpeg

# TLS certs and timezone data
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo                 /usr/share/zoneinfo

# Create a non-root user
RUN adduser -D -u 1000 replider && \
    chown -R replider:replider /app

# Copy the API binary
COPY --from=builder --chown=replider:replider /app/replider-api .

# Copy the migrate CLI (runs migrations on startup)
COPY --from=builder --chown=replider:replider /go/bin/migrate /usr/local/bin/migrate

# Copy migration files so they're available inside the container
COPY --chown=replider:replider db/migrations ./db/migrations/

# Copy and set up the production entrypoint
COPY --chown=replider:replider entrypoint.prod.sh .
RUN chmod +x entrypoint.prod.sh

USER replider

EXPOSE 8080

ENTRYPOINT ["./entrypoint.prod.sh"]
