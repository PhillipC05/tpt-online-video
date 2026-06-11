# Developer Guide

## Prerequisites

- Go 1.22+
- Node.js 20+
- Docker and Docker Compose
- FFmpeg for local transcoding tests
- OBS Studio for live streaming tests

## Start infrastructure

```bash
docker compose up postgres redis minio mediamtx
```

## Run API

```bash
cp .env.example .env
cd services/api
go run ./cmd/tpt-api
```

Default API URL:

```text
http://localhost:8080
```

Health endpoints:

```text
GET /healthz
GET /readyz
GET /api/v1/ping
```

## Run worker

```bash
cd services/worker
go run ./cmd/tpt-worker
```

## Run live helper

```bash
cd services/live
go run ./cmd/tpt-live
```

## Run frontend

```bash
cd apps/web
npm install
npm run dev
```

Default frontend URL:

```text
http://localhost:5173
```

## Go modules

This repository uses a Go workspace:

```bash
go work sync
go test ./...
```

## Frontend

The frontend is a Vite React TypeScript app:

```bash
cd apps/web
npm install
npm run build
```

## Current foundation

The current scaffold includes:

- Go API with health/readiness endpoints
- Go worker skeleton
- Go live helper skeleton
- PostgreSQL schema
- Redis configuration
- Storage abstraction
- Search abstraction
- Argon2id password hashing foundation
- React frontend health-check page