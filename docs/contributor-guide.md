# Contributor Guide

Thank you for your interest in contributing to TPT Online Video!

## Contribution workflow

1. Fork the repository.
2. Create a feature branch from `main`.
3. Make your changes.
4. Run tests and linting.
5. Submit a pull request against `main`.

## Code style

### Go

- Follow [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments).
- Use `gofmt` (run `make lint`).
- Keep packages focused and well-named.
- Error strings should not be capitalized or end with punctuation.
- Use structured logging (zerolog) rather than `fmt.Print*`.
- Context should be passed as the first parameter to interface methods.
- Prefer explicit dependency injection over global state.

### TypeScript / React

- Follow the TypeScript strict configuration.
- Prefer functional components with hooks.
- Use async/await over raw promises.
- Name files with PascalCase for components, camelCase for utilities.
- Use the custom Tailwind-compatible CSS variables defined in `styles.css`.

### Documentation

- All new features must include documentation updates.
- Use Markdown for documentation files.
- Keep diagrams as ASCII art or Mermaid in `.md` files.

## Commit style

- Use conventional commits: `feat:`, `fix:`, `docs:`, `refactor:`, `chore:`, `test:`, `ci:`.
- Keep commits atomic and well-scoped.
- Use the imperative mood ("Add feature" not "Added feature").
- Reference issues or discussions when applicable.

Example:

```
feat(upload): implement resumable chunk upload

Adds chunk upload support with session-based tracking.
Implements completion detection and transcode job creation.

Refs #42
```

## Testing expectations

- All new Go code should have unit tests.
- Bug fixes should include a test that reproduces the bug.
- Integration tests belong in `services/*/internal/` test files.
- Frontend tests are valued but not strictly required for initial contributions.
- CI will run `go test ./...` and `npm run build`.

## Documentation expectations

- Every new API endpoint must be documented in the relevant docs file.
- Every new configuration option must be added to `.env.example`.
- Changes to the Docker Compose setup must be documented.
- Breaking changes must be called out in the PR description.

## Pull request checklist

Before submitting a PR:

- [ ] Code compiles without errors
- [ ] Tests pass (`make test-go`, `npm run build`)
- [ ] Lint passes (`make lint`)
- [ ] Documentation updated
- [ ] `.env.example` updated if new config added
- [ ] Commit messages follow conventional commit style
- [ ] PR description explains the change and motivation