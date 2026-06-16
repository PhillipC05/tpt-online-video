# Testing Guide

This document describes how to run, write, and extend the test suite for TPT Online Video.

---

## Overview

| Layer | Framework | Location | Run command |
|-------|-----------|----------|-------------|
| Go unit tests | `testing` stdlib | `**/*_test.go` | `go test ./...` |
| Go integration tests | `testing` + real DB | `services/api/internal/*/` | `go test ./... -tags integration` |
| Frontend unit/component | Vitest + React Testing Library | `apps/web/src/__tests__/` | `pnpm test` |

---

## Go backend tests

### Running all tests

From the repository root (Go workspace):

```bash
go test ./...
```

To run with verbose output and race detection:

```bash
go test -race -v ./...
```

To run a specific package:

```bash
go test ./services/api/internal/moderation/...
go test ./packages/auth/...
go test ./services/worker/...
```

### Running integration tests

Integration tests require a running PostgreSQL instance. They are tagged with `//go:build integration` and skipped by default.

Start the database:

```bash
docker compose up -d postgres
```

Run integration tests:

```bash
go test -tags integration ./...
```

Set the `DATABASE_URL` environment variable if your local instance uses non-default credentials:

```bash
DATABASE_URL="postgres://tpt:tpt@localhost:5432/tpt_test?sslmode=disable" \
  go test -tags integration ./...
```

### Test coverage

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### What is tested

| Package | Tests cover |
|---------|-------------|
| `packages/auth` | Argon2id hashing, JWT signing/validation, refresh token rotation, reuse detection |
| `packages/media` | Transcoding queue enqueue/dequeue, error scenarios |
| `packages/moderation` | Full moderation workflow (report → assign → resolve → appeal), action reversal |
| `packages/storage` | Local and S3 provider abstraction |
| `packages/search` | PostgreSQL full-text search provider |
| `services/api/internal/live` | DVR window management, stream key generation and hashing |
| `services/worker` | FFmpeg invocation, rendition generation, error handling |

---

## Frontend tests

### Running tests

```bash
cd apps/web
pnpm test          # single run
pnpm test:watch    # watch mode (re-runs on file change)
```

### Test coverage

```bash
cd apps/web
pnpm test --coverage
```

Coverage output goes to `apps/web/coverage/`.

### What is tested

| Test file | Tests cover |
|-----------|-------------|
| `VideoCard.test.tsx` | Video card rendering, thumbnail, duration, title display |
| `EmptyState.test.tsx` | Empty state message variants |
| `auth.test.ts` | Login/register form validation, API client auth flow |
| `api.test.ts` | HTTP client error handling, token injection, refresh logic |
| `comments.test.tsx` | Comment thread rendering, post/delete interactions |
| `search.test.tsx` | Search input, result display, empty results state |
| `LiveChat.test.tsx` | WebSocket connection, message rendering, moderation controls |

### Adding a new component test

Create a file alongside the component or in `apps/web/src/__tests__/`:

```tsx
import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { MyComponent } from '../components/MyComponent'

describe('MyComponent', () => {
  it('renders the title', () => {
    render(<MyComponent title="Hello" />)
    expect(screen.getByText('Hello')).toBeInTheDocument()
  })
})
```

---

## Writing Go tests

### Unit test conventions

- Test files live alongside the code they test (`foo.go` → `foo_test.go`).
- Use table-driven tests for multiple input/output cases.
- Avoid mocking external services in unit tests; use interfaces and pass test doubles.

```go
func TestHashPassword(t *testing.T) {
    cases := []struct {
        name     string
        password string
    }{
        {"short", "abc"},
        {"long", strings.Repeat("x", 72)},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            hash, err := auth.HashPassword(tc.password)
            if err != nil {
                t.Fatalf("unexpected error: %v", err)
            }
            if !auth.CheckPassword(tc.password, hash) {
                t.Fatal("password check failed")
            }
        })
    }
}
```

### Integration test conventions

- Tag with `//go:build integration` at the top of the file.
- Create a fresh schema or use transactions rolled back after each test.
- Use `testenv.MustPostgres(t)` (or equivalent helper) to obtain a database connection.

---

## Test fixtures

Sample media files used in worker and transcoding tests live in `testdata/`:

```
testdata/
  sample.mp4       # short H.264/AAC clip for transcoding tests
  sample.jpg       # thumbnail fixture
```

Do not commit large binary fixtures. Keep sample clips under 1 MB.

---

## CI / pre-merge requirements

All pull requests must pass:

1. `go test ./...` — zero failures, zero race conditions
2. `pnpm test` in `apps/web` — zero failures
3. `go vet ./...` — zero warnings
4. `eslint` — zero errors (run via `pnpm lint` in `apps/web`)

End-to-end tests and testcontainers-based integration tests are planned but not yet wired into CI.

---

## Planned additions

- **End-to-end tests** — Playwright driving a full Docker Compose stack (see TODO Phase 26)
- **Testcontainers** — spin up ephemeral PostgreSQL and Redis containers per test run, removing the need for a pre-running database
- **Load tests** — k6 scripts for upload, playback, and live chat throughput
