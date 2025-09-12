---
Title: YAML Configuration Reference — Vault Envrc Generator
Slug: yaml-configuration-reference
Short: Concise reference for batch processing YAML and template usage
Topics:
- yaml
- batch
- configuration
- templates
IsTemplate: false
IsTopLevel: true
ShowPerDefault: true
SectionType: GeneralTopic
---

# YAML Configuration Reference — Vault Envrc Generator

This document defines the YAML format for the `batch` command and the shared template rules. For the seed format, see:

```
vault-envrc-generator help seed-configuration-guide
```

## Batch Configuration

Batch files describe jobs that read from one or more Vault paths and write a single output per job.

### Top-Level Structure

```yaml
base_path: secrets/environments/{{ .Token.OIDCUserID }}/local
jobs:
  - name: job-name
    description: "Human-readable job description"
    # ... job configuration
```

#### base_path (string, optional)
Prefix applied to relative paths in jobs and sections. Supports Go templates.

Template variables:
- `{{ .Token.OIDCUserID }}` - OIDC user identifier from token
- `{{ .Token.DisplayName }}` - Token display name
- `{{ .Token.EntityID }}` - Vault entity ID
- `{{ .Token.Meta.key }}` - Token metadata values

**Examples:**
```yaml
# Static base path
base_path: secrets/environments/development

# Dynamic user-specific path
base_path: secrets/personal/{{ .Token.OIDCUserID }}/config

# Environment-specific with metadata
base_path: secrets/{{ .Token.Meta.environment }}/shared
```

#### jobs (array, required)
List of jobs and their outputs.

### Job

```yaml
jobs:
  - name: environment-config
    description: "Application environment variables"
    output: config/app.envrc
    format: envrc
    base_path: secrets/app
    prefix: APP_
    transform_keys: true
    sort_keys: true
    sections:
      - name: database
        path: database
        include_keys: [host, port, username, password]
      - name: redis
        path: cache/redis
        prefix: REDIS_
```

#### Job fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | ✓ | Unique job identifier |
| `description` | string | | Human-readable job description |
| `output` | string | ✓ | Output file path (relative to working directory) |
| `format` | string | | Output format: `envrc`, `json`, `yaml` (default: `envrc`) |
| `base_path` | string | | Job-specific base path (overrides global) |
| `prefix` | string | | Default prefix for all keys in this job |
| `transform_keys` | boolean | | Transform keys to UPPERCASE and `-` to `_` |
| `sort_keys` | boolean | | Sort keys deterministically in JSON/YAML |
| `exclude_keys` | array | | Keys to exclude from output |
| `include_keys` | array | | Keys to include (overrides exclude) |
| `template` | string | | Custom template file path |
| `envrc_prefix` | string | | Raw text prepended to envrc outputs (before first section) |
| `envrc_suffix` | string | | Raw text appended to envrc outputs (after all sections) |
| `variables` | object | | Template variables for rendering |
| `sections` | array | | Section definitions for multi-source processing |
| `fixed` | object | | Static key-value pairs added to output |

### Section

```yaml
sections:
  - name: oauth-google
    description: "Google OAuth credentials"
    path: oauth/google
    include_keys: [client_id, client_secret]
    prefix: GOOGLE_
    transform_keys: true
    env_map:
      GOOGLE_CLIENT_ID: client_id
      GOOGLE_CLIENT_SECRET: client_secret
    fixed:
      GOOGLE_PROVIDER: "oauth2"
    # Optional: run shell commands to fill missing values
    commands:
      GOOGLE_CLIENT_ID: "jq -r .installed.client_id ~/credentials/google-oauth.json"
      GOOGLE_CLIENT_SECRET: "jq -r .installed.client_secret ~/credentials/google-oauth.json"
```

#### Section fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | | Section identifier for logging |
| `description` | string | | Human-readable section description |
| `path` | string | | Vault path (relative to base_path if not absolute) |
| `prefix` | string | | Prefix for keys in this section |
| `transform_keys` | boolean | | Transform keys (overrides job setting) |
| `exclude_keys` | array | | Keys to exclude from this section |
| `include_keys` | array | | Keys to include from this section |
| `env_map` | object | | Direct environment variable mapping |
| `fixed` | object | | Static values added to this section |
| `commands` | object | | Shell commands to generate values when not present in Vault |
| `template` | string | | Section-specific template file |
| `variables` | object | | Template variables for section |
| `format` | string | | Section-specific format override |
| `output` | string | | Section-specific output file |

