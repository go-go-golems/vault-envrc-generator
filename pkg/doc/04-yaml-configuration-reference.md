---
Title: YAML Configuration Reference — Vault Envrc Generator
Slug: yaml-configuration-reference
Short: Complete reference for batch processing, seed specifications, and configuration formats
Topics:
- yaml
- batch
- seed
- configuration
- templates
IsTemplate: false
IsTopLevel: true
ShowPerDefault: true
SectionType: GeneralTopic
---

# YAML Configuration Reference — Vault Envrc Generator

The Vault Envrc Generator uses YAML configuration files to define complex batch operations and seed specifications. This reference provides comprehensive documentation for all configuration formats, field options, and usage patterns.

## Batch Configuration Format

Batch configurations enable processing multiple Vault paths with sophisticated transformation, filtering, and output generation capabilities. The configuration uses a hierarchical job-and-section structure to organize complex secret processing workflows.

### Top-Level Structure

```yaml
base_path: secrets/environments/{{ .Token.OIDCUserID }}/local
jobs:
  - name: job-name
    description: "Human-readable job description"
    # ... job configuration
```

#### **base_path** (string, optional)
The default base path prepended to all relative paths in jobs and sections. Supports Go template syntax for dynamic path construction.

**Template Variables Available:**
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

#### **jobs** (array, required)
Array of job definitions that specify processing operations, output generation, and section organization.

### Job Configuration

Each job represents a logical grouping of secrets processing with shared output destination and formatting options.

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

#### Job Fields Reference

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
| `variables` | object | | Template variables for rendering |
| `sections` | array | | Section definitions for multi-source processing |
| `fixed` | object | | Static key-value pairs added to output |

### Section Configuration

Sections provide granular control over individual Vault paths within a job, allowing per-section filtering, transformation, and mapping.

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
```

#### Section Fields Reference

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
| `template` | string | | Section-specific template file |
| `variables` | object | | Template variables for section |
| `format` | string | | Section-specific format override |
| `output` | string | | Section-specific output file |

### Advanced Features

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

### Output Aggregation Behavior

Multiple sections within a job are aggregated based on the output format:

- **envrc**: Sections are concatenated with headers (`# Section: name`)
- **JSON/YAML**: Shallow merge with later sections overriding earlier ones
- **Conflicts**: Later sections take precedence for duplicate keys

### Complete Batch Example

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

## Seed Configuration Format

Seed configurations define how to populate Vault with secrets from local sources including environment variables, files, and static data.

### Top-Level Structure

```yaml
base_path: secrets/personal/{{ .Token.OIDCUserID }}
sets:
  - path: app/config
    data:
      api_version: "v1"
    env:
      api_key: API_KEY
    files:
      private_key: ~/.ssh/id_rsa
```

#### **base_path** (string, optional)
Default base path for all relative paths in sets. Supports the same template variables as batch configurations.

#### **sets** (array, required)
Array of set definitions that specify target Vault paths and data sources.

### Set Configuration

Each set defines a target Vault path and the data sources to populate it with.

```yaml
sets:
  - path: oauth/google
    data:
      provider: "google"
      scope: "openid profile email"
    env:
      client_id: GOOGLE_CLIENT_ID
      client_secret: GOOGLE_CLIENT_SECRET
    files:
      service_account: ~/credentials/google-service-account.json
```

#### Set Fields Reference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `path` | string | ✓ | Target Vault path (absolute or relative to base_path) |
| `data` | object | | Static key-value pairs |
| `env` | object | | Environment variable mappings (vault_key: ENV_VAR) |
| `files` | object | | File content mappings (vault_key: file_path) |

### Data Sources

#### **Static Data (`data`)**
Direct key-value pairs written to Vault as-is.

```yaml
sets:
  - path: app/metadata
    data:
      version: "1.2.3"
      environment: "production"
      maintainer: "devops@company.com"
```

#### **Environment Variables (`env`)**
Values sourced from environment variables at runtime.

```yaml
sets:
  - path: external/apis
    env:
      openai_key: OPENAI_API_KEY      # Vault key: env var name
      github_token: GITHUB_TOKEN
      slack_webhook: SLACK_WEBHOOK_URL
```

#### **File Contents (`files`)**
Values loaded from file contents, supporting home directory expansion (`~`).

```yaml
sets:
  - path: certificates
    files:
      ca_cert: /etc/ssl/certs/ca.pem
      private_key: ~/.ssh/service_key
      config_json: ~/app/config.json
```

### Complete Seed Example

```yaml
base_path: secrets/environments/development/personal/{{ .Token.OIDCUserID }}

sets:
  - path: database
    data:
      provider: "postgresql"
      port: "5432"
    env:
      host: DB_HOST
      username: DB_USER
      password: DB_PASSWORD

  - path: oauth/google
    data:
      provider: "google"
      redirect_uri: "http://localhost:3000/auth/callback"
    env:
      client_id: GOOGLE_CLIENT_ID
      client_secret: GOOGLE_CLIENT_SECRET
    files:
      service_account: ~/credentials/google-service-account.json

  - path: certificates/ssl
    files:
      cert: ~/.ssl/server.crt
      key: ~/.ssl/server.key
      ca_bundle: ~/.ssl/ca-bundle.pem
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

## Configuration Validation

### Required Fields
- **Batch**: `jobs` array with at least one job containing `name` and `output`
- **Seed**: `sets` array with at least one set containing `path` and data source

### Path Resolution Rules
1. **Absolute paths** (starting with `/` or mount name) are used as-is
2. **Relative paths** are joined with `base_path`
3. **Template rendering** occurs after path resolution
4. **Missing base_path** with relative paths causes validation errors

### Format Validation
- **format** must be one of: `envrc`, `json`, `yaml`
- **Output paths** are validated for write permissions
- **Template syntax** is validated during configuration parsing

## Best Practices

### Organization Patterns

#### **Environment-Based Structure**
```yaml
base_path: secrets/environments/{{ .Token.Meta.environment }}
jobs:
  - name: database-config
    output: config/database.{{ .Token.Meta.environment }}.envrc
    sections:
      - path: database/primary
      - path: database/replica
```

#### **Service-Oriented Structure**
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

#### **Multi-Environment Generation**
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

### Security Considerations

1. **Template Validation**: Templates are validated to prevent path injection
2. **Key Filtering**: Use `include_keys` rather than `exclude_keys` for sensitive data
3. **Output Permissions**: Ensure output files have appropriate permissions
4. **Token Scope**: Use least-privilege tokens for batch operations

### Performance Optimization

1. **Section Organization**: Group related secrets to minimize Vault API calls
2. **Base Path Usage**: Leverage base paths to avoid repetitive path specifications
3. **Key Filtering**: Filter at the section level to reduce data transfer
4. **Output Formats**: Use appropriate formats (envrc for shell, JSON for APIs)

This comprehensive reference covers all configuration options and usage patterns for the Vault Envrc Generator's YAML configuration system. For practical examples and getting started guidance, see:

```
glaze help vault-envrc-getting-started
```
