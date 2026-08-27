# syntax=docker/dockerfile:1.7

FROM golang:1.27.0-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY cmd/server ./cmd/server
COPY internal ./internal

ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" \
    go build -buildvcs=false -trimpath -ldflags="-s -w" \
    -o /out/termchat-server ./cmd/server

FROM alpine:3.23

RUN apk add --no-cache ca-certificates \
    && addgroup -S termchat \
    && adduser -S -D -H -h /nonexistent -s /sbin/nologin -G termchat termchat

COPY --from=build --chown=termchat:termchat \
    /out/termchat-server /usr/local/bin/termchat-server

USER termchat
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null || exit 1

ENTRYPOINT ["/usr/local/bin/termchat-server"]
