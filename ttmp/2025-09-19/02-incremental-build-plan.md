# Incremental Build Plan — Vault Envrc Generator Web

Purpose: deliver value quickly, reduce risk, and keep strict parity with existing CLI semantics by reusing `pkg/` packages. Each stage is designed to be a small, shippable milestone.

Audience & orientation (new joiner quickstart)
- Read this plan alongside the design document: `ttmp/2025-09-19/01-design-for-a-web-view-for-the-vault-envrc-generator.md`.
- Skim project docs: `pkg/doc/03-architecture-overview.md`, `04-yaml-configuration-reference.md`, `05-seed-configuration-guide.md`, `06-vault-glazed-middleware.md`.
- Inspect these files first to understand wiring:
  - CLI entrypoint: `cmd/vault-envrc-generator/main.go`
  - Tree behavior: `cmds/tree.go`
  - Core packages: `pkg/vault/`, `pkg/listing/`, `pkg/envrc/`, `pkg/batch/`, `pkg/output/`, `pkg/vaultlayer/`, `pkg/glazed/`

Key symbols to grep for in code:
- `vault.NewClient`, `(*vault.Client).GetSecrets`, `(*vault.Client).ListSecrets`
- `vault.ResolveToken`, `vault.TokenSource`, `vault.BuildTemplateContext`
- `listing.Walk`, `vault.NormalizeListPath`
- `envrc.NewGenerator`, `(*envrc.Generator).Generate`
- `batch.Processor.Process`, `output.Write`

Principles
- Reuse, don’t rewrite: use `pkg/vault`, `pkg/listing`, `pkg/envrc`, `pkg/batch`, `pkg/output`.
- Secure by default: censor values unless explicitly revealed; prefer server-side token sourcing.
- Production from day one: single binary with embedded web assets; fast dev loop via Vite proxy.
- Keep scope small: defer non-essential features to post-MVP.

References
- Design spec: `ttmp/2025-09-19/01-design-for-a-web-view-for-the-vault-envrc-generator.md`
- Docs: `pkg/doc/03-architecture-overview.md`, `04-yaml-configuration-reference.md`, `05-seed-configuration-guide.md`, `06-vault-glazed-middleware.md`, project `README.md`.

Out-of-scope for MVP (explicitly deferred)
- Email/share, history/favorites, template library UI, theme switching, keyboard shortcuts.
- Real-time collaboration, secret version history visualization, SSO/OIDC login.
- Advanced audit dashboards/metrics; enterprise RBAC UI; multi-tenant UX.
- Server-side file writing for batch in production mode (keep dry-run-only for web).

---

Stage 0 — Bootstrap skeleton (server + frontend shell)
Goals
- Create `serve` command under existing CLI using Glazed + `vaultlayer`.
- Health endpoint: `GET /api/v1/health` returns 200.
- Static hosting with SPA fallback; Vite proxy for dev.

Scope
- Embed `web/dist` via Go `embed` (empty scaffold at first).
- Router + SPA fallback like `go-go-mento/go/cmd/frontend/serve.go`.

Acceptance
- `vault-envrc-generator serve --port 8080` serves index and health.

How to run (dev)
```bash
export VAULT_ADDR="https://your-vault:8200"
go run ./cmd/vault-envrc-generator serve --port 8080 --dev-mode
# separately in web/: pnpm dev (Vite) → visit http://localhost:3000
```

Risks/Mitigations
- None significant. Ensure HEAD support, correct MIME types.

---

Stage 1 — Read-only tree discovery (metadata only)
Goals
- List directories/leaf markers without retrieving values.

Backend
- `GET /api/v1/vault/list/{path}` → use `pkg/vault.NormalizeListPath` + `pkg/vault.Client.ListSecrets`.
- `GET /api/v1/vault/tree?path&depth&include=metadata` → build structure via `pkg/listing.Walk` (sorted, error slice). No values.
- Extract `censorString` from `cmds/tree.go` into shared helper (`pkg/webserver/util`) for future stages.

Frontend
- Tree Explorer page: path input, expand/collapse, show directory vs leaf, inline error badges.

Acceptance
- Navigating typical prefixes (e.g., `secrets/`) shows structure; errors appear inline.

Risks/Mitigations
- Depth and large trees: start with depth controls; no value materialization yet.

---

Stage 2 — Secret materialization with censoring (safe by default)
Goals
- Fetch leaf secret values on demand; default censored view.

Backend
- `GET /api/v1/vault/secrets/{path}` → `pkg/vault.Client.GetSecrets`.
- `GET /api/v1/vault/tree?include=values&reveal=false` → materialize leaves, wrap as `__secret__` object; apply censoring (prefix/suffix from query; defaults 2/2).
- Keep `/tree` default at `include=metadata` for safety.
- Token sourcing from `vaultlayer`; allow `POST /vault/connect` only in `--dev-mode` for testing (never echo token).

