# Roadmap

> Living document — priorities shift with feedback.
> Open a [discussion](https://github.com/enzoamate/oklavier/discussions) or [issue](https://github.com/enzoamate/oklavier/issues) to propose, debate, or claim an item.

Legend: ✅ shipped — 🚧 in progress — 📅 planned — 💭 idea (no commitment)

---

## ✅ Shipped — v1.0 (current)

The control plane and per-cluster agent are usable end-to-end.

### Core
- [x] OIDC + local auth, JWT-based sessions with refresh + blacklist
- [x] Admin UI: users, agents, workspaces, storage, branding
- [x] Multi-cluster — one agent per cluster, registered with a token
- [x] Internal Next.js ↔ Go API auth (`internalSecret`), CSRF, rate limiting (Valkey)
- [x] CloudNativePG for the database
- [x] i18n (English / French)
- [x] Mobile-responsive UI

### Workspaces
- [x] Apache Guacamole (`guacd`) for VNC/RDP streaming
- [x] Session recordings (per-agent PVC)
- [x] Audio passthrough
- [x] Display scaling
- [x] Shadowing (multiple viewers on a single session)
- [x] Guest access (shareable session links)
- [x] Custom workspace images (Chrome, Firefox, dev environments)
- [x] Per-workspace network isolation via Kubernetes namespaces

### Operations
- [x] Helm charts for core + agent
- [x] Multi-stage Docker builds, no committed binaries
- [x] CI/CD: lint + test on PR, automated semver releases (release-please) on tag, GHCR images + OCI Helm charts
- [x] Trivy CRITICAL/HIGH scanning on every release
- [x] Dependabot security updates

---

## 🚧 In progress — v1.1 (next ~2 months)

### Workspace UX
- [ ] **Native WebRTC viewer** — replace the Guacamole transcode for clients that support it, ~50% latency reduction for typical desktop workflows
- [ ] **Clipboard & file transfer** — proper bidirectional clipboard, drag-and-drop file upload/download into a workspace
- [ ] **Per-user / per-team quotas** — CPU, memory, storage caps enforced at provisioning time

### Workspace lifecycle
- [ ] **Snapshot & restore** — pause a workspace and resume later from disk image
- [ ] **Idle timeout & auto-cleanup** — configurable per template
- [ ] **Lifecycle webhooks** — `workspace.created`, `.started`, `.stopped`, `.terminated` HTTP callbacks for integrations

### Recordings
- [ ] **In-browser playback UI** — scrub, jump, share-link, retention policies
- [ ] **Search & redaction** — keyword search across transcripts, redact frames before sharing

### Auth
- [ ] **LDAP / Active Directory** alongside OIDC and local
- [ ] **SCIM 2.0** for automated user provisioning
- [ ] **Per-agent JWT secret rotation** with zero downtime

---

## 📅 Planned — v1.2 (next ~6 months)

- [ ] **GPU workspaces** — NVIDIA / AMD passthrough via the device plugin, validated templates for CAD, ML notebooks, and graphics work
- [ ] **Workspace template marketplace** — community-published images with a verified-publisher flag and signed digests
- [ ] **Multi-region** — agents in different geographies, smart routing based on user proximity, shared session DB
- [ ] **Audit log dashboard** — searchable, exportable, retention controls, optional SIEM forwarding
- [ ] **Cost & usage analytics** — per-user, per-team, per-workspace-template, exportable to CSV / Prometheus
- [ ] **Plugin system** — extension points for custom auth, custom provisioners, custom UI panels
- [ ] **Backup & disaster recovery** — first-class CNPG backup story, agent state replication

---

## 💭 Future ideas — v2.0+

These are deliberately speculative. Open a discussion if any of them resonate.

- Native mobile clients (iOS / Android) with hardware-accelerated streaming
- WASM-based browser renderer for ultra-low-latency on modern browsers, no `guacd` round-trip
- BYOC adapters — managed adapters for AKS, EKS, GKE so the agent runs on cloud-managed clusters with minimal config
- Multi-tenant federation — multiple organizations sharing one infrastructure plane
- AI-assisted workspaces — context-aware in-session helper, opt-in, runs locally to the cluster
- Pay-per-use billing module — Stripe/Lemonsqueezy hooks for SaaS deployments
- Edge agents — lightweight agents on bare-metal / k3s for on-prem branch offices

---

## How to influence the roadmap

- 👍 [React](https://github.com/enzoamate/oklavier/issues) on existing issues — the most-upvoted items get prioritized
- 💬 [Open a discussion](https://github.com/enzoamate/oklavier/discussions) before filing a feature request for anything large
- ✋ Comment "I'd like to take this" on a 📅 or 💭 item to claim it — small ones don't need an issue first, just open a PR
- 🔒 Security issues → **don't** open a public issue, see [SECURITY.md](SECURITY.md)

## Versioning

We follow [Semantic Versioning](https://semver.org/). The CI auto-bumps via Conventional Commits — see [CONTRIBUTING.md](CONTRIBUTING.md). Patch releases ship as-needed; minor releases ship roughly every 6–8 weeks; major releases when there's a real reason (breaking API changes, only).
