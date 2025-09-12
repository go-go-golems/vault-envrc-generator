---
Title: Seed Configuration Guide — Vault Envrc Generator
Slug: seed-configuration-guide
Short: Guide to populating Vault with secrets using YAML-driven seed configurations
Topics:
- seed
- yaml
- configuration
- vault-population
- development-setup
- migration
IsTemplate: false
IsTopLevel: true
ShowPerDefault: true
SectionType: GeneralTopic
---

# Seed Configuration Guide — Vault Envrc Generator

The `seed` command writes secrets to Vault from local sources (static values, environment variables, and files) using a YAML specification.

## When to use `seed`

- Development setup for new machines or users
- Migration from env/files to Vault
- CI/CD and test fixtures
- Initial bootstrap of new Vault namespaces

## Seed Configuration Structure

Seed configurations define target Vault paths and data sources. Supported sources: `data` (static), `env` (environment variables), `files` (file contents), `json_files`, `yaml_files`, and `commands` (shell commands; output captured as value).

### Top-Level Configuration

```yaml
base_path: secrets/environments/development/{{ .Token.OIDCUserID }}
sets:
  - path: database/primary
    data:
      provider: "postgresql"
      port: "5432"
    env:
      host: DATABASE_HOST
      username: DB_USER
      password: DB_PASSWORD
  
  - path: certificates
    files:
      ca_cert: /etc/ssl/certs/ca-bundle.pem
      server_cert: ~/.ssl/server.crt
      server_key: ~/.ssl/server.key
```

#### base_path (string, optional)

Prefix for relative `path` entries. Supports Go templates.

Template variables:
- `{{ .Token.OIDCUserID }}` - OIDC user identifier extracted from the token
- `{{ .Token.DisplayName }}` - Human-readable token display name  
- `{{ .Token.EntityID }}` - Vault entity identifier
- `{{ .Token.Meta.key }}` - Token metadata values (e.g., environment, team, role)
- `{{ .Extra.key }}` - Additional template data from CLI flags (see below)

Examples:
```yaml
# Static environment-specific path
base_path: secrets/environments/development

# User-specific personal namespace
base_path: secrets/personal/{{ .Token.OIDCUserID }}

# Team and environment specific
base_path: secrets/{{ .Token.Meta.environment }}/{{ .Token.Meta.team }}

# Conditional path based on user role
base_path: secrets/{{- if eq .Token.Meta.role "admin" }}admin{{- else }}user{{- end }}
```

#### sets (array, required)

Each set writes one secret to a Vault path. Sets can combine multiple sources.

## Set configuration

### Set Structure

```yaml
sets:
  - path: oauth/google
    data:
      provider: "google"
      redirect_uri: "http://localhost:3000/auth/callback"
      scope: "openid profile email"
    env:
      client_id: GOOGLE_CLIENT_ID
      client_secret: GOOGLE_CLIENT_SECRET
    files:
      service_account: ~/credentials/google-service-account.json
      private_key: ~/.ssh/google-service-key
```

### Set fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `path` | string | ✓ | Target Vault path (absolute or relative to base_path) |
| `data` | object | | Static key-value pairs written directly to Vault |
| `env` | object | | Environment variable mappings (vault_key: ENV_VAR_NAME) |
| `files` | object | | File content mappings (vault_key: file_path) |
| `json_files` | object | | JSON file with transforms (vault_key: {file, transforms}) |
| `yaml_files` | object | | YAML file with transforms (vault_key: {file, transforms}) |
| `commands` | object | | Shell commands to run (vault_key: command string). Output becomes the value |

Notes:
- `commands` entries are rendered as Go templates before execution (have access to `.Token` and `.Extra`).
- By default, each command prompts for confirmation before running. Use `--allow-commands` to skip prompts.
- Command outputs are trimmed and stored as string values.

### Path resolution

Vault paths in set definitions can be either absolute or relative:

Absolute paths: start with a mount or `/` and are used as-is:
```yaml
sets:
  - path: secret/production/database  # Absolute path
  - path: /secret/shared/certificates  # Absolute path with leading slash
```

Relative paths: joined with `base_path`:
```yaml
base_path: secrets/environments/development
sets:
  - path: database          # Becomes: secrets/environments/development/database
  - path: api/keys         # Becomes: secrets/environments/development/api/keys
```

