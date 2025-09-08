---
Title: Architecture Overview — Vault Envrc Generator
Slug: architecture
Short: Deep dive into the system architecture, design patterns, and component interactions
Topics:
- architecture
- design-patterns
- glazed
- vault-client
- layers
- commands
IsTemplate: false
IsTopLevel: true
ShowPerDefault: true
SectionType: GeneralTopic
---

# Architecture Overview — Vault Envrc Generator

The Vault Envrc Generator is designed around a layered architecture that separates concerns cleanly while providing maximum flexibility for secret management workflows. Built on the **Glazed** CLI framework, it transforms the complex task of managing HashiCorp Vault secrets into a streamlined, configuration-driven process that scales from simple single-path operations to sophisticated multi-environment batch processing.

## System Architecture Philosophy

The architecture follows three core principles that drive its design and implementation:

**1. Separation of Concerns**: Each layer has a single, well-defined responsibility. The CLI layer handles user interaction and parameter validation, the business logic layer processes secrets and generates output, and the Vault integration layer manages all HashiCorp Vault communication. This separation makes the system easier to test, maintain, and extend.

**2. Configuration-Driven Operations**: Rather than requiring complex command-line invocations, the system uses YAML configuration files to define sophisticated workflows. This approach makes operations repeatable, versionable, and easier to understand. A single configuration file can define dozens of secret extraction and transformation operations.

**3. Extensible Plugin Architecture**: Built on Glazed's parameter layer system, the application can easily incorporate new functionality without breaking existing workflows. New output formats, transformation rules, and secret sources can be added through well-defined interfaces.

## High-Level Component Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                          User Interface Layer                       │
│                                                                     │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐    │
│  │   batch     │ │  generate   │ │    seed     │ │    list     │    │
│  │  command    │ │  command    │ │  command    │ │  command    │    │
│  └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘    │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        Glazed Framework Layer                       │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────────┐ │
│  │                   Parameter Management                          │ │
│  │  • Vault Connection Layer (authentication, endpoints)          │ │
│  │  • Logging Configuration Layer (structured output)             │ │
│  │  • Command-Specific Settings (validation, defaults)            │ │
│  └─────────────────────────────────────────────────────────────────┘ │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────┐
│                       Business Logic Layer                          │
│                                                                     │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐    │
│  │   Batch     │ │   Format    │ │    Seed     │ │   Output    │    │
│  │ Processor   │ │  Generator  │ │   Runner    │ │  Manager    │    │
│  │             │ │             │ │             │ │             │    │
│  │ Multi-path  │ │ envrc/JSON  │ │ Local→Vault │ │ File/stdout │    │
│  │ workflows   │ │ YAML output │ │ population  │ │ handling    │    │
│  └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘    │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      Vault Integration Layer                        │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────────┐ │
│  │                    Intelligent Vault Client                    │ │
│  │                                                                 │ │
│  │  • KV Engine Detection (v1/v2 auto-detection)                 │ │
│  │  • Token Resolution (environment, files, CLI lookup)          │ │
│  │  • Connection Health & Retry Logic                            │ │
│  │  • Template Rendering (dynamic path construction)             │ │
│  │  • Path Validation & Security (injection prevention)          │ │
│  │                                                                 │ │
│  └─────────────────────────────────────────────────────────────────┘ │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

## Command Layer Architecture

The command layer provides the primary user interface and orchestrates all system operations. Each command is designed for specific workflows while sharing common infrastructure through the Glazed framework.

### Batch Command (`cmds/batch.go`)

The batch command is the most sophisticated component, designed for complex multi-environment secret management workflows. It processes YAML configuration files that define multiple jobs, each containing multiple sections with different processing rules.

**Core Capabilities:**
- **Multi-Job Processing**: A single configuration file can define dozens of separate output operations
- **Section-Based Organization**: Within each job, secrets can be grouped into logical sections with different filtering and transformation rules
- **Flexible Output Aggregation**: Sections can be merged (JSON/YAML) or concatenated with headers (envrc)
- **Error Resilience**: Continue-on-error support ensures partial failures don't stop entire workflows

**Real-World Usage Patterns:**
```bash
# Process complete development environment
./vault-envrc-generator batch -c environments/dev.yaml

# Override output locations for testing
./vault-envrc-generator batch -c prod.yaml --output /tmp/test --dry-run

# Generate multiple format outputs simultaneously
./vault-envrc-generator batch -c multi-env.yaml --format json
```

