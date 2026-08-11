# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.26.5

FROM golang:${GO_VERSION}-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/dahua-attendance-backend \
    ./cmd/server

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app \
    && adduser -S -G app app

WORKDIR /app

COPY --from=build /out/dahua-attendance-backend /app/dahua-attendance-backend
COPY configs /app/configs

EXPOSE 8080

USER app

ENTRYPOINT ["/app/dahua-attendance-backend"]
CMD ["-config", "/app/configs/config.docker.toml"]
