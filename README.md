# Vault Envrc Generator — Generate env files from Vault

![](https://img.shields.io/github/license/go-go-golems/vault-envrc-generator)
![](https://img.shields.io/github/actions/workflow/status/go-go-golems/vault-envrc-generator/push.yml?branch=main)

> From Vault to `.envrc`, JSON, or YAML — safely and repeatably.

This CLI turns HashiCorp Vault secrets into developer-friendly environment files. It supports one-off generation, interactive preview, powerful batch composition, Vault listing, and YAML‑driven seeding — all with smart KV v2 detection and flexible token resolution. Built on the Glazed framework.

## Core Features

- KV v2‑first with v1 fallback (no mount list required)
- Multiple token sources: env, file (`~/.vault-token`), or `vault token lookup`
- Commands: `generate`, `interactive`, `batch`, `list`, `seed`, `test`
- Output formats: `.envrc`, `json`, `yaml` (envrc appends; JSON/YAML shallow‑merge)
- Sections‑based batch jobs with headers and per‑section overrides
- YAML seeding from literals, env vars, and files
- KV v2‑aware listing with optional censored values
- Token‑templated paths/outputs and fixed values; deterministic `--sort-keys`

## Installation

Choose one:

- Download binaries from GitHub Releases
- `go install github.com/go-go-golems/vault-envrc-generator/cmd/vault-envrc-generator@latest`
- Run from source: `go run ./cmd/vault-envrc-generator`

## Quick Start

1) Configure Vault
```bash
export VAULT_ADDR="https://vault.example.com:8200"
export VAULT_TOKEN="$(cat ~/.vault-token)"   # optional; token can be auto-resolved
```

2) Common commands
```bash
# Connectivity check
vault-envrc-generator test -v

# Interactive preview and write
vault-envrc-generator interactive

# Generate one path into .envrc
vault-envrc-generator generate --path secrets/environments/dev/shared/database \
  --output out/.envrc --prefix DB_ --transform-keys

# Batch (sections schema)
vault-envrc-generator batch --config go-utility/batch-load-dev.yaml --continue-on-error

# List accessible content (YAML with censored values)
vault-envrc-generator list --path secrets/environments/dev/shared/ \
  --format yaml --include-values --censor "***"

# Seed secrets into Vault from YAML
vault-envrc-generator seed --config go-utility/seed-personal.yaml --dry-run
```

## Commands Overview

### generate
Read a single path and write formatted output (`envrc|json|yaml`).

```bash
vault-envrc-generator generate -p <path> -o out/.envrc --prefix APP_ --transform-keys
```

### interactive
Guided, prompt‑based mode to select a path, preview, and write.

### batch
Compose multiple outputs from multiple paths with a sections‑based schema. Envrc appends with headers; JSON/YAML shallow‑merge.

Example:
```yaml
jobs:
  - name: "Dev envrc"
    description: "Aggregated development variables"
    output: "out/dev/.envrc"
    format: envrc
    transform_keys: true
    sections:
      - name: db
        path: secrets/environments/dev/shared/database
        include_keys: [username, password]
        prefix: DATABASE_
      - name: google-oauth
        path: secrets/external-apis/dev/google-oauth
        env_map:
          GOOGLE_CLIENT_ID: client_id
          GOOGLE_CLIENT_SECRET: client_secret
```

### list
KV v2‑aware listing in YAML or text (with optional censored values).

### seed
Populate Vault from a YAML spec (literals/env/files). Supports token‑templated paths.

## Concepts

- KV Engines: v2 wraps reads under `data/` and listings under `metadata/`. The client tries v2 then falls back to v1.
- Token Resolution: `auto|env|file|lookup` — resolves from flags/env, `~/.vault-token`, or `vault token lookup`.
- Key Transform & Prefix: `transform_keys` uppercases and turns `-` into `_`; `prefix` prepends a string (e.g., `DB_`).
- Output Semantics: Envrc appends; JSON/YAML shallow‑merge. Use `--sort-keys` for deterministic order.
- Sections‑Based Batch: Job defaults plus per‑section overrides, with readable envrc headers.

For a deeper explanation, see pkg/doc/01-topics-overview.md.

## Security & Troubleshooting

- 403 on mount listing is expected for non‑admin tokens; the tool avoids requiring `sys/mounts`.
- 403 on a secret path indicates missing `list`/`read` there.
- Use `test -v` and `list` to probe connectivity and explore capabilities.
- If using the `lookup` source, ensure the Vault CLI is installed and authenticated.

## Development

```bash
go build ./cmd/vault-envrc-generator
go test ./...
```

Key packages:

- `pkg/vault`: KV v2‑first client (Get/Put, metadata listing)
- `pkg/envrc`: Formatting and key transforms
- `pkg/batch`: Sections/legacy schemas and processors
- `pkg/output`: Merge/append semantics for files

## End‑to‑End Example

```bash
# 1) Seed a personal namespace (dry-run first)
vault-envrc-generator seed --config go-utility/seed-personal.yaml --dry-run
vault-envrc-generator seed --config go-utility/seed-personal.yaml

# 2) Explore development paths (YAML + censored values)
vault-envrc-generator list --path secrets/environments/development/ --depth 2 \
  --format yaml --include-values --censor "***"

# 3) Batch-generate a composed .envrc
vault-envrc-generator batch --config go-utility/batch-load-dev.yaml --continue-on-error

# 4) Source it in your shell
source out/dev/.envrc
```
