# Dependency Management

## Go Workspace

The Go backend uses a Go workspace (`go.work`) to manage multi-module development:

```text
go.work ──► packages/shared
         ├── packages/storage
         ├── packages/search
         ├── packages/auth
         ├── packages/media
         ├── packages/moderation
         ├── services/api
         ├── services/worker
         └── services/live
```

**Commands:**
- `go work sync` — sync all module dependencies
- `make tidy-go` — tidy all Go modules and sync workspace
- `make test-go` — run all Go tests

## Frontend Workspace (pnpm)

The frontend uses pnpm workspaces for monorepo dependency management:

```text
pnpm-workspace.yaml ──► apps/*
                     └── packages/*
```

**Commands:**
- `pnpm install` — install all workspace dependencies
- `pnpm dev` — run frontend dev server
- `pnpm build` — build frontend for production
- `pnpm typecheck` — TypeScript type check across all TS packages
- `pnpm lint` — ESLint across all TS packages
- `pnpm format` — Prettier format across all TS packages

**Package naming convention:**
- `@tpt/web` — frontend app (`apps/web`)
- `@tpt/types` — shared TypeScript types (`packages/types`)
- `@tpt/tsconfig` — shared TS config presets (`packages/tsconfig`)

## Adding New Dependencies

### Go
```bash
cd services/api && go get github.com/example/pkg
make tidy-go
```

### Frontend
```bash
pnpm --filter @tpt/web add some-package
# or for dev dependencies:
pnpm --filter @tpt/web add -D some-dev-package
```

## Version Policy

- Go: 1.22 (defined in `go.work` and each `go.mod`)
- Node: >= 18 (defined in root `package.json` engines)
- pnpm: >= 8 (uses `pnpm-lock.yaml` lockfile version 6)