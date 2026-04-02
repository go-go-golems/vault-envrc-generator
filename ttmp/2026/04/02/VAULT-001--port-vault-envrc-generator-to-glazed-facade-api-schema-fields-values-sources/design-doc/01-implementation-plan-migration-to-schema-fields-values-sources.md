---
Title: 'Implementation Plan: Migration to schema/fields/values/sources'
Ticket: VAULT-001
Status: active
Topics:
    - migration
    - glazed
    - go
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - /home/manuel/workspaces/2025-09-10/add-vault-middleware-to-glazed/vault-envrc-generator/pkg/vaultlayer/layer.go:Primary vault section definition – uses layers+parameters old API
    - /home/manuel/workspaces/2025-09-10/add-vault-middleware-to-glazed/vault-envrc-generator/pkg/glazed/middleware.go:UpdateFromVault – uses middlewares.HandlerFunc and old layer iteration
    - /home/manuel/workspaces/2025-09-10/add-vault-middleware-to-glazed/vault-envrc-generator/cmds/generate.go:Representative command – uses ParsedLayers, parameters.*, WithLayersList
    - /home/manuel/workspaces/2025-09-10/add-vault-middleware-to-glazed/vault-envrc-generator/cmds/list.go:GlazeCommand example with settings.NewGlazedParameterLayers
    - /home/manuel/workspaces/2025-09-10/add-vault-middleware-to-glazed/vault-envrc-generator/cmd/vault-envrc-generator/main.go:Main – wires middleware chain with middlewares.* and ParsedLayers
    - /home/manuel/workspaces/2025-09-10/add-vault-middleware-to-glazed/vault-envrc-generator/cmd/examples/vault-glaze-example/main.go:Example – direct layers/middlewares/parameters usage
ExternalSources:
    - /home/manuel/workspaces/2025-09-10/add-vault-middleware-to-glazed/glazed/pkg/doc/tutorials/migrating-to-facade-packages.md
Summary: "Step-by-step plan to port vault-envrc-generator from the removed layers/parameters/middlewares API to schema/fields/values/sources."
LastUpdated: 2026-04-02T13:15:27.400936995-04:00
WhatFor: "Restore compilation and test suite against the new Glazed API."
---

# Implementation Plan: Migration to schema/fields/values/sources

## Executive Summary

The Glazed framework has undergone a breaking rename: the legacy packages
`layers`, `parameters`, `middlewares`, and their parsed-layer types have been
replaced by `schema`, `fields`, `sources`, and `values`. All aliases and shims
have been removed.

`vault-envrc-generator` is currently broken against the new Glazed. This
document describes the systematic, ordered changes needed to restore
`go build ./...` and `go test ./...`.

The changes touch **13 files** across 5 packages. Because the changes are
purely mechanical (no logic change), the safest order is:

1. Bottom-up: fix leaf packages first (`fields`, `schema`, `values`)  
2. Work up to mid-level packages (`vaultlayer`, the custom `glazed` middleware)  
3. Fix the 9 command files  
4. Fix the two entry-point `main.go` files  
5. Validate: compile + tests + linter

---

## Problem Statement

`vault-envrc-generator` depends on `github.com/go-go-golems/glazed` at a version
that removed:

| Removed symbol | Replaced by |
|---|---|
| `layers.ParameterLayer` | `schema.Section` |
| `layers.ParameterLayers` | `schema.Schema` |
| `layers.NewParameterLayer` | `schema.NewSection` |
| `layers.WithParameterDefinitions` | `schema.WithFields` |
| `layers.ParsedLayers` / `layers.NewParsedLayers` | `values.Values` / `values.New()` |
| `layers.ParsedLayer` | `values.SectionValues` |
| `layers.DefaultSlug` | `schema.DefaultSlug` |
| `parameters.NewParameterDefinition` | `fields.New` |
| `parameters.ParameterType*` | `fields.Type*` |
| `parameters.WithHelp/Default/Required/…` | `fields.WithHelp/Default/Required/…` |
| `parameters.ParseStepOption` | `fields.ParseOption` |
| `parameters.WithParseStepSource` | `fields.WithSource` |
| `middlewares.Middleware` / `HandlerFunc` | `sources.Middleware` / `sources.HandlerFunc` |
| `middlewares.ExecuteMiddlewares` | `sources.Execute` |
| `middlewares.ParseFromCobraCommand` | `sources.FromCobra` |
| `middlewares.GatherArguments` | `sources.FromArgs` |
| `middlewares.GatherFlagsFromViper` | `sources.GatherFlagsFromViper` (deprecated in sources but present) |
| `middlewares.SetFromDefaults` | `sources.FromDefaults` |
| `middlewares.WrapWithWhitelistedLayers` | `sources.WrapWithWhitelistedSections` |
| `cmds.WithLayersList` | `cmds.WithSections` |
| `settings.NewGlazedParameterLayers` | `settings.NewGlazedSection` |
| `cli.NewCommandSettingsLayer` | `cli.NewCommandSettingsSection` |
| `logging.AddLoggingLayerToRootCommand` | `logging.AddLoggingSectionToRootCommand` |
| `logging.InitLoggerFromViper` (deprecated) | `logging.SetupLoggingFromValues` |

Additionally, all struct tags `glazed.parameter:"name"` must be changed to `glazed:"name"`.

---

## Proposed Solution

Mechanical, file-by-file API substitution following the Glazed migration guide.
No logic changes to business logic (Vault client, envrc generation, seed, batch).

### Step 0: Update go.mod / go.sum

Point `github.com/go-go-golems/glazed` to the version that carries the new
`schema/fields/values/sources` packages (the local workspace symlink already
points to the correct source tree). Run `go mod tidy`.

Also update `github.com/go-go-golems/clay` if it also uses the old API (check
imports in clay to ensure it exports a compatible `NewCommandSettingsSection`).

### Step 1: `pkg/vaultlayer/layer.go`

**Goal**: Define `NewVaultSection()` returning `schema.Section`, replace
`AddVaultLayerToCommand` to use `c.Description().Schema.Set(...)`, and update
`GetVaultSettings` to accept `*values.Values`.

```go
// OLD
func NewVaultLayer() (glzlayers.ParameterLayer, error) {
    return glzlayers.NewParameterLayer(VaultLayerSlug, "...",
        glzlayers.WithParameterDefinitions(
            parameters.NewParameterDefinition("vault-addr", parameters.ParameterTypeString, ...),
        ),
    )
}

// NEW
func NewVaultSection() (schema.Section, error) {
    return schema.NewSection(VaultLayerSlug, "...",
        schema.WithFields(
            fields.New("vault-addr", fields.TypeString, ...),
        ),
    )
}
```

```go
// OLD signature
func AddVaultLayerToCommand(c glzcms.Command) (glzcms.Command, error) {
    l, err := NewVaultLayer()
    ...
    c.Description().Layers.Set(VaultLayerSlug, l)
    ...
}

// NEW
func AddVaultSectionToCommand(c glzcms.Command) (glzcms.Command, error) {
    s, err := NewVaultSection()
    ...
    c.Description().Schema.Set(VaultLayerSlug, s)
    ...
}
```

```go
// OLD
func GetVaultSettings(parsed *glzlayers.ParsedLayers) (*VaultSettings, error) {
    var s VaultSettings
    if err := parsed.InitializeStruct(VaultLayerSlug, &s); err != nil { ... }
    return &s, nil
}

// NEW
func GetVaultSettings(parsed *values.Values) (*VaultSettings, error) {
    var s VaultSettings
    if err := parsed.DecodeSectionInto(VaultLayerSlug, &s); err != nil { ... }
    return &s, nil
}
```

Struct tags: change `glazed.parameter:"..."` → `glazed:"..."` in `VaultSettings`.

### Step 2: `pkg/glazed/middleware.go`