Templates: supported in absolute and relative paths:
```yaml
sets:
  - path: environments/{{ .Token.Meta.environment }}/database
  - path: users/{{ .Token.OIDCUserID }}/personal
```

## Data sources

### Static data (`data`)

Static data provides fixed key-value pairs that are written directly to Vault. This is useful for configuration constants, metadata, and any values that don't change between environments or deployments.

```yaml
sets:
  - path: app/metadata
    data:
      version: "2.1.0"
      environment: "development"
      maintainer: "platform-team@company.com"
      build_date: "2024-01-15"
      features:
        - "oauth"
        - "metrics"
        - "logging"

  - path: database/config
    data:
      provider: "postgresql"
      port: "5432"
      ssl_mode: "require"
      connection_timeout: "30s"
      max_connections: "100"
```

Use cases: constants, metadata, feature flags, defaults.

### Environment variables (`env`)

Environment variable mappings allow you to source secret values from the current environment at runtime. This is particularly useful for CI/CD scenarios and when migrating from environment-based secret management.

```yaml
sets:
  - path: external/apis
    env:
      openai_key: OPENAI_API_KEY           # Vault key: environment variable name
      github_token: GITHUB_TOKEN
      slack_webhook: SLACK_WEBHOOK_URL
      datadog_api_key: DD_API_KEY

  - path: database/credentials
    env:
      host: DATABASE_HOST
      port: DATABASE_PORT
      username: DATABASE_USER
      password: DATABASE_PASSWORD
      ssl_cert: DATABASE_SSL_CERT
```

Behavior: resolved at runtime; missing variables fail the run; empty values are written as empty strings; names are case‑sensitive.

Use cases: CI/CD, developer credentials, env‑managed secrets, migrations.

### File contents (`files`)

File content mappings read data from local files and store the contents as secret values in Vault. This is essential for certificates, keys, configuration files, and any multi-line secret data.

```yaml
sets:
  - path: certificates/ssl
    files:
      ca_bundle: /etc/ssl/certs/ca-bundle.pem
      server_cert: ~/.ssl/server.crt
      server_key: ~/.ssl/server.key
      client_cert: ~/certs/client.pem

  - path: ssh/keys
    files:
      private_key: ~/.ssh/id_rsa
      public_key: ~/.ssh/id_rsa.pub
      known_hosts: ~/.ssh/known_hosts

  - path: app/config
    files:
      database_config: ~/config/app-development.json
      logging_config: ~/config/logging.yaml
      feature_flags: ~/config/features.toml
```

Path resolution: absolute respected; `~` expands to home; relative resolved from CWD; permissions are not preserved (contents only).

Behavior: read as text; binaries should be base64‑encoded; large files depend on Vault limits; read errors fail the run.

Use cases: TLS/SSH keys, config files, licenses.

### JSON files with transforms (`json_files`)

JSON file mappings allow you to extract specific values from JSON configuration files using dot notation paths. This is particularly useful for extracting credentials, configuration values, or metadata from complex JSON structures.

```yaml
sets:
  - path: database/config
    json_files:
      # Extract specific database configuration values
      db_config:
        file: ~/config/app.json
        transforms:
          host: "database.host"
          port: "database.port"
          username: "database.credentials.username"
          password: "database.credentials.password"
          ssl_mode: "database.options.ssl_mode"

  - path: auth/oauth
    json_files:
      # Extract OAuth configuration from service account file
      google_oauth:
        file: ~/credentials/google-service-account.json
        transforms:
          client_id: "client_id"
          client_email: "client_email"
          private_key_id: "private_key_id"
          private_key: "private_key"
```

Path syntax: dot notation with array indexing support (`servers.0.name`, `config.database.host`).

Behavior: values converted to strings; missing paths fail the run; complex objects/arrays serialized as JSON strings.

Use cases: service account files, complex configurations, API responses, deployment manifests.

### YAML files with transforms (`yaml_files`)

YAML file mappings work similarly to JSON files but parse YAML format. This is ideal for Kubernetes manifests, Helm values, and YAML-based configuration files.

```yaml
sets:
  - path: k8s/secrets
    yaml_files:
      # Extract values from Kubernetes deployment
      deployment_config:
        file: ~/k8s/app-deployment.yaml
        transforms:
          image: "spec.template.spec.containers.0.image"
          service_account: "spec.template.spec.serviceAccountName"
          namespace: "metadata.namespace"
```

Format support: standard YAML; multi-document files use first document; anchors and aliases resolved.

