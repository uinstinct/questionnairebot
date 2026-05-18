# syntax=docker/dockerfile:1

# ---- Builder ----
FROM golang:1.22-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/bot ./cmd/bot

# ---- Runtime ----
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S botuser \
    && adduser -S -G botuser -u 1000 botuser \
    && mkdir -p /app/data \
    && chown -R botuser:botuser /app

WORKDIR /app
COPY --from=builder /out/bot /app/bot
RUN chown botuser:botuser /app/bot && chmod 0755 /app/bot

USER botuser
ENV DATA_DIR=/app/data

ENTRYPOINT ["/app/bot"]