Frontend
- Secret drawer/viewer with per-key copy; reveal toggle (client requests reveal; server responds uncensored only when explicitly asked).

Acceptance
- Leaf retrieval works; censoring matches CLI behavior; reveal requires explicit toggle.

Risks/Mitigations
- Accidental value exposure: default to censored; explicit `reveal=true` required per request.

---

Stage 3 — Single-path generation (envrc/json/yaml)
Goals
- Parity with CLI `generate` for single path.

Backend
- `POST /api/v1/generate/{format}` using `pkg/envrc.Generator`.
- Request: path, prefix, include/exclude, transform, sort, template file(optional).

Frontend
- Generate form with live preview; copy/download. No server-side file writes.

Acceptance
- Outputs match CLI for representative fixtures (golden tests for all three formats).

Risks/Mitigations
- Template file option: support only when uploaded inline or disabled; otherwise defer.

---

Stage 4 — UX and performance hardening
Goals
- Keep UI responsive for large trees; standardize errors.

Backend
- Standard error payload: `{ code, message, details? }`.
- Caching hinting (in-memory per-request only); no persistent cache.

Frontend
- Lazy loading on expand; virtualization for large nodes; debounce search.

Acceptance
- Large prefix lists render without jank; errors show uniformly.

Risks/Mitigations
- Virtualization complexity: start with simple windowing; expand as needed.

---

Stage 5 — Batch processing (dry-run only via web)
Goals
- Minimal batch integration to preview generated content without server-side writes.

Backend
- `POST /api/v1/batch/process` → wrap `pkg/batch.Processor` with `DryRun=true` enforced (ignore file writes, return aggregated outputs/logs)
  - For envrc: aggregate into a single text blob per output path.
  - For json/yaml: return merged object(s) per output path.

Frontend
- Textarea/upload for YAML; show result tabs per output path; copy/download.

Acceptance
- Known example configs produce identical dry-run outputs as CLI (stdout mode).

Risks/Mitigations
- Write semantics: keep web-only DRY RUN to avoid server-side filesystem concerns.

---

Stage 6 — Build/packaging pipeline
Goals
- Biome linting; Dagger web build; embed assets; reproducible builds.

Tasks
- `web/` with pnpm+Vite; `biome.json` and scripts.
- Dagger build (similar to `go-go-mento/go/cmd/dagger/build-web/main.go`) exporting `web/dist`.
- `//go:generate` in serve command to run the Dagger build and embed `dist`.

Acceptance
- `go generate ./... && go build` produces single binary serving the frontend.

---

Stage 7 — Testing and documentation
Goals
- Confidence in endpoints and outputs; onboarding docs.

Tests
- Handlers: unit tests for `list/tree/secrets/generate`.
- Golden tests: envrc/json/yaml against fixtures; batch dry-run results.
- Frontend: critical rendering paths (tree node, secret viewer, generator preview).

Docs
- “Getting started” dev guide (Vite + serve); link to existing architecture and batch references.

Acceptance
- CI passes tests; basic e2e manual verification steps documented.

---

Feature removals (from original spec) for MVP
- Removed: email/share, generation history/favorites, template management UI.
- Removed: theme switching, keyboard shortcuts, extensive notifications.
- Removed: real-time collaboration/websocket, secret version UI, SSO login flows.
- Removed: server-side batch writes in production (web keeps DRY RUN only).

Minimal surface area to ship MVP
- Pages: Dashboard (optional splash), Explorer (required), Generator (required), Settings (minimal — depth + censor controls; Vault address read-only from env; dev-mode connect only).
- API: `health`, `list`, `tree`, `secrets`, `generate/*`, `batch/process` (dry-run only), `vault/status`, `vault/connect` (dev-mode).

Rollout plan
- 0.1.0: Stages 0–2 (read-only explorer with censoring + secret viewer).
- 0.2.0: Stage 3 (single-path generation UI/APIs).
- 0.3.0: Stage 4 (UX/perf + error model).
- 0.4.0: Stage 5 (batch dry-run), Stage 6 (build pipeline), Stage 7 (tests/docs).

Risks & mitigations summary
- Token handling: server-side via `vaultlayer`; dev-mode token accepted but never persisted or echoed.
- Large trees: lazy loading + virtualization; enable depth defaults.
- KV v1/v2 differences: rely on `pkg/vault.Client` auto-detection.
- Output determinism: use `SortKeys` consistently; JSON/YAML merges via `pkg/output` semantics.

Success criteria (MVP)
- Explore large Vault prefixes with censored default, reveal on demand.
- Retrieve leaf secrets on demand; errors inline and consistent.
- Generate envrc/json/yaml for a single path with CLI parity.
- Batch dry-run previews match CLI stdout mode for sample configs.

