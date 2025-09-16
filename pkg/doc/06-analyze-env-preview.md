---
Title: Analyze Environment Before Seeding
Slug: analyze-env-preview
Short: Preview environment variables and seed coverage before migrating secrets into Vault
Topics:
- cli
- onboarding
- envrc
IsTemplate: false
IsTopLevel: true
ShowPerDefault: true
SectionType: GeneralTopic
---

# Analyze Environment Before Seeding

The `analyze-env` command inspects your current environment, `.envrc`, or dotenv file and compares it against a seed specification. Use it to understand exactly which values will be captured, notice missing variables, and spot secrets that are not yet modeled in Vault before you run any destructive operations.

## When to Use analyze-env

Running a dry analysis is the safest way to migrate an existing developer setup. The report highlights the variables that will seed Vault, those you still need to define locally, and extra values that may deserve a new mapping. Teams can share the report to agree on naming conventions before secrets are written anywhere else.

## Command Overview

The command accepts a seed YAML and lets you choose the environment source. The example below sources an `.envrc` in a clean shell, masks sensitive values, prints a table, and writes the full YAML report to disk.

```bash
vault-envrc-generator analyze-env \
  --config ttmp/2025-09-08/vault-generation/seed-vault-from-env.yaml \
  --env-source envrc \
  --envrc ./.envrc \
  --empty-env \
  --confirm-exec \
  --format table \
  --include-values \
  --censor "***" \
  --report env-analysis.yaml \
  --strict
```

## Environment Sources

Choosing the correct source ensures the analyzer sees the same variables that a developer uses day to day. Each option is read-only and leaves the original files untouched.

- `current`: captures the process environment exactly as the command runs. Use it when the shell already has every variable exported.
- `envrc`: spawns a clean subshell and sources the given `.envrc`. Combine with `--empty-env` to avoid inheriting ambient values. You must pass `--confirm-exec` because `.envrc` files can execute arbitrary scripts.
- `direnv`: runs `direnv export json` if the tool is available. This is the safest option when projects already rely on direnv for sandboxing.
- `file`: parses dotenv-style files. Provide a custom path through `--dotenv` when the file is not named `.env`.

## Understanding the Report

The analyzer produces a summary row plus detailed categories that match the YAML output. Masked values keep secrets out of scrollback while still indicating that data exists.

- **Mapped Present**: Variables that are available in the chosen environment and mapped in the seed file. The report shows the target set, Vault path, and key.
- **Mapped Missing**: Seed mappings that could not be resolved from the environment. These must be set locally before running `seed`.
- **Unmapped Present**: Variables found in the environment but absent from the seed configuration so you can decide whether to add new mappings.
- **Non-Env Seed Inputs**: Keys that originate from `data`, `files`, or `commands` in the seed file. They are included for context only.

## Formatting and Output Options

The analyzer reuses the Glazed output pipeline, so you can switch formats without changing the command logic. `--format table` renders a human readable view, while `--format json` or `--format yaml` stream machine friendly data that is compatible with the rest of Glazed tooling. Use `--report` to write a full YAML or JSON report; the extension determines the serialization, and `--report -` prints the structured data to `stdout`.

## Strict Mode and Exit Codes

Enable `--strict` to fail the command when either mapped variables are missing or unmapped variables are detected. This is ideal for CI checks that guard pull requests. Even in strict mode the analyzer still prints the full report, so you can inspect the discrepancies before fixing them.

## Safety and Trust

Sourcing an `.envrc` executes arbitrary shell code. Always review the file and pass `--confirm-exec` only when you are confident about its contents. Prefer the `direnv` source when available because it performs its own sandboxing. All other sources only read files, never modify them, and the analyzer never writes to Vault.

## Related Topics

Continue with the seeding workflow once your environment is clean:

```
vault-envrc-generator help seed-configuration-guide
```

To learn more about output customization, review the Glazed documentation:

```
glaze help build-first-command
```
