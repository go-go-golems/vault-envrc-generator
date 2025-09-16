---
Title: Analyze Command Reference
Slug: analyze-command-reference
Short: In-depth reference for running and interpreting the environment analysis reports.
Topics:
- analyze
- cli
- diagnostics
IsTemplate: false
IsTopLevel: true
ShowPerDefault: true
SectionType: GeneralTopic
---

# Analyze Command Reference

The `analyze-env` command inspects a seed configuration alongside a captured environment and produces a structured report that highlights mapped variables, missing values, and unmapped secrets. Use this reference whenever you need to understand every flag, report field, and automation hook supported by the analyzer.

## When to Run analyze-env

Run the analyzer before the first seed of a new environment or when auditing changes to an existing seed file. The diff-oriented report lets your team confirm that every required variable is present, discover forgotten secrets that still live in local shells, and catch stale mappings before Terraform or application deploys fail.

## Capturing the Environment

Choosing the correct source determines which variables the analyzer reads. All sources are read-only.

- `current`: Snapshot the process environment exactly as the command runs. Choose this when you already exported variables in the current shell.
- `envrc`: Source a specific `.envrc` in a clean subshell. Combine with `--empty-env` to avoid inheriting ambient variables and pass `--confirm-exec` to acknowledge the shell execution.
- `direnv`: Invoke `direnv export json` so the active direnv profile is captured with the same sandboxing guarantees developers rely on.
- `file`: Parse dotenv-style files such as `.env`. Override the path with `--dotenv` if necessary.

## Key Flags

```bash
vault-envrc-generator analyze-env \
  --config infra/terraform/bots/staging/seed-staging-vault-from-spaces.yaml \
  --env-source envrc \
  --envrc ./bots/staging/.envrc \
  --empty-env \
  --confirm-exec \
  --format table \
  --include-values \
  --censor "***" \
  --report env-analysis.yaml \
  --strict
```

| Flag | Purpose |
|------|---------|
| `--config` | Seed YAML to be analyzed (required). |
| `--env-source` | Environment capture strategy (`current`, `envrc`, `direnv`, `file`). |
| `--envrc`, `--dotenv` | Provide explicit file paths for `envrc` and `file` sources. |
| `--empty-env` | Start from an empty environment before sourcing files. |
| `--confirm-exec` | Required when `--env-source envrc` is used, acknowledging that shell code will run. |
| `--format` | Select Glazed output format (`table`, `json`, `yaml`, etc.). |
| `--include-values` / `--censor` | Control whether variable values appear in the report and how to mask them. |
| `--report` | Persist the full structured report to a file or `-` for `stdout`. |
| `--strict` | Exit with non-zero status when mapped variables are missing or unmapped variables exist. |

## Reading the Report

Each run returns a summary followed by detailed sections. The JSON/YAML structure serialised by `--report` mirrors the `Result` type in the Go API.

- **Summary** contains counters for `mapped_present`, `mapped_missing`, and `unmapped_present`.
- **Mapped Present** lists environment variables that match a seed mapping, showing the target set, Vault path, and key.
- **Mapped Missing** highlights required mappings that were not found. These must be defined before running `seed`.
- **Unmapped Present** surfaces variables that exist in the environment but are not covered by the seed file. Suggested Vault targets help prioritise new mappings.
- **Non-Env Seed Inputs** documents keys sourced from `data`, `files`, `commands`, or `json/yaml` transforms. They provide context but do not require environment values.

### Example table output

```text
SUMMARY
Mapped Present   18
Mapped Missing    2
Unmapped Present  5

MAPPED MISSING
ENV VAR            SET NAME        VAULT PATH                                        VAULT KEY
DATABASE_URL       postgres-main   secrets/environments/staging/resources/postgres   url
SLACK_BOT_TOKEN    slack-app       secrets/environments/staging/resources/slack/app  bot_token
```

## Strict Mode and Automation

`--strict` makes the analyzer CI-friendly. The command continues to print the report, but returns exit code `1` when `mapped_missing > 0` or `unmapped_present > 0`. Use this in pipelines to guard pull requests that modify seed specs.

## Programmatic Usage

The analyzer logic is available as `analyze.Analyze(ctx, opts)`. This function returns a `Result` struct plus the resolved environment map so tooling can generate custom reports or apply additional policy checks. Populate `Options` the same way the CLI does:

```go
result, env, err := analyze.Analyze(ctx, analyze.Options{
    SeedPath:   "infra/terraform/bots/staging/seed-staging-vault-from-spaces.yaml",
    EnvSource:  analyze.EnvSourceEnvrc,
    EnvrcPath:  "./bots/staging/.envrc",
    EmptyEnv:   true,
    ConfirmExec: true,
})
if err != nil {
    log.Fatal(err)
}
```

`result` mirrors the CLI report, while `env` contains the captured variables after any censoring logic.

## Next Steps

Once the analyzer shows no missing or unmapped variables, continue with the seeding workflow:

```
vault-envrc-generator help seed-configuration-guide
```

For command output customization, refer to the Glazed documentation:

```
glaze help build-first-command
```
