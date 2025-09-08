# Vault Envrc Generator — Generate env files from Vault

![](https://img.shields.io/github/license/go-go-golems/vault-envrc-generator)
![](https://img.shields.io/github/actions/workflow/status/go-go-golems/vault-envrc-generator/push.yml?branch=main)

> From Vault to `.envrc`, JSON, or YAML — safely and repeatably.

This CLI generates `.envrc`, JSON, and YAML files from HashiCorp Vault secrets. It supports single-path generation, batch processing with YAML configuration, Vault exploration, and reverse operations to populate Vault from local sources. Built on the Glazed framework with automatic KV version detection and multiple token resolution methods.

## Problem Statement

Working with HashiCorp Vault secrets across multiple environments involves several recurring challenges: manual extraction from the Vault web UI or complex CLI commands, inconsistent formatting across different applications and environments, repetitive tasks when managing multiple services or environments, and error-prone processes when secrets change or new services are added.

This tool addresses these issues by providing a configuration-driven approach to secret management. It automates the process of extracting secrets from Vault and converting them into the formats applications need, while supporting both simple single-path operations and complex multi-environment workflows.

## Core Features

- **Vault Integration**: Automatic KV engine version detection (v1/v2), multiple token resolution methods (environment variables, files, CLI integration), and connection health checking
- **Output Formats**: Generate `.envrc`, JSON, and YAML files with consistent formatting and optional key sorting for version control
- **Batch Processing**: Process multiple Vault paths using YAML configuration files with per-section filtering, transformation, and output settings
- **Reverse Operations**: Populate Vault with secrets from local sources including environment variables, files, and static data
- **Vault Exploration**: List and explore Vault contents with structured output and optional value censoring

## Installation

The tool can be installed through multiple methods depending on your environment and preferences.

### Pre-built Binaries (Recommended)

Download the latest release for your platform from [GitHub Releases](https://github.com/go-go-golems/vault-envrc-generator/releases):

```bash
# Linux/macOS - download and install
curl -L https://github.com/go-go-golems/vault-envrc-generator/releases/latest/download/vault-envrc-generator_linux_amd64.tar.gz | tar xz
sudo mv vault-envrc-generator /usr/local/bin/
vault-envrc-generator --version
```

### Package Managers

**Homebrew (macOS/Linux):**
```bash
brew install go-go-golems/tap/vault-envrc-generator
```

**Debian/Ubuntu:**
```bash
curl -s https://packagecloud.io/install/repositories/go-go-golems/main/script.deb.sh | sudo bash
sudo apt-get install vault-envrc-generator
```

### Go Install

```bash
go install github.com/go-go-golems/vault-envrc-generator/cmd/vault-envrc-generator@latest
```

### Build from Source

```bash
git clone https://github.com/go-go-golems/vault-envrc-generator.git
cd vault-envrc-generator
go build -ldflags "-w -s" -o vault-envrc-generator ./cmd/vault-envrc-generator
```

## Quick Start

Get up and running with the Vault Envrc Generator in minutes by following this essential workflow:

### 1. Configure Vault Connection

Set up your Vault connection using environment variables (the most common approach):

```bash
# Set your Vault server address
export VAULT_ADDR="https://vault.company.com:8200"

# Set authentication token (multiple methods supported)
export VAULT_TOKEN="hvs.CAESIGqjzSuHYTLSaI..."
# OR save to file: echo "your_token" > ~/.vault-token
# OR use vault CLI integration for automatic discovery
```

The tool supports multiple token resolution methods, automatically trying different sources including command-line flags, environment variables, token files, and Vault CLI integration.

### 2. Verify Connectivity

Test your connection and explore available secrets:

```bash
# Test connection and display Vault information
vault-envrc-generator test -v

# Explore available secret paths
vault-envrc-generator list --path secrets/ --format table

# Interactive exploration and generation
vault-envrc-generator interactive
```

### 3. Extract Your First Secrets

Start with simple single-path extraction to understand the tool's capabilities:

```bash
# Generate environment variables from database secrets
vault-envrc-generator generate \
  --path secrets/app/database \
  --format envrc \
  --prefix DB_ \
  --transform-keys \
  --output database.envrc

# Load the generated environment
source database.envrc
echo $DB_HOST  # Verify variables are available
```

### 4. Scale to Batch Processing

For complex applications requiring secrets from multiple Vault paths, use the batch processing feature:

```bash
# Process comprehensive environment configuration
vault-envrc-generator batch --config production.yaml --continue-on-error

# Preview batch operations without writing files
vault-envrc-generator batch --config production.yaml --dry-run
```

### 5. Populate Development Vault

Bootstrap your development environment by seeding Vault with necessary secrets:

```bash
# Preview what will be written to Vault
vault-envrc-generator seed --config dev-setup.yaml --dry-run

# Populate Vault with development secrets
vault-envrc-generator seed --config dev-setup.yaml
```

## Command Reference

The tool provides five specialized commands, each designed for specific workflows while sharing consistent parameter handling and output formatting.

### generate — Single Path Extraction

The `generate` command focuses on extracting secrets from a single Vault path with immediate output. It's perfect for quick secret extraction, debugging, shell scripting, and single-service applications.

```bash
# Basic usage with transformation and prefixing
vault-envrc-generator generate \
  --path secrets/app/database \
  --format envrc \
  --prefix DB_ \
  --transform-keys \
  --output database.envrc

# JSON output with key filtering for API consumption
vault-envrc-generator generate \
  --path secrets/api/keys \
  --format json \
  --include client_id,client_secret \
  --sort-keys \
  --output api-config.json
```

### batch — Multi-Path Processing

The `batch` command processes YAML configuration files that define multiple jobs with different transformation and output rules.

```yaml
# Example batch configuration
base_path: secrets/environments/production
jobs:
  - name: application-environment
    description: "Complete application configuration"
    output: config/app.envrc
    format: envrc
    sections:
      - name: database
        path: database/primary
        prefix: DB_
        transform_keys: true
        include_keys: [host, port, database, username, password]
      
      - name: redis-cache
        path: cache/redis
        env_map:
          REDIS_URL: connection_url
          REDIS_PASSWORD: auth_token
      
      - name: api-keys
        path: external/apis
        prefix: API_
        transform_keys: true
        fixed:
          API_VERSION: "v1"
          ENVIRONMENT: "production"
```

```bash
# Process complete batch configuration
vault-envrc-generator batch --config production.yaml

# Preview operations without writing files
vault-envrc-generator batch --config production.yaml --dry-run --output -
```

### list — Vault Discovery

The `list` command explores Vault contents with multiple output formats, useful for understanding secret organization and debugging access permissions.

```bash
# Explore secret structure with table output
vault-envrc-generator list --path secrets/app --format table --depth 3

# Generate documentation with censored values
vault-envrc-generator list \
  --path secrets/ \
  --format yaml \
  --include-values \
  --censor "***" \
  --output vault-inventory.yaml
```

### seed — Vault Population

The `seed` command reverses the typical flow by populating Vault with secrets from local sources, perfect for development setup and migration scenarios.

```yaml
# Example seed configuration
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
  
  - path: api-keys
    files:
      private_key: ~/.ssh/service_key
      certificate: ~/.ssl/service.crt
```

```bash
# Preview seed operations
vault-envrc-generator seed --config dev-setup.yaml --dry-run

# Execute seeding
vault-envrc-generator seed --config dev-setup.yaml
```

### interactive — Guided Exploration

The `interactive` command provides a user-friendly interface for learning the tool's capabilities and performing quick operations without complex configuration files.

## Key Concepts

- **KV Engines**: The tool automatically detects KV v2 (which wraps reads under `data/` and listings under `metadata/`) and falls back to KV v1 when needed
- **Token Resolution**: The `auto|env|file|lookup` strategy resolves tokens from command flags, environment variables, `~/.vault-token` file, or `vault token lookup`
- **Key Transformation**: The `transform_keys` option converts keys to UPPERCASE and replaces `-` with `_`; `prefix` adds a string prefix (e.g., `DB_`)
- **Output Semantics**: Envrc format appends sections with headers; JSON/YAML formats use shallow merge. Use `--sort-keys` for deterministic ordering
- **Batch Processing**: Jobs define defaults with per-section overrides, supporting different paths, filters, and transformations

## Documentation

For comprehensive guides and detailed configuration options:

```bash
# Project overview and command introduction
vault-envrc-generator help overview

# Complete installation and usage guide
vault-envrc-generator help vault-envrc-getting-started

# YAML configuration reference
vault-envrc-generator help yaml-configuration-reference

# Architecture and design details
vault-envrc-generator help vault-envrc-architecture
```

## Troubleshooting

- **403 errors on mount listing**: Expected for non-admin tokens; the tool avoids requiring `sys/mounts` access
- **403 errors on secret paths**: Indicates missing `list` or `read` permissions on that specific path
- **Connection issues**: Use `vault-envrc-generator test -v` to verify connectivity and token validity
- **Token lookup failures**: Ensure the `vault` CLI is installed and authenticated when using the `lookup` token source

## Complete Workflow Example

```bash
# 1) Seed development Vault with initial secrets
vault-envrc-generator seed --config dev-setup.yaml --dry-run
vault-envrc-generator seed --config dev-setup.yaml

# 2) Explore available secrets
vault-envrc-generator list --path secrets/environments/development/ \
  --format yaml --include-values --censor "***"

# 3) Generate complete environment configuration
vault-envrc-generator batch --config batch-dev.yaml --continue-on-error

# 4) Load generated environment
source .envrc
```

## Development

```bash
# Build and test
go build ./cmd/vault-envrc-generator
go test ./...
```

**Key packages:**
- `pkg/vault`: KV v2-first client with automatic fallback
- `pkg/envrc`: Output formatting and key transformations  
- `pkg/batch`: YAML configuration processing
- `pkg/output`: File merge and append operations
