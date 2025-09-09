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

Seed configurations define target Vault paths and data sources. Supported sources: `data` (static), `env` (environment variables), `files` (file contents).

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
      database_config: ~/config/database.json
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

  - path: monitoring/config
    json_files:
      # Extract monitoring endpoints from complex config
      endpoints:
        file: ~/config/monitoring.json
        transforms:
          prometheus_url: "services.prometheus.endpoint"
          grafana_url: "services.grafana.endpoint"
          alertmanager_url: "services.alertmanager.endpoint"
          first_server: "servers.0.hostname"  # Array indexing
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

  - path: helm/values
    yaml_files:
      # Extract Helm chart values
      chart_config:
        file: ~/charts/myapp/values.yaml
        transforms:
          replicas: "replicaCount"
          image_tag: "image.tag"
          db_host: "postgresql.host"
          redis_url: "redis.url"

  - path: ci/config
    yaml_files:
      # Extract CI/CD pipeline configuration
      pipeline_config:
        file: ~/.github/workflows/deploy.yml
        transforms:
          docker_registry: "env.DOCKER_REGISTRY"
          k8s_cluster: "env.KUBERNETES_CLUSTER"
          deploy_branch: "on.push.branches.0"
```

Format support: standard YAML; multi-document files use first document; anchors and aliases resolved.

Behavior: identical to JSON transforms; YAML-specific features preserved during parsing.

Use cases: Kubernetes manifests, Helm values, CI/CD configs, Docker Compose files.

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

Rules: merged into one secret; key conflicts error; processing order `data` → `env` → `files` → `json_files` → `yaml_files`; empty values are written.

## Template integration

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

### Dynamic path examples

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

## Examples

### Development environment bootstrap

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

### Migration from environment variables

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

### Certificate and key management

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

### JSON/YAML configuration extraction

```yaml
base_path: secrets/config/{{ .Token.Meta.environment }}

sets:
  # Extract database configuration from app config file
  - path: database/primary
    json_files:
      db_config:
        file: ~/config/app.json
        transforms:
          host: "database.primary.host"
          port: "database.primary.port"
          name: "database.primary.database"
          username: "database.primary.username"
          password: "database.primary.password"
          ssl_mode: "database.primary.ssl.mode"
          max_connections: "database.primary.pool.max_connections"

  # Extract service credentials from Google Cloud service account
  - path: gcp/service-account
    json_files:
      gcp_credentials:
        file: ~/credentials/gcp-service-account.json
        transforms:
          project_id: "project_id"
          client_id: "client_id"
          client_email: "client_email"
          private_key_id: "private_key_id"
          private_key: "private_key"

  # Extract Kubernetes deployment configuration
  - path: k8s/deployment
    yaml_files:
      app_deployment:
        file: ~/k8s/app-deployment.yaml
        transforms:
          namespace: "metadata.namespace"
          app_name: "metadata.labels.app"
          image: "spec.template.spec.containers.0.image"
          replicas: "spec.replicas"
          service_account: "spec.template.spec.serviceAccountName"

  # Extract Helm values for chart deployment
  - path: helm/values
    yaml_files:
      chart_values:
        file: ~/charts/myapp/values.yaml
        transforms:
          image_tag: "image.tag"
          image_repository: "image.repository"
          ingress_host: "ingress.hosts.0.host"
          storage_class: "persistence.storageClass"
          storage_size: "persistence.size"

  # Extract monitoring configuration from complex JSON
  - path: monitoring/config
    json_files:
      monitoring_setup:
        file: ~/config/monitoring.json
        transforms:
          prometheus_url: "services.prometheus.external_url"
          grafana_admin_password: "services.grafana.security.admin_password"
          alertmanager_webhook: "services.alertmanager.route.receiver"
          first_target: "scrape_configs.0.static_configs.0.targets.0"

  # Combine multiple sources for complete application config
  - path: app/complete
    data:
      environment: "{{ .Token.Meta.environment }}"
      deployed_by: "{{ .Token.DisplayName }}"
    env:
      secret_key: APP_SECRET_KEY
      session_secret: SESSION_SECRET
    json_files:
      app_config:
        file: ~/config/app.json
        transforms:
          debug_mode: "debug"
          log_level: "logging.level"
          cache_ttl: "cache.default_ttl"
    yaml_files:
      k8s_config:
        file: ~/k8s/configmap.yaml
        transforms:
          api_version: "data.API_VERSION"
          feature_flags: "data.FEATURE_FLAGS"
```

## Operations

### Dry run

```bash
# Preview all operations
vault-envrc-generator seed --config development-setup.yaml --dry-run

# Preview with detailed logging
vault-envrc-generator seed --config development-setup.yaml --dry-run --log-level debug
```

Dry run shows target paths, key counts, rendered templates, file/env resolution, and validation issues.

### Incremental updates

```bash
# Initial seed
vault-envrc-generator seed --config base-setup.yaml

# Add additional secrets
vault-envrc-generator seed --config additional-secrets.yaml

# Update existing secrets
vault-envrc-generator seed --config updated-config.yaml
```

### Validation and testing

```bash
# Test configuration parsing
vault-envrc-generator seed --config production-seed.yaml --dry-run

# Verify all files and environment variables are available
vault-envrc-generator seed --config production-seed.yaml --dry-run --log-level debug

# Test against development Vault first
VAULT_ADDR=http://dev-vault:8200 vault-envrc-generator seed --config production-seed.yaml --dry-run
```

## Security

### File permissions

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

### Audit

```bash
# Enable audit logging
vault-envrc-generator seed --config production-seed.yaml --log-level info

# Review audit logs in Vault
vault audit list
```

### Data validation

```bash
# Verify file contents before seeding
cat ~/.ssl/server.crt | openssl x509 -text -noout

# Check environment variables
echo $DATABASE_PASSWORD | wc -c  # Verify password length

# Validate JSON configuration files
jq . ~/config/app.json
```

## Troubleshooting

### File not found
```bash
# Verify file paths
ls -la ~/.ssl/server.crt
ls -la ~/config/app.json

# Check file permissions
stat ~/.ssl/server.key
```

### Environment variables
```bash
# Verify environment variables are set
env | grep DATABASE_PASSWORD
printenv OPENAI_API_KEY

# Check for typos in variable names
vault-envrc-generator seed --config config.yaml --dry-run --log-level debug
```

### Template errors
```bash
# Check token information
vault token lookup -format=json

# Verify metadata availability
vault token lookup -format=json | jq '.data.meta'

# Test template rendering
vault-envrc-generator seed --config config.yaml --dry-run
```

### Vault permissions
```bash
# Check token capabilities
vault token capabilities secrets/environments/development/database

# Verify path permissions
vault kv get secrets/environments/development/test
```

For practical examples and getting started, see:

```
vault-envrc-generator help vault-envrc-getting-started
```
