# Release Process

This document describes the versioning strategy, release checklist, and installer release process for TPT Online Video.

## Versioning strategy

TPT Online Video follows [Semantic Versioning 2.0.0](https://semver.org/):

- **MAJOR** — breaking API or data format changes
- **MINOR** — new features (backward compatible)
- **PATCH** — bug fixes and small improvements

Pre-release suffixes are used for beta/RC builds:

```
v1.0.0-beta.1
v1.0.0-rc.1
```

All Go binaries embed the version at build time via `-ldflags`:

```
go build -ldflags="-X main.Version=v1.0.0" ./cmd/tpt-api
```

Frontend versions are tracked in `apps/web/package.json`.

## Release checklist

### Preparation

- [ ] Verify all acceptance criteria for the target milestone are met.
- [ ] Run full test suite: `make test-go && cd apps/web && npm run build`.
- [ ] Verify Docker Compose stack starts cleanly: `docker compose up -d && docker compose logs`.
- [ ] Verify no security warnings from Go dependencies (`go list -u -m all` review).
- [ ] Update `README.md` if feature list or status has changed.
- [ ] Update `docs/architecture.md` if any component boundaries changed.
- [ ] Update `docs/roadmap.md` to reflect completed phases.
- [ ] Verify `.env.example` matches all current configuration keys.
- [ ] Ensure `CHANGELOG.md` at repository root is updated.
- [ ] Tag the release commit with the version number.

### Release

1. Create a signed tag:

   ```bash
   git tag -s v1.0.0 -m "v1.0.0"
   git push origin v1.0.0
   ```

2. Build Docker images:

   ```bash
   docker build -t tpt-api:1.0.0 -f infra/docker/api.Dockerfile .
   docker build -t tpt-worker:1.0.0 -f infra/docker/worker.Dockerfile .
   docker build -t tpt-live:1.0.0 -f infra/docker/live.Dockerfile .
   ```

3. Build binaries for installer targets (see `infra/installer/`).

4. Generate release notes (from `CHANGELOG.md`).

5. Publish release on GitHub or preferred distribution channel.

### Installer checklist

- [ ] Frontend built: `cd apps/web && npm run build`.
- [ ] Binaries compiled for target OS/arch.
- [ ] Installer package version matches release version.
- [ ] Database migrations tested against fresh install.
- [ ] Services start, stop, and restart correctly.
- [ ] Health endpoints respond.
- [ ] Uninstall cleans up all files and services.

### Migration checklist

- [ ] New migrations are backward compatible with the previous version.
- [ ] Migration rollback has been tested.
- [ ] Downgrade instructions documented (if supported).
- [ ] Storage layout changes are additive only.

## Changelog format

```
# Changelog

## [v1.0.0] — 2026-06-11

### Added

- Feature description (PR #NN)

### Changed

- Change description (PR #NN)

### Fixed

- Bug fix description (PR #NN)

### Security

- Security fix description (PR #NN)
```

## Branching model

- `main` — stable development branch, always releasable.
- Feature branches — scoped work merged into `main` via PR.
- Release tags — immutable snapshots.