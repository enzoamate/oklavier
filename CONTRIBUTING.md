# Contributing to Oklavier

## Branch model

Trunk-based. `main` is always deployable.

- **Feature work** — branch from `main`, name `feature/<short-desc>` or `fix/<short-desc>`
- **Pull requests** target `main`, require CI green
- **Releases** are git tags `vX.Y.Z` on `main` (created automatically by release-please)

No `develop`, no `release/*`, no GitFlow.

## Commit messages — Conventional Commits

Drives automatic versioning via [release-please](https://github.com/googleapis/release-please).

| Prefix | Bump | Example |
|---|---|---|
| `feat:` | minor (1.2.0 → 1.3.0) | `feat(agent): add WebRTC fallback` |
| `fix:` | patch (1.2.0 → 1.2.1) | `fix(api): rate-limit refresh races` |
| `feat!:` or footer `BREAKING CHANGE:` | major (1.2.0 → 2.0.0) | `feat(core)!: drop session cookies` |
| `chore:`, `docs:`, `ci:`, `refactor:`, `test:`, `perf:` | no release | `chore: bump go to 1.25.1` |

Scope (`(agent)`, `(core)`, `(api)`, `(frontend)`, `(helm)`) is optional but encouraged.

## Release flow

1. Merge PRs into `main` with conventional commits.
2. **release-please** opens (and keeps updated) a "release PR" titled `chore(main): release vX.Y.Z` with the version bump and CHANGELOG.
3. Merge the release PR. release-please creates the git tag.
4. The tag triggers `release.yml`, which:
   - Builds and pushes `ghcr.io/<owner>/oklavier-{api,frontend,agent}:vX.Y.Z` + `:latest` + `:X.Y` + `:X`
   - Creates a GitHub Release with auto-generated notes

Manual ad-hoc builds are still possible via `Actions → Build & push images → Run workflow`.

## Local dev

```bash
# Backend (per component)
cd oklavier-core/backend && go run ./cmd/server
cd oklavier-agent/backend && go run ./cmd/agent

# Frontend
cd oklavier-core/frontend && npm install && npm run dev

# Helm dry-run
helm template test ./oklavier-core/helm \
  --set jwtSecret=test --set internalSecret=test \
  --set database.password=test --set admin.password=test
```

## Environment variables

All secrets are required. The API will fail fast on missing `JWT_SECRET` or `DB_PASSWORD`. See [README.md](README.md#configuration).

## Quality gates (enforced by `ci.yml`)

- Go: `go vet`, `gofmt`, `go build`, `go test -race`
- Frontend: `npm run lint`, `tsc --noEmit`, `next build`
- Helm: `helm lint`, `helm template`
- Docker: build all three images on every PR

CI must pass before merge.
