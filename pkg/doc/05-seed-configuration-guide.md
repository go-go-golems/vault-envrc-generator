---
Title: Seed Configuration Guide — Vault Envrc Generator
Slug: seed-configuration-guide
Short: Complete guide to populating Vault with secrets using YAML-driven seed configurations
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
SectionType: Reference
---

# Seed Configuration Guide — Vault Envrc Generator

The seed command reverses the typical Vault workflow by populating Vault with secrets from local sources. This capability is essential for development environment setup, migrating secrets from other systems, and bootstrapping new Vault instances. The seed command uses YAML configuration files to define exactly what data should be written to which Vault paths.

## When to Use Seed Operations

Seed operations are particularly valuable in several scenarios where you need to populate Vault with data from external sources:

**Development Environment Setup**: New team members need their local Vault instance populated with the secrets necessary for development work. Rather than manually entering dozens of secrets through the Vault UI, a seed configuration can populate everything automatically.

**Migration from Other Secret Stores**: When moving from other secret management systems or from file-based configuration, seed operations provide a structured way to transfer existing secrets into Vault while maintaining organization and applying consistent naming patterns.

**Testing and CI/CD**: Automated testing environments often need predictable secret data. Seed configurations ensure that test Vault instances have exactly the secrets needed for comprehensive testing scenarios.

**Initial System Bootstrap**: New Vault deployments require initial secret population. Seed configurations provide a repeatable, auditable way to establish the base secret structure for new environments.

## Seed Configuration Structure

Seed configurations use a straightforward YAML structure that defines target Vault paths and the data sources used to populate them. The configuration supports multiple data sources including static values, environment variables, and file contents.

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

#### **base_path** (string, optional)

The base path serves as a prefix for all relative paths defined in the sets array. This allows you to organize secrets under a common hierarchy without repeating the full path in every set definition. The base path supports Go template syntax for dynamic path construction based on token information.

**Template Variables Available:**
- `{{ .Token.OIDCUserID }}` - OIDC user identifier extracted from the token
- `{{ .Token.DisplayName }}` - Human-readable token display name  
- `{{ .Token.EntityID }}` - Vault entity identifier
- `{{ .Token.Meta.key }}` - Token metadata values (e.g., environment, team, role)

**Base Path Examples:**
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

#### **sets** (array, required)

The sets array contains individual set definitions, each specifying a target Vault path and the data sources used to populate it. Each set can combine multiple data sources, allowing you to build comprehensive secret structures from various inputs.

## Set Configuration Details

Each set in the configuration defines a complete secret population operation, specifying where the data should be written in Vault and where it should be sourced from locally.

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

### Set Fields Reference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `path` | string | ✓ | Target Vault path (absolute or relative to base_path) |
| `data` | object | | Static key-value pairs written directly to Vault |
| `env` | object | | Environment variable mappings (vault_key: ENV_VAR_NAME) |
| `files` | object | | File content mappings (vault_key: file_path) |

### Path Resolution

Vault paths in set definitions can be either absolute or relative:

**Absolute Paths**: Start with a mount point name or `/` and are used exactly as specified:
```yaml
sets:
  - path: secret/production/database  # Absolute path
  - path: /secret/shared/certificates  # Absolute path with leading slash
```

**Relative Paths**: Combined with the base_path to create the final Vault path:
```yaml
base_path: secrets/environments/development
sets:
  - path: database          # Becomes: secrets/environments/development/database
  - path: api/keys         # Becomes: secrets/environments/development/api/keys
```

**Template Support**: Both absolute and relative paths support Go template syntax:
```yaml
sets:
  - path: environments/{{ .Token.Meta.environment }}/database
  - path: users/{{ .Token.OIDCUserID }}/personal
```

## Data Sources

The seed command supports three types of data sources, which can be combined within a single set to create comprehensive secret structures.

### Static Data (`data`)

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

**Use Cases for Static Data:**
- Application version information and build metadata
- Configuration constants that don't vary by environment
- Feature flags and application settings
- Default values and fallback configurations