**Internal Architecture:**
The batch command follows a pipeline pattern where configuration parsing, Vault client initialization, job processing, and output generation are separate, composable stages. This makes it easy to add new features like parallel processing or different aggregation strategies.

### Generate Command (`cmds/generate.go`)

The generate command focuses on single-path operations with immediate output. It's designed for quick secret extraction and simple transformation workflows.

**Core Capabilities:**
- **Single-Path Focus**: Optimized for extracting secrets from one Vault path
- **Immediate Output**: Results can go directly to stdout or files without intermediate processing
- **Interactive Friendly**: Perfect for shell scripts and interactive exploration
- **Format Flexibility**: Supports envrc, JSON, and YAML output with consistent formatting

**Design Philosophy:**
While the batch command handles complex workflows, generate command prioritizes simplicity and speed. It's the tool you reach for when you need a quick secret extraction or want to explore what's available at a specific path.

**Usage Examples:**
```bash
# Quick environment variable extraction
./vault-envrc-generator generate --path secrets/app/database --format envrc

# Filtered JSON output for API consumption
./vault-envrc-generator generate --path secrets/api/keys \
  --include client_id,client_secret --format json --sort-keys

# Prefixed environment variables for service isolation
./vault-envrc-generator generate --path secrets/redis \
  --prefix REDIS_ --transform-keys --output redis.envrc
```

### Seed Command (`cmds/seed.go`)

The seed command reverses the typical flow by populating Vault with secrets from local sources. This is essential for development workflows and initial system setup.

**Core Capabilities:**
- **Multi-Source Ingestion**: Combines static data, environment variables, and file contents
- **Template-Driven Paths**: Uses the same template system as other commands for dynamic path construction
- **Dry-Run Support**: Preview operations without actually writing to Vault
- **Validation**: Ensures all required data sources are available before starting

**Workflow Integration:**
The seed command is typically used at the beginning of development workflows to populate Vault with necessary secrets, then other commands extract and transform these secrets for application use.

### List Command (`cmds/list.go`)

The list command provides discovery and exploration capabilities, essential for understanding what secrets are available and how they're organized.

**Core Capabilities:**
- **Recursive Traversal**: Explores Vault hierarchies to any depth
- **Structured Output**: Uses Glazed's output system for consistent formatting
- **Value Censoring**: Can hide sensitive values while showing structure
- **Depth Control**: Limits traversal depth for large hierarchies

**Exploration Workflow:**
The list command is invaluable during initial system exploration and debugging. It helps developers understand how secrets are organized and what data is available for their applications.

## Glazed Framework Integration

The Glazed framework provides the foundation for consistent parameter handling, output formatting, and extensibility across all commands. This integration is what makes the application feel cohesive despite its complexity.

### Parameter Layer System

The parameter layer system allows different aspects of configuration to be managed independently while composing seamlessly at runtime. This creates a clean separation between Vault-specific settings, logging configuration, and command-specific options.

**Vault Parameter Layer (`pkg/vaultlayer/`)**:
This layer encapsulates all Vault connection settings, making them reusable across commands. It handles:
- **Connection Configuration**: Vault address, namespace, and timeout settings
- **Authentication Parameters**: Token sources, file paths, and authentication methods
- **Security Settings**: TLS configuration and certificate validation
- **Template Variables**: Context for dynamic path construction

**Benefits of This Approach:**
- **Consistency**: All commands use identical Vault connection logic
- **Reusability**: Connection settings can be shared across different operations
- **Testability**: Vault interactions can be mocked at the layer boundary
- **Extensibility**: New authentication methods can be added without changing command code

### Structured Output System

Glazed's output system ensures that all commands produce consistent, machine-readable output when requested. This makes the tool suitable for both interactive use and automation workflows.

**Output Format Support:**
- **Table Format**: Human-readable output for interactive exploration
- **JSON Format**: Structured data for API consumption and further processing
- **YAML Format**: Configuration-friendly output for documentation
- **CSV Format**: Spreadsheet-compatible output for analysis

## Vault Integration Architecture

The Vault integration layer abstracts the complexity of HashiCorp Vault's API while providing intelligent behavior for common operations. This layer is where much of the application's robustness comes from.

### Intelligent Client Wrapper (`pkg/vault/client.go`)

The Vault client wrapper provides a simplified interface over Vault's complex API while handling edge cases and providing intelligent defaults.

**KV Engine Auto-Detection:**
One of the most valuable features is automatic detection of KV engine versions. Vault's KV v1 and v2 engines have different API paths and data structures, but the client automatically detects which version is in use and adapts accordingly.