**Goal**: Port `UpdateFromVault` to use `sources.Middleware` / `sources.HandlerFunc`,
iterate over `schema.Schema` sections, and use updated `values.Values` types.

```go
// OLD signature
func UpdateFromVault(path string, options ...parameters.ParseStepOption) gmiddlewares.Middleware {
    return func(next gmiddlewares.HandlerFunc) gmiddlewares.HandlerFunc {
        return func(layers *glayers.ParameterLayers, parsed *glayers.ParsedLayers) error {

// NEW signature
func UpdateFromVault(path string, options ...fields.ParseOption) sources.Middleware {
    return func(next sources.HandlerFunc) sources.HandlerFunc {
        return func(schema_ *schema.Schema, parsedValues *values.Values) error {
```

Inner loop:
```go
// OLD
err = layers.ForEachE(func(_ string, l glayers.ParameterLayer) error {
    parsedLayer := parsed.GetOrCreate(l)
    pds := l.GetParameterDefinitions()
    return pds.ForEachE(func(pd *parameters.ParameterDefinition) error {
        if v, ok := secrets[pd.Name]; ok {
            parsedLayer.Parameters.UpdateValue(pd.Name, pd, v, options...)
        }
        return nil
    })
})

// NEW
err = schema_.ForEachE(func(_ string, s schema.Section) error {
    sectionVals := parsedValues.GetOrCreate(s)
    defs := s.GetDefinitions()
    return defs.ForEachE(func(pd *fields.Definition) error {
        if v, ok := secrets[pd.Name]; ok {
            if err := sectionVals.Parameters.UpdateValue(pd.Name, pd, v, options...); err != nil {
                return err
            }
        }
        return nil
    })
})
```

### Step 3: `cmds/*.go` — All 9 command files

Apply the following mechanical changes to each file:

**Import changes** (per file):
- Remove `glayers "glazed/pkg/cmds/layers"`
- Replace `"glazed/pkg/cmds/parameters"` with `"glazed/pkg/cmds/fields"`
- Add `"glazed/pkg/cmds/schema"` (for `schema.DefaultSlug`)
- Add `"glazed/pkg/cmds/values"` (for `*values.Values`)
- For list.go/validate.go: `settings.NewGlazedParameterLayers` → `settings.NewGlazedSection`

**Symbol changes**:

| Old | New |
|---|---|
| `parameters.NewParameterDefinition(n, t, opts...)` | `fields.New(n, t, opts...)` |
| `parameters.ParameterTypeString` | `fields.TypeString` |
| `parameters.ParameterTypeBool` | `fields.TypeBool` |
| `parameters.ParameterTypeInteger` | `fields.TypeInteger` |
| `parameters.ParameterTypeStringList` | `fields.TypeStringList` |
| `parameters.ParameterTypeChoice` | `fields.TypeChoice` |
| `parameters.WithHelp(s)` | `fields.WithHelp(s)` |
| `parameters.WithDefault(v)` | `fields.WithDefault(v)` |
| `parameters.WithRequired(b)` | `fields.WithRequired(b)` |
| `parameters.WithShortFlag(c)` | `fields.WithShortFlag(c)` |
| `parameters.WithChoices(...)` | `fields.WithChoices(...)` |
| `gcmds.WithLayersList(layers...)` | `gcmds.WithSections(sections...)` |
| `glzcli.NewCommandSettingsLayer()` | `glzcli.NewCommandSettingsSection()` |
| `settings.NewGlazedParameterLayers()` | `settings.NewGlazedSection()` |
| `*glayers.ParsedLayers` (Run param) | `*values.Values` |
| `parsed.InitializeStruct(glayers.DefaultSlug, s)` | `parsed.DecodeSectionInto(schema.DefaultSlug, s)` |
| `vaultlayer.AddVaultLayerToCommand(cd)` | `vaultlayer.AddVaultSectionToCommand(cd)` |
| `vaultlayer.GetVaultSettings(parsed)` | (unchanged call, but signature changed – will compile after step 1) |

