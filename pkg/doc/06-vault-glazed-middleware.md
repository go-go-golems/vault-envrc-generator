---
Title: Vault Glazed Middleware
Slug: vault-glazed-middleware
Short: Load parameter values from HashiCorp Vault using a Glazed middleware.
Topics:
- middlewares
- vault
- configuration
IsTemplate: false
IsTopLevel: false
ShowPerDefault: true
SectionType: GeneralTopic
---

# Vault Glazed Middleware

## 1. Overview

Glazed supports composing parameter sources using middlewares (see `glaze help middlewares-guide`). This page explains how to populate parameter values from HashiCorp Vault with a middleware, how to bootstrap Vault connection settings early, how precedence works, and how to combine Vault with config files, environment variables, and command-line flags. The goal is to make secrets feel like “just another source” in your parameter chain while keeping behavior predictable and debuggable.

## 2. What You’ll Build

- A middleware that fetches key/value pairs from Vault and updates parameters with matching names.
- A bootstrap sub-chain that parses only the `vault` layer first so the middleware can connect.
- A small example command demonstrating end-to-end usage.

## 3. Key Concepts

- **Vault layer**: A reusable parameter layer providing `vault-addr`, `vault-token`, `vault-token-file`, and `vault-token-source`.
- **Early bootstrapping**: Restrict middlewares to the `vault` layer to obtain connection info before loading other parameters.
- **Source tracking**: Pass `parameters.WithParseStepSource("vault")` so parameter history shows where values came from.
- **Mapping by name**: A Vault secret key updates a parameter when the names match (e.g., secret key `api-key` updates parameter `api-key`). If no secret exists with that name, nothing is changed.
- **KV engine support**: The client auto-detects KV v2 (using `mount/data/path`) and falls back to KV v1 transparently.
- **Templated paths**: Paths can use Go templates with token metadata: `kv/{{ .Token.OIDCUserID }}/config`.

## 4. Middleware API

The middleware is provided by this package:

```go
// package: github.com/go-go-golems/vault-envrc-generator/pkg/glazed
func UpdateFromVault(path string, options ...parameters.ParseStepOption) middlewares.Middleware
```

- **path**: A Vault KV path (supports templates like `kv/{{ .Token.OIDCUserID }}/config`).
- **behavior**: For each layer/parameter, if a Vault secret key matches the parameter name, the value is set on the parsed layer.

### 4.1. How values are applied

- The middleware iterates all layers and their parameter definitions.
- If a secret value exists for a parameter name, the parsed layer is updated with that value (using the provided `ParseStepOption`s).
- This means Vault can populate multiple layers at once (for example, `default`, `api`, `db`).

### 4.2. Path templating

Token metadata is available via `{{ .Token.* }}`. Common fields:
- `DisplayName`, `EntityID`, `Path`, `TTL`, `Type`, `Policies`, `Meta` (key/value), and derived `OIDCUserID` when `display_name` starts with `oidc-...`.

Example:

```go
glazed.UpdateFromVault("kv/{{ .Token.OIDCUserID }}/app", parameters.WithParseStepSource("vault"))
```

## 5. Recommended Ordering

To ensure `vault-addr`/token are available, parse only the `vault` layer first, then run `UpdateFromVault`, then the rest.

```go
mw := []middlewares.Middleware{
    // Highest-priority CLI sources collected first
    middlewares.ParseFromCobraCommand(cmd, parameters.WithParseStepSource("flags")),
    middlewares.GatherArguments(args, parameters.WithParseStepSource("args")),

    // 1) Bootstrap ONLY the vault layer
    middlewares.WrapWithWhitelistedLayers(
        []string{vaultlayer.VaultLayerSlug},
        middlewares.SetFromDefaults(parameters.WithParseStepSource("defaults")),
        middlewares.GatherFlagsFromViper(parameters.WithParseStepSource("viper")),
        middlewares.ParseFromCobraCommand(cmd, parameters.WithParseStepSource("flags")),
    ),

    // 2) Load values from Vault using parsed vault settings
    glazed.UpdateFromVault("kv/{{ .Token.OIDCUserID }}/app", parameters.WithParseStepSource("vault")),

    // 3) Normal middlewares for all layers
    middlewares.GatherFlagsFromViper(parameters.WithParseStepSource("viper")),
    middlewares.UpdateFromEnv("APP", parameters.WithParseStepSource("env")),
    middlewares.SetFromDefaults(parameters.WithParseStepSource("defaults")),
}
```

Tips:
- Wrap `UpdateFromVault` with `WrapWithWhitelistedLayers` if you only want to fill specific layers.
- The token resolver supports env/file/lookup (`~/.vault-token`, `VAULT_TOKEN`, `vault token lookup`).
- If you need `VAULT_ADDR` or `VAULT_TOKEN_FILE` to flow into the `vault` layer before templating, you can map environment values into the vault layer up front:

```go
middlewares.WrapWithWhitelistedLayers(
    []string{vaultlayer.VaultLayerSlug},
    middlewares.UpdateFromMapFirst(map[string]map[string]interface{}{
        vaultlayer.VaultLayerSlug: {
            "vault-addr":       os.Getenv("VAULT_ADDR"),
            "vault-token-file": os.Getenv("VAULT_TOKEN_FILE"),
        },
    }, parameters.WithParseStepSource("env")),
)
```

### 5.1. Precedence and overrides

Middlewares run in reverse wrapping order (see `middlewares.ExecuteMiddlewares`). A practical pattern is:
- Defaults (lowest precedence)
- Config files and Viper
- Vault
- CLI flags/args (highest precedence)

Adjust to your needs: place `UpdateFromVault` before flags if flags should win over Vault; place it after Viper if Vault should override config files.

## 6. Minimal Example

See `cmd/examples/vault-glaze-example`. It demonstrates:
- Bootstrapping the `vault` layer
- Loading secrets from Vault to fill parameters
- Printing resulting parameters

Key points in the example:
- Declares an `api-key` parameter in the default layer.
- Adds a `vault` layer for connection settings.
- Uses Cobra for flag parsing and integrates the middleware chain.
- Calls `UpdateFromVault("kv/example/app")` and prints the resolved `api-key`.

Expected behavior:
- If the secret at `kv/example/app` contains `{ "api-key": "XYZ" }`, the example prints that value.
- Flags or Viper can still override values depending on your chain order.

## 7. Troubleshooting

- Ensure the Vault path is correct; KV v2 and v1 are handled automatically.
- If the path uses templates, verify that token lookup works (`vault token lookup -format=json`).
- Use `WithParseStepSource("vault")` for clear provenance in parameter history.
- If nothing updates, check naming: secret keys must match parameter names (including `-`/`_` usage).
- Handle errors explicitly when Vault is required: fail fast if the path is mandatory, or ignore-missing if it’s optional.
- Avoid logging secret values in debug output.

## 8. References

- `glaze help middlewares-guide`
- `github.com/go-go-golems/glazed/pkg/cmds/middlewares`
- `github.com/go-go-golems/vault-envrc-generator/pkg/vaultlayer`
- `github.com/go-go-golems/vault-envrc-generator/pkg/glazed`

## 9. Appendix: Rationale and Alternatives

Why a middleware? It keeps parameter flow explicit and testable, reuses Glazed’s provenance and precedence model, and avoids sprinkling Vault calls across commands. Alternative approaches (manual reads in each command) make precedence harder to reason about and duplicate logic. A dedicated middleware is simpler to compose and document.