### Advanced
#### **Fallback Commands (`commands`)**
Run shell commands during batch processing to supply values when they are not present in Vault. Commands are rendered as Go templates with access to the same template context as paths. Use `--allow-commands` to run without prompts; otherwise you will be asked once per key (with options to allow/skip all).

```yaml
sections:
  - name: identity-jwt
    path: resources/identity/jwt-rs256
    env_map:
      IDENTITY_SERVICE_JWT_AUDIENCE: audience
    commands:
      IDENTITY_SERVICE_JWT_AUDIENCE: "echo mento-rails"
```

Notes:
- With `env_map`, `commands` keys refer to the target environment variable names.
- Without `env_map`, `commands` keys refer to source keys which are then subject to `prefix`, `transform_keys`, and filtering.
- Dry run prints placeholders like `<command: ...>`.
- Use responsibly; outputs are captured from stdout and trimmed.


#### **envrc prefix/suffix**
Inject raw shell lines before/after the generated `envrc` content. Useful for exports referencing other variables without hitting Vault, e.g. Terraform `TF_VAR_*`.

```yaml
jobs:
  - name: development-environment
    output: .envrc
    format: envrc
    envrc_prefix: |
      # ---- envrc prologue ----
      export TF_VAR_do_token=${DIGITALOCEAN_ACCESS_TOKEN}
      export TF_VAR_spaces_access_key_id=${SPACES_ACCESS_KEY_ID}
      export TF_VAR_spaces_secret_access_key=${SPACES_SECRET_ACCESS_KEY}
    envrc_suffix: |
      # ---- envrc epilogue ----
      layout ruby
```
Notes:
- Prefix/suffix are only applied when `format: envrc` (for jobs or sections resolved to envrc). They are ignored for `json`/`yaml` outputs.
- When multiple sections write to the same envrc output, the prefix appears once at the top, suffix once at the end.
- Prefix/suffix are rendered as Go templates with the same token context as paths (see Template System section).

#### **Environment Mapping (`env_map`)**
Direct mapping from Vault keys to environment variable names, bypassing prefix and transformation rules.

```yaml
sections:
  - name: service-account
    path: google/service-account
    env_map:
      GOOGLE_SERVICE_ACCOUNT_EMAIL: client_email
      GOOGLE_PRIVATE_KEY: private_key
      GOOGLE_PROJECT_ID: project_id
```

#### **Fixed Values (`fixed`)**
Static key-value pairs injected into the output, useful for configuration constants.

```yaml
sections:
  - name: app-config
    path: app/settings
    fixed:
      APP_VERSION: "1.0.0"
      ENVIRONMENT: "development"
      DEBUG: "true"
```

#### **Key Filtering**
Sophisticated inclusion and exclusion patterns for precise secret selection.

```yaml
sections:
  - name: database-config
    path: database
    # Include only connection parameters
    include_keys: [host, port, database, username, password]
    # Exclude production-only settings in development
    exclude_keys: [ssl_cert, ssl_key, backup_*]
```

### Output aggregation

- **envrc**: Sections are concatenated with headers (`# Section: name`)
- **JSON/YAML**: Shallow merge with later sections overriding earlier ones
- **Conflicts**: Later sections take precedence for duplicate keys

### Complete example

```yaml
base_path: secrets/environments/{{ .Token.OIDCUserID }}/local

jobs:
  - name: development-environment
    description: "Complete development environment configuration"
    output: .envrc
    format: envrc
    sort_keys: true
    sections:
      - name: database
        description: "PostgreSQL connection"
        path: database
        include_keys: [host, port, database, username, password]
        prefix: DB_
        transform_keys: true
      
      - name: redis
        description: "Redis cache configuration"
        path: cache/redis
        env_map:
          REDIS_URL: url
          REDIS_PASSWORD: password
      
      - name: api-keys
        description: "External service API keys"
        path: providers
        include_keys: [openai_api_key, anthropic_api_key]
        transform_keys: true
      
      - name: app-config
        description: "Application constants"
        fixed:
          NODE_ENV: "development"
          LOG_LEVEL: "debug"

  - name: kubernetes-secrets
    description: "Kubernetes secret manifests"
    output: k8s/secrets.yaml
    format: yaml
    sort_keys: true
    sections:
      - name: database-secret
        path: database
        include_keys: [username, password]
        prefix: db-
```