**Struct tags** (in every `*Settings` struct in `cmds/`):
```go
// OLD
type GenerateSettings struct {
    Path string `glazed.parameter:"path"`
}
// NEW
type GenerateSettings struct {
    Path string `glazed:"path"`
}
```

### Step 4: `cmd/vault-envrc-generator/main.go`

```go
// OLD imports
"glazed/pkg/cmds/layers"
"glazed/pkg/cmds/middlewares"
"glazed/pkg/cmds/parameters"

// NEW imports
"glazed/pkg/cmds/values"
"glazed/pkg/cmds/sources"
"glazed/pkg/cmds/fields"
```

```go
// OLD
func getMiddlewares(parsedLayers *layers.ParsedLayers, cmd *cobra.Command, args []string) ([]middlewares.Middleware, error) {
    commandSettings := &cli.CommandSettings{}
    err := parsedLayers.InitializeStruct(cli.CommandSettingsSlug, commandSettings)
    ...
    mw_ := []middlewares.Middleware{
        middlewares.ParseFromCobraCommand(cmd, parameters.WithParseStepSource("cobra")),
        middlewares.GatherArguments(args, parameters.WithParseStepSource("arguments")),
        middlewares.GatherFlagsFromViper(parameters.WithParseStepSource("viper")),
        middlewares.SetFromDefaults(parameters.WithParseStepSource("defaults")),
    }

// NEW
func getMiddlewares(parsedLayers *values.Values, cmd *cobra.Command, args []string) ([]sources.Middleware, error) {
    commandSettings := &cli.CommandSettings{}
    err := parsedLayers.DecodeSectionInto(cli.CommandSettingsSlug, commandSettings)
    ...
    mw_ := []sources.Middleware{
        sources.FromCobra(cmd, fields.WithSource("cobra")),
        sources.FromArgs(args, fields.WithSource("arguments")),
        sources.GatherFlagsFromViper(fields.WithSource("viper")),
        sources.FromDefaults(fields.WithSource("defaults")),
    }
```

Also update logging setup:
```go
// OLD
logging.InitLoggerFromViper()  // deprecated

// NEW – preferred
logging.SetupLoggingFromValues(parsedLayers)
// or keep deprecated call (it still works but emits a warning)
```

### Step 5: `cmd/examples/vault-glaze-example/main.go`

```go
// OLD
appLayer, _ := glayers.NewParameterLayer(glayers.DefaultSlug, "Example App Settings",
    glayers.WithParameterDefinitions(
        parameters.NewParameterDefinition("api-key", parameters.ParameterTypeString, ...),
    ),
)
vaultLayer, _ := vaultlayer.NewVaultLayer()
pls := glayers.NewParameterLayers(glayers.WithLayers(cs, vaultLayer, appLayer))
parsed := glayers.NewParsedLayers()
mw := []middlewares.Middleware{
    middlewares.ParseFromCobraCommand(nil, parameters.WithParseStepSource("flags")),
    middlewares.GatherArguments(os.Args[1:], parameters.WithParseStepSource("args")),
    middlewares.WrapWithWhitelistedLayers(
        []string{vaultlayer.VaultLayerSlug},
        middlewares.SetFromDefaults(parameters.WithParseStepSource("defaults")),
        middlewares.GatherFlagsFromViper(parameters.WithParseStepSource("viper")),
    ),
    vglazed.UpdateFromVault("kv/example/app", parameters.WithParseStepSource("vault")),
    middlewares.GatherFlagsFromViper(parameters.WithParseStepSource("viper")),
    middlewares.SetFromDefaults(parameters.WithParseStepSource("defaults")),
}
middlewares.ExecuteMiddlewares(pls, parsed, mw...)
parsed.InitializeStruct(glayers.DefaultSlug, &s)

// NEW
appSection, _ := schema.NewSection(schema.DefaultSlug, "Example App Settings",
    schema.WithFields(
        fields.New("api-key", fields.TypeString, ...),
    ),
)
vaultSection, _ := vaultlayer.NewVaultSection()
pls := schema.NewSchema(schema.WithSections(cs, vaultSection, appSection))
parsed := values.New()
mw := []sources.Middleware{
    sources.FromCobra(nil, fields.WithSource("flags")),
    sources.FromArgs(os.Args[1:], fields.WithSource("args")),
    sources.WrapWithWhitelistedSections(
        []string{vaultlayer.VaultLayerSlug},
        sources.FromDefaults(fields.WithSource("defaults")),
        sources.GatherFlagsFromViper(fields.WithSource("viper")),
    ),
    vglazed.UpdateFromVault("kv/example/app", fields.WithSource("vault")),
    sources.GatherFlagsFromViper(fields.WithSource("viper")),
    sources.FromDefaults(fields.WithSource("defaults")),
}
sources.Execute(pls, parsed, mw...)
parsed.DecodeSectionInto(schema.DefaultSlug, &s)
```