### Environment Variables (`env`)

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

**Environment Variable Behavior:**
- Variables are resolved at runtime when the seed command executes
- Missing environment variables cause the seed operation to fail with a clear error message
- Empty environment variables are treated as empty strings in Vault
- Variable names are case-sensitive and must match exactly

**Use Cases for Environment Variables:**
- CI/CD pipeline secrets that vary by build environment
- Developer-specific credentials and API keys
- Secrets that are already managed through environment variables
- Migration from Docker secrets or Kubernetes secrets

### File Contents (`files`)

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
      database_config: ~/config/database.json
      logging_config: ~/config/logging.yaml
      feature_flags: ~/config/features.toml
```

**File Path Resolution:**
- **Absolute paths** are used exactly as specified: `/etc/ssl/certs/ca.pem`
- **Home directory expansion** is supported: `~/.ssh/id_rsa` expands to `/home/user/.ssh/id_rsa`
- **Relative paths** are resolved relative to the current working directory
- **File permissions** are not preserved; only file contents are stored in Vault

**File Reading Behavior:**
- Files are read as text content and stored as string values in Vault
- Binary files are supported but stored as base64-encoded strings
- Large files are supported but consider Vault's value size limits
- File reading errors cause the entire seed operation to fail

**Use Cases for File Contents:**
- SSL/TLS certificates and private keys
- SSH keys and known hosts files
- Configuration files that need to be stored as secrets
- License files and other credential documents

## Data Source Combination

Multiple data sources can be combined within a single set to create comprehensive secret structures. The seed command processes all data sources and combines them into a single secret at the target Vault path.

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
```

**Combination Rules:**
- All data sources are merged into a single Vault secret
- Key conflicts between sources result in an error
- Processing order: `data`, then `env`, then `files`
- Empty values from any source are included in the final secret

## Template System Integration

The seed command uses the same template system as other Vault Envrc Generator commands, providing access to token information for dynamic path construction.

### Template Context

Templates have access to comprehensive token information:

```yaml
# Template context structure
Token:
  OIDCUserID: "user123"              # Extracted from OIDC tokens
  DisplayName: "oidc-user123"        # Token display name
  EntityID: "entity-abc123"          # Vault entity identifier
  Meta:                              # Token metadata
    environment: "development"
    team: "platform"
    role: "developer"
  Policies: ["default", "developer"] # Assigned policies
```

### Dynamic Path Examples

```yaml
# User-specific development secrets
base_path: secrets/personal/{{ .Token.OIDCUserID }}/development
sets:
  - path: database
    data:
      environment: "{{ .Token.Meta.environment }}"
      user: "{{ .Token.OIDCUserID }}"

# Environment-specific configuration
base_path: secrets/environments/{{ .Token.Meta.environment }}
sets:
  - path: shared/database
    env:
      host: "{{ .Token.Meta.environment | upper }}_DATABASE_HOST"
      password: "{{ .Token.Meta.environment | upper }}_DATABASE_PASSWORD"

# Conditional paths based on user role
sets:
  - path: "{{- if eq .Token.Meta.role \"admin\" }}admin/secrets{{- else }}user/secrets{{- end }}"
    data:
      access_level: "{{ .Token.Meta.role }}"
```

## Complete Configuration Examples

### Development Environment Bootstrap

This example shows how to bootstrap a complete development environment with all necessary secrets:

