Applying Software Architecture Document guideline

### Design: envrc/env analysis and preview before seeding

This document sketches a new "analyze-env" feature for `vault-envrc-generator` that previews what is currently in a developer's shell environment and/or `.envrc` and compares it against a `seed` configuration. The goal is to safely show:
- What env vars will be seeded to Vault (present in `env` and mapped in `seed`)
- What expected vars are missing from `env` (mapped in `seed` but not set)
- What extra vars exist in `env` but are not covered by the `seed` config (candidate additions)

This is a read-only preview that helps existing developers migrate their current `.envrc` into Vault without surprises.

### Command overview

```bash
vault-envrc-generator analyze-env \
  --config ttmp/2025-09-08/vault-generation/seed-vault-from-env.yaml \
  --env-source envrc \
  --envrc ./.envrc \
  --empty-env \
  --format table \
  --include-values \
  --censor "***" \
  --output env-analysis.yaml \
  --strict
```

- **--config**: seed YAML to parse (required)
- **--env-source**: where to gather variables from: `current|envrc|direnv|file`
  - `current`: current process `os.Environ()`
  - `envrc`: source `.envrc` in a pristine subshell (see below)
  - `direnv`: use `direnv export json` if available
  - `file`: parse a dotenv-style `.env` file
- **--envrc**: path to `.envrc` (default: `./.envrc`)
- **--empty-env**: when using `envrc`, start from an empty environment for accurate results
- **--format**: `table|yaml|json` (default: `table`)
- **--include-values**: include actual values in output (with `--censor` masking)
- **--censor**: mask for values (default: `"***"`)
- **--output**: write a full machine-readable report (yaml/json) to file
- **--strict**: exit with non-zero code if there are missing expected vars or other issues

### What gets analyzed

From the provided `seed` config (example excerpt), we extract the env variable names from each `sets[].env` mapping:

```yaml
sets:
  - name: postgres-main
    path: resources/postgres/main
    env:
      host: MENTO_SERVICE_DB_HOST
      port: MENTO_SERVICE_DB_PORT
      database: MENTO_SERVICE_DB_DATABASE
      username: MENTO_SERVICE_DB_USER
      password: MENTO_SERVICE_DB_PASSWORD
```

We do not seed from `data`, `files`, or `commands` (those are not sourced from the environment). The analysis reports those sections as "non-env inputs" for context but excludes them from presence checks.

### Capturing environment variables

- **current**: use `os.Environ()`; fast, no side effects
- **envrc**: spawn a clean subshell and source `.envrc`:
  - Default: `env -i HOME="$HOME" PATH="$PATH" bash -lc 'set -a; source ./.envrc >/dev/null 2>&1 || true; env -0'`
  - Parse the `\0`-separated output into a map (`key=value`)
  - Pros: simple, does not require `direnv`
  - Cons: `.envrc` can execute arbitrary commands. We document this and add `--confirm-exec` guard.
- **direnv**: if `direnv` is installed, prefer `direnv export json` in the target directory; parse JSON to a map (safe, accurate)
- **file**: dotenv parser for `.env`-like files

Safety knobs:
- `--confirm-exec`: required to execute `.envrc`; otherwise fail with an explanation
- `--empty-env`: pass `env -i` to avoid leaking the parent environment into the analysis

### Output structure

Machine-readable report (yaml/json) structure:

```yaml
env_source: envrc
envrc_path: ./.envrc
seed_config: ttmp/2025-09-08/vault-generation/seed-vault-from-env.yaml
base_path: secrets/environments/development/personal/${OIDC_USER}/environments/local
summary:
  mapped_present: 18
  mapped_missing: 5
  unmapped_present: 12
  non_env_seed_inputs: 3
details:
  mapped_present:
    - env_var: MENTO_SERVICE_DB_HOST
      value: "***"
      mapped_set: postgres-main
      vault_path: resources/postgres/main
      vault_key: host
    # ...
  mapped_missing:
    - env_var: IDENTITY_SERVICE_DB_PASSWORD
      mapped_set: postgres-identity
      vault_path: resources/postgres/identity
      vault_key: password
    # ...
  unmapped_present:
    - env_var: TF_VAR_do_token
      value: "***"
      suggestions:
        - reason: "Terraform token detected; likely provider path"
          candidate_set: digitalocean
          candidate_vault_path: resources/deploy/digitalocean
          candidate_vault_key: access_token
    # ...
  non_env_seed_inputs:
    - set: identity-jwt
      type: commands
      keys: [private_pem, public_pem]
```

### CLI examples