```go
// The client tries KV v2 first, falls back to v1
func (c *Client) GetSecrets(path string) (map[string]interface{}, error) {
    mountPath, secretPath := c.parsePath(path)
    
    // Try KV v2 first (most common)
    if data, err := c.getKVv2Secrets(mountPath, secretPath); err == nil {
        return data, nil
    }
    
    // Fallback to KV v1 if v2 fails
    return c.getKVv1Secrets(path)
}
```

**Connection Health Management:**
The client performs health checks before operations and provides detailed error messages when connections fail. This prevents cryptic failures and helps with debugging connection issues.

**Path Intelligence:**
The client understands Vault's path structure and can intelligently separate mount paths from secret paths, handle trailing slashes, and validate path formats.

### Token Resolution System (`pkg/vault/token_loader.go`)

Token resolution is one of the most complex aspects of Vault integration, and the system provides multiple strategies to handle different deployment scenarios.

**Resolution Strategies:**
1. **Explicit Tokens**: Directly provided via command line or configuration
2. **Environment Variables**: Standard `VAULT_TOKEN` environment variable
3. **Token Files**: Home directory `.vault-token` file or custom paths
4. **CLI Lookup**: Integration with `vault` CLI for token discovery

**Security Considerations:**
The token resolution system never logs tokens or exposes them in error messages. It also supports token validation to ensure tokens are valid before attempting operations.

### Template Rendering System (`pkg/vault/templates.go`)

The template system enables dynamic path construction using Go's template syntax. This is essential for multi-user and multi-environment scenarios.

**Template Context:**
Templates receive rich context information extracted from the current Vault token:

```go
type TemplateContext struct {
    Token TokenContext
}

type TokenContext struct {
    OIDCUserID  string                // Extracted from OIDC tokens
    DisplayName string                // Human-readable token name
    EntityID    string                // Vault entity identifier
    Meta        map[string]string     // Custom token metadata
    Policies    []string              // Assigned policies
}
```

**Usage Examples:**
```yaml
# User-specific paths
base_path: secrets/personal/{{ .Token.OIDCUserID }}/config

# Environment-based paths using metadata
base_path: secrets/{{ .Token.Meta.environment }}/shared

# Conditional paths
base_path: secrets/{{- if eq .Token.Meta.role "admin" }}admin{{- else }}user{{- end }}
```

## Business Logic Components

The business logic layer contains the core processing engines that transform Vault secrets into usable configuration formats. Each component is designed for specific use cases while maintaining consistent interfaces.

### Batch Processing Engine (`pkg/batch/`)

The batch processing engine is the most complex component, designed to handle sophisticated multi-step workflows with error recovery and flexible output generation.

**Processing Pipeline:**
1. **Configuration Parsing**: YAML configurations are parsed and validated
2. **Template Resolution**: All template strings are rendered with current context
3. **Job Orchestration**: Jobs are processed sequentially with shared state
4. **Section Processing**: Within jobs, sections are processed with individual settings
5. **Data Aggregation**: Section outputs are combined according to format rules
6. **Output Generation**: Final content is generated and written to specified destinations

**Section Processing Details:**
Each section within a job can have completely different processing rules:
- **Path Targeting**: Different Vault paths for different types of secrets
- **Key Filtering**: Include/exclude patterns for fine-grained control
- **Transformation Rules**: Per-section prefix and key transformation settings
- **Environment Mapping**: Direct mapping from Vault keys to environment variable names
- **Static Data Injection**: Fixed key-value pairs added to section output

**Error Handling Strategy:**
The batch processor supports continue-on-error mode, where individual section failures don't stop the entire job. This is crucial for production workflows where partial success is better than complete failure.

### Format Generation Engine (`pkg/envrc/`)

The format generation engine handles the conversion of raw Vault data into usable configuration formats. It's designed around a strategy pattern that makes adding new formats straightforward.

**Format-Specific Processing:**

**Envrc Format:**
- **Shell Escaping**: Proper escaping of special characters for shell safety
- **Export Generation**: Automatic `export` statement generation
- **Comment Headers**: Section headers for organization and readability
- **Deterministic Output**: Consistent key ordering for version control

**JSON Format:**
- **Structured Output**: Proper JSON structure with nested objects
- **Key Sorting**: Optional alphabetical key sorting for consistency
- **Pretty Printing**: Formatted output for human readability

**YAML Format:**
- **Configuration Friendly**: Output suitable for application configuration files
- **Comment Preservation**: Section comments carried through to output
- **Nested Structure Support**: Complex data structures maintained