```yaml
base_path: secrets/environments/development/personal/{{ .Token.OIDCUserID }}

sets:
  # Database configuration
  - path: database/primary
    data:
      provider: "postgresql"
      port: "5432"
      ssl_mode: "disable"
      max_connections: "10"
    env:
      host: DEV_DATABASE_HOST
      database: DEV_DATABASE_NAME
      username: DEV_DATABASE_USER
      password: DEV_DATABASE_PASSWORD

  # External API credentials
  - path: apis/external
    env:
      openai_api_key: OPENAI_API_KEY
      github_token: GITHUB_TOKEN
      slack_webhook: SLACK_WEBHOOK_URL
      datadog_api_key: DATADOG_API_KEY

  # OAuth configuration
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

  # SSL certificates for local development
  - path: certificates/local
    files:
      ca_cert: ~/.ssl/ca.pem
      server_cert: ~/.ssl/localhost.crt
      server_key: ~/.ssl/localhost.key

  # SSH keys for deployment
  - path: ssh/deployment
    files:
      private_key: ~/.ssh/deploy_key
      public_key: ~/.ssh/deploy_key.pub
      known_hosts: ~/.ssh/known_hosts

  # Application configuration
  - path: app/config
    data:
      log_level: "debug"
      feature_flags:
        - "new_ui"
        - "advanced_metrics"
      cache_ttl: "300s"
    files:
      app_config: ~/config/app-development.json
```

### Migration from Environment Variables

This example demonstrates migrating from an environment variable-based setup to Vault:

```yaml
base_path: secrets/migration/{{ .Token.Meta.environment }}

sets:
  # Web application secrets
  - path: web/app
    env:
      secret_key: SECRET_KEY
      session_secret: SESSION_SECRET
      csrf_token: CSRF_TOKEN
      jwt_secret: JWT_SECRET

  # Database connections
  - path: databases/primary
    env:
      url: DATABASE_URL
      host: DATABASE_HOST
      port: DATABASE_PORT
      name: DATABASE_NAME
      user: DATABASE_USER
      password: DATABASE_PASSWORD

  # Redis configuration
  - path: cache/redis
    env:
      url: REDIS_URL
      password: REDIS_PASSWORD
      max_connections: REDIS_MAX_CONNECTIONS

  # External service credentials
  - path: services/external
    env:
      stripe_secret_key: STRIPE_SECRET_KEY
      stripe_webhook_secret: STRIPE_WEBHOOK_SECRET
      sendgrid_api_key: SENDGRID_API_KEY
      aws_access_key_id: AWS_ACCESS_KEY_ID
      aws_secret_access_key: AWS_SECRET_ACCESS_KEY

  # Monitoring and observability
  - path: monitoring
    env:
      datadog_api_key: DD_API_KEY
      datadog_app_key: DD_APP_KEY
      sentry_dsn: SENTRY_DSN
      prometheus_token: PROMETHEUS_TOKEN
```

### Certificate and Key Management

This example focuses on managing certificates and keys for different environments:

```yaml
base_path: secrets/certificates/{{ .Token.Meta.environment }}

sets:
  # Root CA certificates
  - path: ca/root
    data:
      issuer: "Company Internal CA"
      validity_period: "10 years"
    files:
      ca_cert: /etc/ssl/certs/company-ca.pem
      ca_key: /etc/ssl/private/company-ca-key.pem
      ca_bundle: /etc/ssl/certs/ca-bundle.pem

  # Web server certificates
  - path: web/ssl
    data:
      common_name: "*.company.com"
      san_domains:
        - "company.com"
        - "api.company.com"
        - "app.company.com"
    files:
      server_cert: ~/.ssl/server.crt
      server_key: ~/.ssl/server.key
      intermediate_cert: ~/.ssl/intermediate.crt

  # Client certificates for mutual TLS
  - path: client/mtls
    files:
      client_cert: ~/.ssl/client.crt
      client_key: ~/.ssl/client.key
      ca_cert: ~/.ssl/ca.pem

  # SSH host keys
  - path: ssh/host-keys
    files:
      rsa_host_key: /etc/ssh/ssh_host_rsa_key
      rsa_host_cert: /etc/ssh/ssh_host_rsa_key.pub
      ed25519_host_key: /etc/ssh/ssh_host_ed25519_key
      ed25519_host_cert: /etc/ssh/ssh_host_ed25519_key.pub

  # Application signing keys
  - path: signing/jwt
    data:
      algorithm: "RS256"
      key_size: "2048"
    files:
      private_key: ~/.keys/jwt-signing.key
      public_key: ~/.keys/jwt-signing.pub
```