- Preview using current env only:
```bash
vault-envrc-generator analyze-env \
  --config seed-vault-from-env.yaml --env-source current --format table
```

- Preview by sourcing `.envrc` in a clean subshell and writing a full YAML report:
```bash
vault-envrc-generator analyze-env \
  --config seed-vault-from-env.yaml \
  --env-source envrc --envrc ./.envrc --empty-env --confirm-exec \
  --output env-analysis.yaml --format table
```

- Prefer `direnv` when available:
```bash
vault-envrc-generator analyze-env --config seed.yaml --env-source direnv
```

### Table output (human-friendly)

Columns:
- Category (Mapped Present | Mapped Missing | Unmapped Present)
- Env Var
- Set / Vault Path / Vault Key (when applicable)
- Value (masked unless `--include-values`)

### Heuristics for suggestions (unmapped vars)

When a variable exists in env but is not mapped in `seed`, propose candidates by:
- **Prefix heuristics**: `MENTO_SERVICE_DB_*` → `resources/postgres/main`, `REDIS_*` → `resources/redis/main`, `ELASTICSEARCH_*` → `resources/elasticsearch/main`, `GOOGLE_*` → `resources/google/*`, etc.
- **Known aliases**: `OPENAI_API_KEY` → `resources/ai/openai: api_key`, `ANTHROPIC_API_KEY` → `resources/ai/anthropic: api_key`
- **Seed proximity**: prefer sets already present in `seed` with matching component names

The analyzer lists suggestions with rationale but does not modify files unless a separate flag is provided in the future (see below).

### Optional follow-ups (future flags)

- `--propose-seed-update <path>`: emit a YAML snippet to append to the `seed` config (non-destructive, user merges manually)
- `--only category`: filter results by a category (`mapped-present|mapped-missing|unmapped-present`)
- `--strict`: non-zero exit code if `mapped_missing > 0` or `unmapped_present > 0`

### Implementation sketch

1) Parse seed YAML
- Collect `base_path` (for display only)
- For each `set` with an `env` map, record `envVar → (setName, vaultPath, vaultKey)`
- Record non-env inputs (`data`, `files`, `commands`) for context

2) Capture environment
- `current`: `os.Environ()`
- `envrc`: run `env -i ... bash -lc 'set -a; source <envrc>; env -0'` if `--confirm-exec`
- `direnv`: `direnv export json` and parse
- `file`: parse dotenv file

3) Classify
- `mapped_present`: env has var AND var is mapped in seed
- `mapped_missing`: var is mapped in seed BUT not present in env
- `unmapped_present`: env has var BUT not mapped in seed (filter out noisy system vars; maintain a denylist such as `PWD`, `SHELL`, `HOME`, etc.)

4) Suggestions for unmapped vars
- Apply prefix and alias heuristics; produce suggestion candidates (without side effects)

5) Output
- Render table to stdout (unless `--format yaml|json`)
- Write full report to `--output` if provided

### Security and side effects

- Sourcing `.envrc` executes arbitrary code. Require `--confirm-exec` when `--env-source envrc` is used; otherwise abort with an explanation and suggest `--env-source direnv`.
- Run sourcing in an `env -i` clean environment whenever `--empty-env` is set (recommended), only preserving `HOME` and `PATH`.
- Capture output via `env -0` to correctly handle values containing newlines or spaces.

### Testing plan

- Unit tests for: seed parsing, classification, censoring, suggestion heuristics
- Integration tests: `envrc` sourcing (guarded behind an integration tag), `direnv` path, and dotenv file path
- Golden tests for YAML/JSON output stability

### Integration with existing workflow

- Pair with `seed --dry-run` to show what would be written to Vault after resolving gaps
- Use alongside `batch --dry-run` to preview resulting `.envrc.managed`
- Link from onboarding wizard as a pre-flight check for existing developers

### Example result (YAML excerpt)

```yaml
env_source: envrc
summary: { mapped_present: 22, mapped_missing: 3, unmapped_present: 9 }
details:
  mapped_missing:
    - env_var: IDENTITY_SERVICE_DB_PASSWORD
      mapped_set: postgres-identity
      vault_path: resources/postgres/identity
      vault_key: password
  unmapped_present:
    - env_var: TF_VAR_do_token
      value: "***"
      suggestions:
        - candidate_set: digitalocean
          candidate_vault_path: resources/deploy/digitalocean
          candidate_vault_key: access_token
```

### Next steps

- Implement `analyze-env` as a new command in `cmd/` with shared parsing utilities
- Add `--confirm-exec` guard and `--direnv` mode
- Provide default censoring and allow printing without values by default
- Add docs to README and a short `--help` topic with examples