## Seed Configuration

See the dedicated guide for the `seed` YAML format:

```
vault-envrc-generator help seed-configuration-guide
```

## Template System

Both batch and seed configurations support Go template syntax for dynamic path and value generation.

### Available Template Context

The template context provides access to Vault token information for dynamic configuration:

```go
type TemplateContext struct {
    Token TokenContext
}

type TokenContext struct {
    Accessor    string                // Token accessor
    DisplayName string                // Display name (e.g., "oidc-user123")
    EntityID    string                // Vault entity ID
    OIDCUserID  string                // Extracted OIDC user ID
    Meta        map[string]string     // Token metadata
    Policies    []string              // Assigned policies
    // ... additional fields
}
```

### Template Examples

#### **User-Specific Paths**
```yaml
# Using OIDC user ID for personal namespaces
base_path: secrets/personal/{{ .Token.OIDCUserID }}/config

# Using display name for path construction
base_path: secrets/users/{{ .Token.DisplayName }}/data
```

#### **Metadata-Driven Configuration**
```yaml
# Using token metadata for environment selection
base_path: secrets/{{ .Token.Meta.environment }}/{{ .Token.Meta.team }}

# Conditional path construction
base_path: secrets/{{- if eq .Token.Meta.role "admin" }}admin{{- else }}user{{- end }}/config
```

#### **Complex Path Logic**
```yaml
jobs:
  - name: user-environment
    base_path: secrets/environments/{{ .Token.Meta.env | default "development" }}/users/{{ .Token.OIDCUserID }}
    sections:
      - name: personal-config
        path: config
      - name: shared-resources
        path: ../shared/{{ .Token.Meta.team }}
```

## Validation

### Required fields
- **Batch**: `jobs` array with at least one job containing `name` and `output`
- **Seed**: `sets` array with at least one set containing `path` and data source

### Path resolution
1. **Absolute paths** (starting with `/` or mount name) are used as-is
2. **Relative paths** are joined with `base_path`
3. **Template rendering** occurs after path resolution
4. **Missing base_path** with relative paths causes validation errors

### Format validation
- **format** must be one of: `envrc`, `json`, `yaml`
- **Output paths** are validated for write permissions
- **Template syntax** is validated during configuration parsing

## Best practices

### Organization

#### Environment-based
```yaml
base_path: secrets/environments/{{ .Token.Meta.environment }}
jobs:
  - name: database-config
    output: config/database.{{ .Token.Meta.environment }}.envrc
    sections:
      - path: database/primary
      - path: database/replica
```

#### Service-oriented
```yaml
jobs:
  - name: api-service
    output: services/api/.envrc
    prefix: API_
    sections:
      - path: services/api/database
      - path: services/api/cache
      - path: shared/monitoring
```

#### Multi-environment
```yaml
jobs:
  - name: development-config
    output: .envrc.development
    base_path: secrets/environments/development
    sections: [...]
  
  - name: staging-config
    output: .envrc.staging
    base_path: secrets/environments/staging
    sections: [...]
```

### Security

1. **Template Validation**: Templates are validated to prevent path injection
2. **Key Filtering**: Use `include_keys` rather than `exclude_keys` for sensitive data
3. **Output Permissions**: Ensure output files have appropriate permissions
4. **Token Scope**: Use least-privilege tokens for batch operations

### Performance

1. **Section Organization**: Group related secrets to minimize Vault API calls
2. **Base Path Usage**: Leverage base paths to avoid repetitive path specifications
3. **Key Filtering**: Filter at the section level to reduce data transfer
4. **Output Formats**: Use appropriate formats (envrc for shell, JSON for APIs)

For practical examples and getting started, see:

```
vault-envrc-generator help vault-envrc-getting-started
```