Behavior: identical to JSON transforms; YAML-specific features preserved during parsing.

Use cases: Kubernetes manifests, Helm values, CI/CD configs, Docker Compose files.

### Command execution (`commands`)

Run shell commands and store their stdout as secret values. Useful to generate random secrets or derive values dynamically.

```yaml
sets:
  - path: resources/identity/crypto
    commands:
      jwt_secret: "openssl rand -base64 48"
      oauth_enc_key: "head -c 32 /dev/urandom | base64"
```

Behavior and safety:
- Commands are rendered as templates; you can use `{{ .Token.* }}` and `{{ .Extra.* }}`.
- By default each command asks for confirmation. Use `--allow-commands` to skip prompts in non-interactive contexts.
- Use `--dry-run` to preview without running commands.
- Use `--force` to overwrite existing keys without prompting; otherwise you will be asked per key when a value already exists.

Limitations:
- RSA keypair generation requires producing consistent private/public pairs; prefer generating to temporary files and seeding via `files` source, or pre-seed externally and mirror into your namespace.

## Combining sources

```yaml
sets:
  - path: app/complete-config
    data:
      # Static configuration
      app_name: "my-application"
      version: "1.0.0"
      environment: "development"
    env:
      # Runtime secrets from environment
      database_password: DATABASE_PASSWORD
      api_key: EXTERNAL_API_KEY
    files:
      # Certificate data from files
      ssl_cert: ~/.ssl/app.crt
      ssl_key: ~/.ssl/app.key
    json_files:
      # Extract from JSON configuration
      app_config:
        file: ~/config/app.json
        transforms:
          db_host: "database.host"
          db_port: "database.port"
          feature_flags: "features"
    yaml_files:
      # Extract from YAML manifest
      k8s_config:
        file: ~/k8s/deployment.yaml
        transforms:
          image_tag: "spec.template.spec.containers.0.image"
          replicas: "spec.replicas"
```

Rules: merged into one secret; key conflicts error; processing order `data` → `env` → `files` → `json_files` → `yaml_files` → `commands`; empty values are written.

## Template integration

### Template Context

Templates have access to comprehensive token information and any extra data you pass via CLI flags.

```yaml
# Template context structure
Token:
  OIDCUserID: "user123"
  DisplayName: "oidc-user123"
  Meta:
    environment: "development"
    team: "platform"
Extra:
  env: "development"
  team: "platform"
```

### CLI flags for template data

- `--extra key=value` (repeatable): add ad-hoc values accessible as `{{ .Extra.key }}`
- `--extra-file file.(yaml|json)`: merge structured data into `.Extra`

## Operations

### New flags

```bash
# Overwrite existing keys without prompting
vault-envrc-generator seed --config config.yaml --force

# Allow running commands without confirmation
vault-envrc-generator seed --config config.yaml --allow-commands

# Provide extra template data
vault-envrc-generator seed --config config.yaml \
  --extra env=development --extra team=platform \
  --extra-file extra.yaml
```

### Dry run

```bash
# Preview all operations
vault-envrc-generator seed --config development-setup.yaml --dry-run

# Preview with detailed logging
vault-envrc-generator seed --config development-setup.yaml --dry-run --log-level debug
```

Dry run shows target paths, key counts, rendered templates, file/env/command resolution, and validation issues.

## Security

### Token scope

```hcl
# Example Vault policy for seed operations
path "secrets/environments/development/*" {
  capabilities = ["create", "update", "read"]
}

path "secrets/personal/{{identity.entity.metadata.oidc_user_id}}/*" {
  capabilities = ["create", "update", "read"]
}
```

### Data validation

```bash
# Verify file contents before seeding
cat ~/.ssl/server.crt | openssl x509 -text -noout

# Validate JSON configuration files
jq . ~/config/app.json
```

## Troubleshooting

- 403 on write with `secrets/metadata/...`: Use `secrets/...` base path (KV v2 data path).
- Missing key errors during seed: Limit sets with `--sets` while iterating, fix mappings, or add source keys.
- Commands not running: enable `--allow-commands` or answer prompts interactively.
- Overwrites skipped: use `--force` to overwrite without prompting.

For practical examples and getting started, see:

```
vault-envrc-generator help vault-envrc-getting-started
```

See also the batch guide for runtime fallback commands in sections:

```
vault-envrc-generator help yaml-configuration-reference
```