### Step 6: Validate

```bash
go build ./...
go test ./...
golangci-lint run -v
rg -n -i "glazed\.parameter" --include="*.go" .   # should be 0 hits
rg -n "layers\." --include="*.go" . | grep -v "_test.go"  # should be 0 meaningful hits
```

---

## Design Decisions

### Keep `AddVaultLayerToCommand` name vs rename to `AddVaultSectionToCommand`

**Decision**: Rename to `AddVaultSectionToCommand` to align with Glazed's own
naming conventions (e.g., `logging.AddLoggingSectionToCommand`). Update all call
sites in `cmds/*.go`.

### Keep `GetVaultSettings(parsed)` call shape

**Decision**: Keep the same helper function shape but change the argument type
from `*layers.ParsedLayers` to `*values.Values`. All call sites in `cmds/*.go`
pass `parsed` which will be `*values.Values` after their own migration, so no
additional changes are needed at call sites.

### `sources.GatherFlagsFromViper` (deprecated) vs `sources.FromFiles`+`sources.FromEnv`

**Decision**: Use `sources.GatherFlagsFromViper` initially for the smallest
diff. Log the deprecation warning in a follow-up if it becomes noisy. A full
replacement with `sources.FromFiles`+`sources.FromEnv` can be done in a
separate ticket.

### `logging.InitLoggerFromViper` (deprecated)

**Decision**: Replace with `logging.SetupLoggingFromValues` in `main.go`.
The new API takes the parsed values, which are now available at the point
where `InitLoggerFromViper` was called.

---

## Alternatives Considered

### Shim layer (`layers` alias package)

The migration guide notes that alias shims have been **removed**. There is no
shim package to depend on; the change must be done at the source level.

### AST rename tool

Glazed provides a symbol rename YAML (Appendix A of the migration guide).
Using `gorename` or a custom AST tool could automate some renames. However,
given the small number of files (13), manual edits are faster and less risky.

---

## Open Questions

1. Does `clay` package `clay.InitViper` still work as-is, or does it also need
   updating? (Check if `clay` re-exports any old Glazed types.)
2. Does `github.com/go-go-golems/glazed/pkg/middlewares.Processor` (the output
   processor, not the sources middleware) remain unchanged? (Answer: yes –
   `pkg/middlewares` is unrelated to `pkg/cmds/middlewares`/`sources`.)
3. Are there any YAML command definition files under `cmds/` or similar that use
   `layers:` key and need to be renamed to `sections:`? (Quick scan shows no
   embedded YAML command specs – only Go-defined commands.)

---

## References

- Migration guide: `glazed/pkg/doc/tutorials/migrating-to-facade-packages.md`
- New sources API: `glazed/pkg/cmds/sources/`
- New schema API: `glazed/pkg/cmds/schema/`
- New fields API: `glazed/pkg/cmds/fields/`
- New values API: `glazed/pkg/cmds/values/`
