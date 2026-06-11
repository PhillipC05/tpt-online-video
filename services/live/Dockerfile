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
COPY services/live ./services/live

RUN go build -o /build/tpt-live ./services/live/cmd/tpt-live

FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /build/tpt-live /app/tpt-live

EXPOSE 8081

ENTRYPOINT ["/app/tpt-live"]