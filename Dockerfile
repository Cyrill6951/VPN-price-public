# syntax=docker/dockerfile:1

# ---- build stage ----
FROM golang:1.25-alpine AS build
WORKDIR /src

# Resolve dependencies first (better layer caching). go.sum may be absent on M0,
# so we tidy inside the container with module writes allowed.
ENV GOFLAGS=-mod=mod
COPY go.mod ./
COPY go.sum* ./
RUN go mod download || true

COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/api ./cmd/api

# ---- runtime stage ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates curl && adduser -D -u 10001 app
USER app
COPY --from=build /out/api /usr/local/bin/api
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=3 \
    CMD curl -fsS http://localhost:8080/health || exit 1
ENTRYPOINT ["/usr/local/bin/api"]
