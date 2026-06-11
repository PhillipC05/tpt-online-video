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
COPY services/worker ./services/worker

RUN go build -o /build/tpt-worker ./services/worker/cmd/tpt-worker

FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata ffmpeg ffprobe

WORKDIR /app
COPY --from=builder /build/tpt-worker /app/tpt-worker

ENTRYPOINT ["/app/tpt-worker"]