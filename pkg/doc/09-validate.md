---
Title: Validate Seed and Batch — Vault Envrc Generator
Slug: validate
Short: Check seed env mappings against local env and cross-check seed vs batch requirements
Topics:
- cli
- validation
- seed
- batch
- diagnostics
IsTemplate: false
IsTopLevel: true
ShowPerDefault: true
SectionType: GeneralTopic
---

# Validate Seed and Batch

The `validate` command performs two classes of checks:

1) Seed env presence: verifies that every `sets[].env` mapping in your seed YAML has a corresponding environment variable set locally.
2) Seed vs batch coverage: when a batch YAML is provided, verifies that every key required by batch sections is present in your seed spec; also reports seed keys that are unused by batch.

It produces structured rows via the Glazed output pipeline, making it suitable for both interactive use and CI.

## When to use `validate`

- Before running `seed`: ensure all referenced environment variables exist
- When introducing a batch file: confirm seed/spec coverage of required keys
- As a CI step: prevent merges that desynchronize seed and batch

## Usage

```bash
# Validate seed env mappings only
vault-envrc-generator validate \
  --seed-config infra/seed.yaml \
  --format table

# Validate seed against batch requirements (and vice versa)
vault-envrc-generator validate \
  --seed-config infra/seed.yaml \
  --batch-config infra/batch.yaml \
  --format table
```

## Flags

- `--seed-config, -s` string (required): Path to seed YAML
- `--batch-config, -b` string: Optional path to batch YAML

Glazed formatting and output flags are available (table, json, yaml, csv, etc.).

## Output rows

Three row types are emitted:

- `seed_env_missing`: A seed `env` mapping refers to an env var that is not set locally
  - Fields: `type`, `set`, `path`, `key`, `env_var`

- `batch_missing_key`: A key required by a batch section is not present in the seed spec
  - Fields: `type`, `path`, `key`, `env_vars` (list of env names that would use this key via `env_map`)

- `seed_key_unused`: A key defined in the seed (data/env/files) is not referenced by the batch file
  - Fields: `type`, `path`, `key`

### Example (table)

```text
TYPE               SET        PATH                                        KEY          ENV_VAR/ENV_VARS
seed_env_missing   db-main    secrets/environments/dev/database           password     DB_PASSWORD
batch_missing_key             secrets/environments/dev/providers/openai   api_key      [OPENAI_API_KEY]
seed_key_unused               secrets/environments/dev/redis              tls_ca_cert
```

## Behavior notes

- This command does not contact Vault; it parses YAML and inspects the current process environment
- Seed keys considered: all keys from `data`, `env`, and `files` in each set
- Batch requirements considered: `sections[].include_keys` and keys referenced by `sections[].env_map`
- For multi-section jobs, requirements are aggregated per section path

## Related

```bash
vault-envrc-generator help seed-configuration-guide
vault-envrc-generator help yaml-configuration-reference
vault-envrc-generator help diff-env
vault-envrc-generator help analyze-command-reference
```


