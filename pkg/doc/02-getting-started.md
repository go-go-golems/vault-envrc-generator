---
Title: Getting Started — Vault Envrc Generator
Slug: vault-envrc-getting-started
Short: Complete guide to installation, configuration, and practical workflows for secret management
Topics:
- tutorial
- quick-start
- installation
- configuration
- workflows
IsTemplate: false
IsTopLevel: true
ShowPerDefault: true
SectionType: Tutorial
---

# Getting Started — Vault Envrc Generator

The Vault Envrc Generator transforms HashiCorp Vault secrets into environment variables and configuration files through a simple, configuration-driven approach. This guide takes you from installation through practical workflows, showing you how to extract secrets from Vault and generate the exact configuration formats your applications need.

## What You'll Learn

By the end of this guide, you'll understand how to:
- **Install and configure** the tool for your environment
- **Connect to Vault** using multiple authentication methods
- **Extract secrets** from single paths for quick tasks
- **Process multiple paths** using batch configurations
- **Generate different formats** (envrc, JSON, YAML) for various use cases
- **Integrate the tool** into development and deployment workflows

## Prerequisites and Setup

Before starting, ensure you have the necessary components and access:

**Required:**
- **Go 1.21+** for building from source
- **HashiCorp Vault instance** (local development server or production cluster)
- **Valid Vault token** with read access to your target secret paths
- **Command-line experience** with environment variables and basic shell operations

**Optional but Recommended:**
- **direnv** for automatic environment loading
- **jq** for JSON processing and token manipulation
- **vault CLI** for additional Vault operations

## Installation

See the repository README for installation methods (binaries, package managers, source build).

## Vault Connection Configuration

The tool provides flexible authentication options to work with different Vault deployments and security requirements. Understanding these options is crucial for smooth operation.

### Connection Parameters

The tool needs two pieces of information to connect to Vault:

**1. Vault Address** - Where your Vault server is located
**2. Authentication Token** - Proof of your identity and permissions

### Vault Address Configuration

Set your Vault server address using any of these methods:

```bash
# Method 1: Environment variable (recommended for consistency)
export VAULT_ADDR=https://vault.company.com:8200

# Method 2: Command-line flag (overrides environment)
vault-envrc-generator list --vault-addr https://vault.company.com:8200 --path secrets/

# Method 3: Configuration file (for complex setups)
echo "vault_addr: https://vault.company.com:8200" > ~/.vault-config.yaml
```

**Common Address Formats:**
- Local development: `http://127.0.0.1:8200`
- HTTPS with custom port: `https://vault.company.com:8200`
- HTTPS with standard port: `https://vault.company.com`

### Token Authentication Methods

The tool supports multiple token resolution strategies, automatically trying different sources until it finds a valid token.

#### Automatic Token Discovery (Recommended)

The default `auto` mode tries token sources in this order:

1. Command-line `--vault-token` flag
2. `VAULT_TOKEN` environment variable
3. `~/.vault-token` file
4. Vault agent socket (if available)

```bash
# Set up token via environment variable
export VAULT_TOKEN=hvs.CAESIGqjzSuHYTLSaI...

# Tool automatically discovers and uses the token
vault-envrc-generator list --path secrets/app
```

#### Environment Variable Method

For development and CI/CD environments:

```bash
# Set token in environment
export VAULT_TOKEN=hvs.CAESIGqjzSuHYTLSaI...

# Explicitly use environment token
vault-envrc-generator generate --path secrets/database \
  --vault-token-source env --format envrc
```

#### Token File Method

For persistent local development:

```bash
# Save token to file
echo "hvs.CAESIGqjzSuHYTLSaI..." > ~/.vault-token

# Use token file explicitly
vault-envrc-generator generate --path secrets/api \
  --vault-token-source file --format json
```

#### Custom Token File

For multiple environments or shared systems:

```bash
# Save environment-specific token
echo "hvs.DEV_TOKEN..." > ~/.vault-tokens/development

# Use custom token file
vault-envrc-generator batch --config dev.yaml \
  --vault-token-file ~/.vault-tokens/development
```

### Connection Verification

Before processing secrets, verify your connection works:

```bash
# Test connection and permissions
vault-envrc-generator list --path secrets/ --vault-addr $VAULT_ADDR

# Expected output: list of available secret paths
# If this fails, check your address and token configuration
```

**Troubleshooting Connection Issues:**

**"connection refused" errors:**
- Verify Vault address is correct and accessible
- Check if Vault server is running
- Confirm network connectivity and firewall rules

