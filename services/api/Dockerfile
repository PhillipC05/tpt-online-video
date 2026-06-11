# syntax=docker/dockerfile:1
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build
COPY go.work go.work.sum ./
COPY packages/shared ./packages/shared
COPY packages/storage ./packages/storage
COPY packages/search ./packages/search
COPY packages/auth ./packages/auth
COPY packages/media ./packages/media
COPY packages/moderation ./packages/moderation
COPY services/api ./services/api

RUN go build -o /build/tpt-api ./services/api/cmd/tpt-api

FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata ffmpeg ffprobe

WORKDIR /app
COPY --from=builder /build/tpt-api /app/tpt-api

EXPOSE 8080

ENTRYPOINT ["/app/tpt-api"]