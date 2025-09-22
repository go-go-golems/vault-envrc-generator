---
Title: Diff Environment — Vault Envrc Generator
Slug: diff-env
Short: Compare current environment against values derived from seed or batch configs
Topics:
- cli
- diagnostics
- env
- seed
- batch
IsTemplate: false
IsTopLevel: true
ShowPerDefault: true
SectionType: GeneralTopic
---

# Diff Environment — Compare Env vs Vault Mapping

The `diff-env` command compares your current process environment to the environment variables that would be produced from Vault using either a seed spec or a batch config. It helps you see what matches, what differs, what is missing locally, and (optionally) what extra variables you have in your shell that are not modeled yet.

## When to use `diff-env`

- **Before onboarding**: Check that your `.envrc` or shell matches the values in Vault
- **During migrations**: Verify local variables against a seed or batch mapping
- **CI checks**: Detect drift between expected env and actual runtime values

## How it works

`diff-env` builds an expected env map by reading Vault using one of:

- Seed spec: uses `sets[].env` mappings to translate Vault keys → env var names
- Batch config: uses `sections[].env_map` when present, otherwise applies the section/job `include/exclude`, `prefix`, and `transform_keys` rules to produce env-style names

The command then compares that expected map to the current process environment.

No writes to Vault occur.

## Usage

```bash
# From a seed spec (uses sets[].env mappings)
vault-envrc-generator diff-env \
  --seed infra/seed.yaml \
  --vault-addr $VAULT_ADDR

# From a batch config (applies env_map or prefix/transform rules)
vault-envrc-generator diff-env \
  --batch infra/batch.yaml \
  --vault-addr $VAULT_ADDR

# Show variables present locally but not in the mapping
vault-envrc-generator diff-env --seed infra/seed.yaml --show-extra

# Override base_path rendering used inside the config templates
vault-envrc-generator diff-env --batch infra/batch.yaml --base-path secrets/environments/dev
```

Vault connection flags are shared across commands (see `vaultlayer`): `--vault-addr`, `--vault-token`, `--vault-token-source`, `--vault-token-file`, etc.

## Flags

- `--seed` string: Path to seed YAML (derive expected env from `sets[].env`)
- `--batch` string: Path to batch YAML (derive expected env from jobs/sections)
- `--base-path` string: Override `base_path` when rendering templates
- `--show-extra` bool: Also list env vars present locally but not in mapping
- `--reveal-values` bool: Print full values instead of censored output
- `--censor-prefix` int: Visible prefix characters when censored (default 2)
- `--censor-suffix` int: Visible suffix characters when censored (default 2)

Exactly one of `--seed` or `--batch` must be provided.

## Output

The command prints a compact summary and a categorized diff. By default, values are censored; use `--reveal-values` to print them fully.

```text
Matches: 12, Changed: 2, Missing: 3, Extra: 4
~ DATABASE_URL
  env="postgres://local..."
  vault="postgres://prod..."
  path=secrets/environments/dev/database
- REDIS_PASSWORD
  vault="***rd"
  path=secrets/environments/dev/cache/redis
+ UNUSED_ENV_VAR
  env="abc...xyz"
```

Legend:
- `~ NAME`: present in both but values differ
- `- NAME`: missing in your environment; expected from Vault mapping
- `+ NAME`: extra in your environment (only shown with `--show-extra`)

## Notes

- Seed mode only considers keys listed under `sets[].env` and only when those keys exist in Vault at runtime
- Batch mode honors `env_map` first; otherwise it uses include/exclude filters, then applies `transform_keys` and `prefix`
- Use `analyze-env` when you want to capture `.envrc`, direnv, or dotenv files rather than the current process environment

## Related

```bash
vault-envrc-generator help analyze-env-preview
vault-envrc-generator help analyze-command-reference
vault-envrc-generator help seed-configuration-guide
vault-envrc-generator help yaml-configuration-reference
vault-envrc-generator help validate
```