**"permission denied" errors:**
- Verify token has read permissions for target paths
- Check token expiration: `vault token lookup`
- Ensure you're using the correct Vault namespace (if applicable)

**"invalid token" errors:**
- Token may be expired or revoked
- Regenerate token: `vault auth` or request new token
- Verify token format (should start with `hvs.` for Vault 1.10+)

## Your First Secret Extraction

Let's start with a simple example to verify everything works correctly.

### Step 1: Explore Available Secrets

First, see what secrets are available in your Vault:

```bash
# List top-level secret paths
vault-envrc-generator list --path secrets/

# Explore a specific path
vault-envrc-generator list --path secrets/app/

# Show secret values (use carefully!)
vault-envrc-generator list --path secrets/app/database --show-values
```

### Step 2: Extract a Single Secret Path

Use the `generate` command for quick secret extraction:

```bash
# Generate environment variables from database secrets
vault-envrc-generator generate \
  --path secrets/app/database \
  --format envrc \
  --output database.envrc

# Review the generated file
cat database.envrc
```

**Expected Output:**
```bash
# Generated by vault-envrc-generator
export DATABASE_HOST="postgres.company.com"
export DATABASE_PORT="5432"
export DATABASE_NAME="myapp"
export DATABASE_USER="app_user"
export DATABASE_PASSWORD="secure_password_123"
```

### Step 3: Load and Use Environment Variables

```bash
# Load environment variables
source database.envrc

# Verify variables are set
echo $DATABASE_HOST
echo $DATABASE_PORT

# Use in your application
psql postgresql://$DATABASE_USER:$DATABASE_PASSWORD@$DATABASE_HOST:$DATABASE_PORT/$DATABASE_NAME
```

### Step 4: Generate Different Formats

The same secrets can be output in different formats for various use cases:

```bash
# JSON for API consumption
vault-envrc-generator generate \
  --path secrets/app/database \
  --format json \
  --sort-keys \
  --output database.json

# YAML for configuration files
vault-envrc-generator generate \
  --path secrets/app/database \
  --format yaml \
  --sort-keys \
  --output database.yaml

# Review different formats
cat database.json
cat database.yaml
```

## Key Transformation and Filtering

Real-world secret management often requires transforming key names and filtering sensitive data.

### Key Transformation

Transform Vault key names to standard environment variable format:

```bash
# Transform keys to UPPERCASE and replace hyphens with underscores
vault-envrc-generator generate \
  --path secrets/app/api-keys \
  --transform-keys \
  --format envrc

# Add prefix to avoid naming conflicts
vault-envrc-generator generate \
  --path secrets/shared/redis \
  --prefix "REDIS_" \
  --transform-keys \
  --format envrc
```

**Transformation Examples:**
- `api-key` → `API_KEY`
- `client-id` → `CLIENT_ID`
- `oauth_token` → `OAUTH_TOKEN`
- With prefix `REDIS_`: `host` → `REDIS_HOST`

### Key Filtering

Filter secrets to include only what your application needs:

```bash
# Include only specific keys
vault-envrc-generator generate \
  --path secrets/app/database \
  --include host,port,database,username,password \
  --format envrc

# Exclude sensitive keys for development
vault-envrc-generator generate \
  --path secrets/app/config \
  --exclude "*_prod,*_production,admin_*" \
  --format envrc

# Combine filtering with transformation
vault-envrc-generator generate \
  --path secrets/api/oauth \
  --include client_id,client_secret \
  --prefix "OAUTH_" \
  --transform-keys \
  --format envrc
```

## Batch Processing (basics)

The `batch` command processes multiple Vault paths and produces one output per job.

### Understanding Batch Configuration

Batch processing uses YAML configuration files to define complex workflows. Here's a practical example:

```yaml
# app-config.yaml
base_path: secrets/environments/production/app

jobs:
  - name: database-config
    description: "Database connection settings"
    output: config/database.envrc
    format: envrc
    sections:
      - name: primary-db
        path: database/primary
        prefix: DB_PRIMARY_
        transform_keys: true
        include_keys: [host, port, database, username, password]
      
      - name: replica-db
        path: database/replica
        prefix: DB_REPLICA_
        transform_keys: true
        include_keys: [host, port, database, username, password]

  - name: api-keys
    description: "External service API keys"
    output: config/api-keys.json
    format: json
    sort_keys: true
    sections:
      - name: openai
        path: providers/openai
        prefix: OPENAI_
        transform_keys: true
      
      - name: stripe
        path: providers/stripe
        prefix: STRIPE_
        transform_keys: true
```