**Key Transformation Engine:**
The transformation engine handles the conversion of Vault key names to environment variable names:
- **Case Conversion**: Lowercase to UPPERCASE transformation
- **Character Replacement**: Hyphens to underscores, special character handling
- **Prefix Application**: Consistent prefix application with separator handling
- **Conflict Resolution**: Handling of duplicate keys after transformation

### Seed Processing Engine (`pkg/seed/`)

The seed processing engine handles the reverse flow, taking local data and populating Vault. It's designed for development workflows and initial system setup.

**Data Source Integration:**
- **Static Data**: Direct key-value pairs from configuration
- **Environment Variables**: Runtime environment variable capture
- **File Content**: File reading with path expansion and error handling
- **Template Processing**: Dynamic path and value generation

**Processing Workflow:**
1. **Source Resolution**: All data sources are resolved and validated
2. **Path Template Rendering**: Target paths are rendered with current context
3. **Data Combination**: Multiple sources are merged into final data sets
4. **Vault Writing**: Data is written to Vault with appropriate error handling
5. **Verification**: Optional verification of written data

## Data Flow Architecture

Understanding how data flows through the system is crucial for debugging and extending functionality. The system uses a pipeline architecture where data is transformed at each stage.

### Batch Processing Data Flow

The batch processing flow is the most complex, involving multiple transformation stages:

1. **Configuration Ingestion**: YAML files are parsed into structured configuration objects
2. **Template Context Building**: Vault token information is extracted and structured for template rendering
3. **Path Resolution**: All template strings in paths are rendered with current context
4. **Vault Client Initialization**: Connection is established and health-checked
5. **Job Iteration**: Each job is processed sequentially with shared client
6. **Section Processing**: Within jobs, sections are processed with individual settings
7. **Secret Retrieval**: Vault API calls retrieve secret data for each section
8. **Data Transformation**: Keys are filtered, transformed, and prefixed according to section rules
9. **Output Aggregation**: Section outputs are combined according to format-specific rules
10. **File Generation**: Final content is written to specified output destinations

**Error Propagation:**
Errors at each stage are wrapped with context information, making it easy to identify where problems occur. The continue-on-error feature allows processing to continue even when individual sections fail.

### Template Processing Flow

Template processing occurs at multiple points in the data flow:

1. **Context Building**: Token information is extracted and structured
2. **Path Template Rendering**: Configuration paths are rendered first
3. **Value Template Rendering**: Individual values can contain templates
4. **Validation**: Rendered templates are validated for security and correctness

## Security Architecture

Security is woven throughout the system architecture, with multiple layers of protection against common vulnerabilities.

### Token Security

**Token Handling:**
- **No Logging**: Tokens are never logged or exposed in error messages
- **Memory Protection**: Tokens are cleared from memory when no longer needed
- **Validation**: Tokens are validated before use to prevent unnecessary operations
- **Source Flexibility**: Multiple token sources provide deployment flexibility

### Path Injection Prevention

**Template Security:**
- **Validation**: All template strings are validated before rendering
- **Sandboxing**: Template execution is sandboxed to prevent code injection
- **Path Validation**: Rendered paths are validated against expected patterns
- **Error Handling**: Template errors are caught and reported safely

### Value Protection

**Output Security:**
- **Shell Escaping**: Proper escaping prevents injection attacks in envrc output
- **Censoring Support**: Sensitive values can be censored in list output
- **File Permissions**: Output files are created with appropriate permissions
- **Atomic Writes**: Output files are written atomically to prevent partial updates

## Performance Considerations

The architecture is designed for efficiency while maintaining flexibility and reliability.

### Connection Management

**Client Reuse:**
- **Single Client**: One Vault client per command execution reduces connection overhead
- **Health Checking**: Connection health is verified before operations
- **Retry Logic**: Intelligent retry logic handles transient failures
- **Timeout Management**: Configurable timeouts prevent hanging operations

### Memory Management

**Streaming Processing:**
- **Minimal Buffering**: Large outputs are streamed rather than buffered in memory
- **Efficient Data Structures**: Optimized data structures for common operations
- **Garbage Collection**: Explicit memory management for long-running operations

### API Efficiency

**Batch Operations:**
- **Minimal API Calls**: Multiple secrets retrieved in single operations where possible
- **Intelligent Caching**: Results cached within single command execution
- **Path Optimization**: Related paths processed together to reduce round trips

This architecture provides a robust foundation for Vault secret management that scales from simple single-path operations to complex multi-environment workflows. The clear separation of concerns, comprehensive error handling, and security-first design make it suitable for both development and production use cases.
