---
Title: End-to-End Workflow Walkthrough — Vault Envrc Generator
Slug: workflow-walkthrough
Short: A practical, step-by-step journey from discovery to seeding to generating app config
Topics:
- tutorial
- workflows
- onboarding
- seed
- batch
- analyze
- diff
IsTemplate: false
IsTopLevel: true
ShowPerDefault: true
SectionType: Tutorial
---

# End-to-End Workflow Walkthrough

This walkthrough guides you through a realistic workflow using Vault Envrc Generator, from initial exploration to validation, seeding, and generating application configuration files.

You can copy/paste the commands and adapt the paths to your environment.

## 1) Connect and Explore

First ensure connectivity to Vault and explore what’s available.

```bash
export VAULT_ADDR=https://vault.company.com:8200
export VAULT_TOKEN=hvs.XXXXXXXX

# Verify connection and permissions
vault-envrc-generator list --path secrets/ --format table

# Explore a likely app namespace
vault-envrc-generator list --path secrets/environments/development --format table
```

If you cannot list secrets, check address and token configuration. See Getting Started for connection methods.

## 2) Prepare Seed and Batch Specs

Create a minimal seed spec that writes your local values into Vault. Example:

```yaml
# seed.yaml
base_path: secrets/environments/development
sets:
  - name: database
    path: database
    data:
      provider: postgresql
    env:
      host: DB_HOST
      username: DB_USER
      password: DB_PASSWORD
  - name: api-keys
    path: providers
    env:
      openai_api_key: OPENAI_API_KEY
```

Create a batch spec to read secrets and produce configuration files for your app:

```yaml
# batch.yaml
base_path: secrets/environments/development/app
jobs:
  - name: app-env
    description: Application environment
    output: .envrc
    format: envrc
    sections:
      - name: database
        path: ../database
        prefix: DB_
        transform_keys: true
        include_keys: [host, username, password]
      - name: api-keys
        path: ../providers
        env_map:
          OPENAI_API_KEY: openai_api_key
```

## 3) Analyze Current Environment (Optional but Recommended)

Preview how your local environment maps to the seed spec before writing anything to Vault.

```bash
vault-envrc-generator analyze-env \
  --config seed.yaml \
  --env-source envrc \
  --envrc ./.envrc \
  --empty-env \
  --confirm-exec \
  --format table \
  --include-values --censor "***" \
  --strict
```

Fix any missing variables locally (e.g., `export DB_PASSWORD=...`).

## 4) Validate Seed and Batch

Ensure your seed `env` mappings are present locally and that batch-required keys are covered by the seed.

```bash
# Seed env presence only
vault-envrc-generator validate --seed-config seed.yaml --format table

# Cross-check seed vs batch
vault-envrc-generator validate --seed-config seed.yaml --batch-config batch.yaml --format table
```

Address reported issues (`seed_env_missing`, `batch_missing_key`, `seed_key_unused`) before proceeding.

## 5) Seed Vault (Dry Run, then Apply)

Write the local values to Vault using the seed spec.

```bash
# Preview writes
vault-envrc-generator seed --config seed.yaml --dry-run

# Apply writes
vault-envrc-generator seed --config seed.yaml
```

## 6) Generate Application Config

Produce your application configuration files from Vault using the batch spec.

```bash
vault-envrc-generator batch --config batch.yaml --format envrc

# Review and load
cat .envrc
source .envrc
```

Optionally emit JSON or YAML files too by adding another job or switching `format`.

## 7) Diff Local Env vs Vault Mapping

Detect drift between what Vault would produce and your current shell.

```bash
# Use seed-based mapping
vault-envrc-generator diff-env --seed seed.yaml

# Or batch-based mapping
vault-envrc-generator diff-env --batch batch.yaml --show-extra
```

Look for `Changed`, `Missing`, and `Extra` to keep environments consistent.

## 8) Iterate Safely

- Use `--dry-run` to preview writes
- Prefer `env_map` for explicit variable naming in batch
- Keep seed/batch files in version control
- Run `validate` in CI to prevent drift

## References

```bash
vault-envrc-generator help vault-envrc-getting-started
vault-envrc-generator help yaml-configuration-reference
vault-envrc-generator help seed-configuration-guide
vault-envrc-generator help analyze-command-reference
vault-envrc-generator help diff-env
vault-envrc-generator help validate
```