### Run a batch file

```bash
# Process complete configuration
vault-envrc-generator batch --config app-config.yaml

# Preview without writing files
vault-envrc-generator batch --config app-config.yaml --dry-run

# Override output directory
vault-envrc-generator batch --config app-config.yaml --output /tmp/test-config

# Continue processing on errors
vault-envrc-generator batch --config app-config.yaml --continue-on-error
```

### Multi-environment with templates

```yaml
# multi-env.yaml
base_path: secrets/environments/{{ .Token.Meta.environment }}/app

jobs:
  - name: app-config
    output: config/{{ .Token.Meta.environment }}.envrc
    format: envrc
    sections:
      - name: database
        path: database
        prefix: DB_
        transform_keys: true
      
      - name: cache
        path: redis
        prefix: REDIS_
        transform_keys: true
```

Template variables commonly used:
- `{{ .Token.OIDCUserID }}`
- `{{ .Token.Meta.environment }}`

## Seeding Vault (basics)

Write secrets to Vault from local sources using a YAML spec:

```yaml
# seed-dev.yaml
base_path: secrets/environments/development
sets:
  - path: database
    data:
      provider: "postgresql"
      port: "5432"
    env:
      host: DB_HOST
      username: DB_USER
      password: DB_PASSWORD
  - path: certificates
    files:
      ca_cert: ~/.ssl/ca.pem
      server_key: ~/.ssl/localhost.key
```

Dry run, then seed:

```bash
vault-envrc-generator seed --config seed-dev.yaml --dry-run
vault-envrc-generator seed --config seed-dev.yaml
```

## Next steps

For details and complete references:

```bash
vault-envrc-generator help yaml-configuration-reference   # batch YAML
vault-envrc-generator help seed-configuration-guide       # seed YAML
vault-envrc-generator help vault-envrc-architecture       # internals
```

## Troubleshooting Common Issues

### Connection Problems

**Issue: "connection refused"**
```bash
# Check Vault address
echo $VAULT_ADDR
curl -k $VAULT_ADDR/v1/sys/health

# Test with explicit address
vault-envrc-generator list --vault-addr http://127.0.0.1:8200 --path secrets/
```

**Issue: "certificate verification failed"**
```bash
# For development with self-signed certificates
export VAULT_SKIP_VERIFY=true

# Or provide CA certificate
export VAULT_CACERT=/path/to/ca.pem
```

### Authentication Problems

**Issue: "permission denied"**
```bash
# Check token permissions
vault token lookup

# Verify path access
vault kv get secrets/your/path

# Check token policies
vault token capabilities secrets/your/path
```

**Issue: "token expired"**
```bash
# Renew token if renewable
vault token renew

# Or authenticate again
vault auth -method=userpass username=yourusername
```

### Configuration Problems

**Issue: "path not found"**
```bash
# List available paths
vault-envrc-generator list --path secrets/

# Check path structure
vault kv list secrets/
```

**Issue: "template rendering failed"**
```bash
# Test template variables
vault token lookup -format=json | jq '.data.meta'

# Use simpler paths without templates for testing
```

## Best Practices

### Security Best Practices

1. **Token Management**:
   - Use short-lived tokens when possible
   - Store tokens securely (not in version control)
   - Rotate tokens regularly
   - Use least-privilege policies

2. **Output Security**:
   - Set appropriate file permissions on generated files
   - Clean up temporary files containing secrets
   - Use `.gitignore` to prevent accidental commits
   - Consider encrypting output files for storage

3. **Access Control**:
   - Use specific Vault policies for different environments
   - Implement token renewal strategies
   - Monitor token usage and access patterns

### Operational Best Practices

1. **Configuration Management**:
   - Version control your batch configuration files
   - Use environment-specific configurations
   - Document your secret organization structure
   - Test configurations in development before production

2. **Automation**:
   - Integrate into CI/CD pipelines
   - Use health checks to verify secret availability
   - Implement rollback procedures for failed deployments
   - Monitor secret usage and rotation

3. **Development Workflow**:
   - Use consistent naming conventions
   - Organize secrets logically in Vault
   - Document required secrets for each application
   - Provide example configurations for new developers

This comprehensive guide provides everything you need to start using the Vault Envrc Generator effectively. For detailed configuration options and advanced features, see:

```
glaze help yaml-configuration-reference
glaze help vault-envrc-architecture
```