## Operational Workflows

### Dry Run Operations

Always preview seed operations before executing them to understand exactly what will be written to Vault:

```bash
# Preview all operations
vault-envrc-generator seed --config development-setup.yaml --dry-run

# Preview with detailed logging
vault-envrc-generator seed --config development-setup.yaml --dry-run --log-level debug
```

The dry run output shows:
- Which Vault paths will be written to
- How many keys will be written to each path
- Any template rendering results
- File reading and environment variable resolution
- Validation errors or warnings

### Incremental Updates

Seed operations can be run multiple times safely. The command overwrites existing secrets at the specified paths, allowing for incremental updates and corrections:

```bash
# Initial seed
vault-envrc-generator seed --config base-setup.yaml

# Add additional secrets
vault-envrc-generator seed --config additional-secrets.yaml

# Update existing secrets
vault-envrc-generator seed --config updated-config.yaml
```

### Validation and Testing

Validate seed configurations before deployment:

```bash
# Test configuration parsing
vault-envrc-generator seed --config production-seed.yaml --dry-run

# Verify all files and environment variables are available
vault-envrc-generator seed --config production-seed.yaml --dry-run --log-level debug

# Test against development Vault first
VAULT_ADDR=http://dev-vault:8200 vault-envrc-generator seed --config production-seed.yaml --dry-run
```

## Security Considerations

### File Permissions

Ensure seed configuration files and referenced files have appropriate permissions:

```bash
# Secure the seed configuration file
chmod 600 production-seed.yaml

# Secure referenced certificate files
chmod 600 ~/.ssl/*.key
chmod 644 ~/.ssl/*.crt

# Secure SSH keys
chmod 600 ~/.ssh/id_rsa
chmod 644 ~/.ssh/id_rsa.pub
```

### Token Scope

Use tokens with minimal required permissions for seed operations:

```hcl
# Example Vault policy for seed operations
path "secrets/environments/development/*" {
  capabilities = ["create", "update", "read"]
}

path "secrets/personal/{{identity.entity.metadata.oidc_user_id}}/*" {
  capabilities = ["create", "update", "read"]
}
```

### Audit Trail

Seed operations are logged for audit purposes:

```bash
# Enable audit logging
vault-envrc-generator seed --config production-seed.yaml --log-level info

# Review audit logs in Vault
vault audit list
```

### Data Validation

Validate sensitive data before seeding:

```bash
# Verify file contents before seeding
cat ~/.ssl/server.crt | openssl x509 -text -noout

# Check environment variables
echo $DATABASE_PASSWORD | wc -c  # Verify password length

# Validate JSON configuration files
jq . ~/config/app.json
```

## Troubleshooting

### Common Issues

**File Not Found Errors**:
```bash
# Verify file paths
ls -la ~/.ssl/server.crt
ls -la ~/config/app.json

# Check file permissions
stat ~/.ssl/server.key
```

**Environment Variable Issues**:
```bash
# Verify environment variables are set
env | grep DATABASE_PASSWORD
printenv OPENAI_API_KEY

# Check for typos in variable names
vault-envrc-generator seed --config config.yaml --dry-run --log-level debug
```

**Template Rendering Errors**:
```bash
# Check token information
vault token lookup -format=json

# Verify metadata availability
vault token lookup -format=json | jq '.data.meta'

# Test template rendering
vault-envrc-generator seed --config config.yaml --dry-run
```

**Vault Permission Errors**:
```bash
# Check token capabilities
vault token capabilities secrets/environments/development/database

# Verify path permissions
vault kv get secrets/environments/development/test
```

The seed command provides a comprehensive solution for populating Vault with secrets from local sources. By understanding the configuration format and following operational best practices, you can create reliable, repeatable processes for managing secret data across different environments and use cases.

For practical examples and getting started guidance, see:

```
vault-envrc-generator help vault-envrc-getting-started
```
