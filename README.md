# Oklavier

**Website**: [oklavier.com](https://oklavier.com) · **Docs**: [docs.oklavier.com](https://docs.oklavier.com)

[![CI](https://github.com/enzoamate/oklavier/actions/workflows/ci.yml/badge.svg)](https://github.com/enzoamate/oklavier/actions/workflows/ci.yml)
[![Release](https://github.com/enzoamate/oklavier/actions/workflows/release.yml/badge.svg)](https://github.com/enzoamate/oklavier/actions/workflows/release.yml)
[![codecov](https://codecov.io/gh/enzoamate/oklavier/branch/main/graph/badge.svg)](https://codecov.io/gh/enzoamate/oklavier)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![GHCR](https://img.shields.io/badge/ghcr-oklavier-blue?logo=docker)](https://github.com/enzoamate?tab=packages&repo_name=oklavier)
[![Trivy scanned](https://img.shields.io/badge/trivy-scanned-success?logo=aqua)](https://github.com/enzoamate/oklavier/security/code-scanning)

Self-hosted virtual workspace platform on Kubernetes. Stream desktop sessions to a browser via Apache Guacamole.

## Architecture

| Component | Path | Stack |
|---|---|---|
| **Core API** | `oklavier-core/backend` | Go 1.25, Fiber, PostgreSQL, JWT, k8s client-go |
| **Core Frontend** | `oklavier-core/frontend` | Next.js 16, React 19, i18next |
| **Agent** | `oklavier-agent/backend` | Go 1.25, Fiber, Guacamole, k8s provisioner |
| **Helm charts** | `*/helm` | Control plane + per-cluster agent |

The **core** runs once (control plane: API + frontend + Postgres + Valkey). One **agent** runs per Kubernetes cluster you want to provision workspaces in. Agents register with the core via a token and create workspace pods on demand.

## Quick start

### 1. Install the control plane

From the OCI registry (recommended):

```bash
helm install oklavier oci://ghcr.io/enzoamate/charts/oklavier-core --version 1.0.2 \
  --namespace oklavier --create-namespace \
  --set jwtSecret="$(openssl rand -hex 32)" \
  --set internalSecret="$(openssl rand -hex 32)" \
  --set database.password="$(openssl rand -base64 24 | tr -d '+/=')" \
  --set admin.password="$(openssl rand -base64 18 | tr -d '+/=')"
```

Or from a local checkout:

```bash
helm install oklavier ./oklavier-core/helm \
  --namespace oklavier --create-namespace \
  --set jwtSecret="$(openssl rand -hex 32)" \
  --set internalSecret="$(openssl rand -hex 32)" \
  --set database.password="$(openssl rand -base64 24 | tr -d '+/=')" \
  --set admin.password="$(openssl rand -base64 18 | tr -d '+/=')"
```

Requires the [CloudNativePG operator](https://cloudnative-pg.io/) for the database. Expose the `oklavier-front` service via your ingress controller.

### 2. Register an agent

Log into the control plane as admin → **Admin → Agents → New** → copy the generated token.

### 3. Install an agent

```bash
helm install oklavier-agent oci://ghcr.io/enzoamate/charts/oklavier-agent --version 1.0.2 \
  --namespace oklavier-agent --create-namespace \
  --set agent.name="cluster-1" \
  --set agent.token="<token-from-step-2>" \
  --set agent.controlPlane="https://oklavier.example.com" \
  --set agent.publicURL="https://agent-1.example.com" \
  --set agent.jwtSecret="<same-as-core-jwtSecret>"
```

The agent ships its own `guacd` and provisions workspace pods in its namespace.

## Build

Multi-stage Docker builds, no prebuilt binaries committed.

```bash
docker build -t oklavier-api      oklavier-core/backend
docker build -t oklavier-frontend oklavier-core/frontend
docker build -t oklavier-agent    oklavier-agent/backend
```

## CI/CD

| Trigger | Workflow | Effect |
|---|---|---|
| PR / push branch | [`ci.yml`](.github/workflows/ci.yml) | Lint (Go, TS, Helm) + tests + Docker build (no push) |
| Push to `main` | [`release.yml`](.github/workflows/release.yml) | Push `ghcr.io/<owner>/oklavier-*:main-<sha>` and `:latest` |
| Tag `v*.*.*` | [`release.yml`](.github/workflows/release.yml) | Push `:vX.Y.Z` + `:X.Y` + `:X` + `:latest` + GitHub Release |
| Conventional commit on main | [`release-please.yml`](.github/workflows/release-please.yml) | Auto-opens release PR with version bump + CHANGELOG |
| Manual | [`build.yml`](.github/workflows/build.yml) | Ad-hoc dispatch with target/version inputs |
| Weekly | [`dependabot.yml`](.github/dependabot.yml) | Auto-PR for Go / npm / actions / Docker base images |

**Branch model**: trunk-based. `main` is always deployable, features go in PRs, releases are tags. See [CONTRIBUTING.md](CONTRIBUTING.md).

## Roadmap

Current and planned features live in [ROADMAP.md](ROADMAP.md). Open a [discussion](https://github.com/enzoamate/oklavier/discussions) to propose, debate, or claim an item.

## Configuration

All secrets are required via env / Helm `values.yaml`. No defaults are committed.

| Var | Component | Notes |
|---|---|---|
| `JWT_SECRET` | core, agent | Must be **identical** on both for session tokens |
| `INTERNAL_SECRET` | core | Next.js ↔ Go API auth |
| `DB_PASSWORD` | core | Required, no default |
| `OKLAVIER_AGENT_TOKEN` | agent | Per-agent registration token |
| `OKLAVIER_CONTROL_PLANE` | agent | Public URL of the core |
| `OKLAVIER_PUBLIC_URL` | agent | Public URL the agent is reachable at (for WebRTC) |

## License

Apache 2.0 — see [LICENSE](LICENSE). Free for commercial and personal use, with patent grant.
