---
Title: Port vault-envrc-generator to Glazed facade API (schema/fields/values/sources)
Ticket: VAULT-001
Status: active
Topics:
    - migration
    - glazed
    - go
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: vault-envrc-generator/cmd/examples/vault-glaze-example/main.go:Example program – uses old layers/middlewares/parameters directly
    - Path: vault-envrc-generator/cmd/vault-envrc-generator/main.go:Entry point – wires middleware chain with legacy middlewares.*
    - Path: vault-envrc-generator/cmds/batch.go
      Note: batch command – all legacy parameters/layers/ParsedLayers usage
    - Path: vault-envrc-generator/cmds/seed.go
      Note: seed command – legacy parameters/layers/ParsedLayers
    - Path: vault-envrc-generator/cmds/token.go
      Note: token command – GlazeCommand
    - Path: vault-envrc-generator/cmds/tree.go
      Note: tree command – legacy parameters/layers/ParsedLayers
    - Path: vault-envrc-generator/cmds/validate.go
      Note: validate command – GlazeCommand with NewGlazedParameterLayers
    - Path: vault-envrc-generator/pkg/glazed/middleware.go:UpdateFromVault middleware – uses old sources/layers/parameters API
    - Path: vault-envrc-generator/pkg/vaultlayer/layer.go:Core vault layer/section – primary migration target
ExternalSources:
    - /home/manuel/workspaces/2025-09-10/add-vault-middleware-to-glazed/glazed/pkg/doc/tutorials/migrating-to-facade-packages.md
Summary: Migrate all Glazed API usages in vault-envrc-generator from the legacy layers/parameters/middlewares API to the new schema/fields/values/sources facade packages.
LastUpdated: 2026-04-02T13:14:28.241171622-04:00
WhatFor: Keeping vault-envrc-generator compatible with the current Glazed API after the breaking migration that removed layers/parameters/middlewares packages.
WhenToUse: ""
---


# Port vault-envrc-generator to Glazed facade API (schema/fields/values/sources)

## Overview

The Glazed framework has undergone a breaking API migration. The legacy packages
`layers`, `parameters`, `middlewares`, and `parsedlayers` have been removed and
replaced with `schema`, `fields`, `values`, and `sources` respectively.

`vault-envrc-generator` currently uses the old API extensively across:
- `pkg/vaultlayer/layer.go` – the reusable VaultLayer/VaultSection
- `pkg/glazed/middleware.go` – the `UpdateFromVault` middleware
- `cmds/*.go` – all 9 command files
- `cmd/vault-envrc-generator/main.go` – root Cobra wiring + middleware chain
- `cmd/examples/vault-glaze-example/main.go` – standalone example

This ticket tracks the complete migration to compile and pass `go test ./...`
against the new Glazed facade packages.

## Conceptual Terminology Map

| Old | New |
|---|---|
| Layer | Section |
| ParameterLayer | schema.Section |
| ParameterLayers | schema.Schema |
| ParsedLayer | values.SectionValues |
| ParsedLayers | values.Values |
| Middlewares | Sources |

## Package Import Mapping

| Old import | New import |
|---|---|
| `glazed/pkg/cmds/layers` | `glazed/pkg/cmds/schema` + `glazed/pkg/cmds/values` |
| `glazed/pkg/cmds/parameters` | `glazed/pkg/cmds/fields` |
| `glazed/pkg/cmds/middlewares` | `glazed/pkg/cmds/sources` |

## Scope of Changes

### `pkg/vaultlayer/layer.go`
- `layers.ParameterLayer` → `schema.Section`
- `layers.NewParameterLayer(...)` → `schema.NewSection(...)`
- `layers.WithParameterDefinitions(...)` → `schema.WithFields(...)`
- `parameters.NewParameterDefinition(...)` → `fields.New(...)`
- `parameters.ParameterTypeString/Bool/Choice/etc` → `fields.TypeString/TypeBool/TypeChoice/etc`
- `parameters.WithHelp/WithDefault/WithChoices/WithRequired` → same names in `fields` package
- `c.Description().Layers.Set(slug, l)` → `c.Description().Schema.Set(slug, l)`
- `parsed.InitializeStruct(slug, &s)` → `parsed.DecodeSectionInto(slug, &s)`
- Import `glzlayers "glazed/pkg/cmds/layers"` → `"glazed/pkg/cmds/schema"` + `"glazed/pkg/cmds/values"`
- Import `"glazed/pkg/cmds/parameters"` → `"glazed/pkg/cmds/fields"`
- `*glzlayers.ParsedLayers` argument type → `*values.Values`

### `pkg/glazed/middleware.go`
- `gmiddlewares.Middleware` → `sources.Middleware`
- `gmiddlewares.HandlerFunc` → `sources.HandlerFunc`
- `func(next HandlerFunc) HandlerFunc { return func(layers *glayers.ParameterLayers, parsed *glayers.ParsedLayers) error {...} }`  
  → `func(next sources.HandlerFunc) sources.HandlerFunc { return func(schema_ *schema.Schema, parsedValues *values.Values) error {...} }`
- `layers.ForEachE(func(slug string, l glayers.ParameterLayer) error {...})` → `schema_.ForEachE(func(slug string, s schema.Section) error {...})`
- `parsed.GetOrCreate(l)` → `parsedValues.GetOrCreate(s)` (note: `GetOrCreate` takes a `Section` not a `ParameterLayer`)
- `l.GetParameterDefinitions()` → `s.GetDefinitions()`
- `pds.ForEachE(func(pd *parameters.ParameterDefinition) error {...})` → `pds.ForEachE(func(pd *fields.Definition) error {...})`
- `parsedLayer.Parameters.UpdateValue(name, pd, v, options...)` → `sectionValues.Parameters.UpdateValue(name, pd, v, options...)`  
  (verify exact method signatures in new `fields.FieldValues`)
- `parameters.ParseStepOption` → `fields.ParseOption`
- `parameters.WithParseStepSource("vault")` → `fields.WithSource("vault")`
- Import `gmiddlewares "glazed/pkg/cmds/middlewares"` → `"glazed/pkg/cmds/sources"`
- Import `glayers "glazed/pkg/cmds/layers"` → `"glazed/pkg/cmds/schema"` + `"glazed/pkg/cmds/values"`
- Import `"glazed/pkg/cmds/parameters"` → `"glazed/pkg/cmds/fields"`

### `cmds/*.go` (all 9 command files)

Common changes across every command file:
- Import `glayers "glazed/pkg/cmds/layers"` → drop (use `values` where needed)
- Import `"glazed/pkg/cmds/parameters"` → `"glazed/pkg/cmds/fields"`
- `parameters.NewParameterDefinition(name, paramType, opts...)` → `fields.New(name, fieldType, opts...)`
- `parameters.ParameterTypeString` → `fields.TypeString`, `ParameterTypeInteger` → `fields.TypeInteger`, etc.
- `parameters.WithHelp/WithDefault/WithRequired/WithShortFlag/WithChoices` → same names in `fields`
- `gcmds.WithLayersList(layer1, layer2, ...)` → `gcmds.WithSections(layer1, layer2, ...)`
- `func (c *XCommand) Run(ctx context.Context, parsed *glayers.ParsedLayers) error` → `func (c *XCommand) Run(ctx context.Context, parsed *values.Values) error`
- `func (c *XCommand) RunIntoGlazeProcessor(ctx context.Context, parsed *glayers.ParsedLayers, gp middlewares.Processor) error` → `func (c *XCommand) RunIntoGlazeProcessor(ctx context.Context, parsed *values.Values, gp middlewares.Processor) error`
  (note: `middlewares.Processor` lives in `glazed/pkg/middlewares`, not the same package — keep that import)
- `parsed.InitializeStruct(glayers.DefaultSlug, s)` → `parsed.DecodeSectionInto(schema.DefaultSlug, s)`
- `struct tags: glazed.parameter:"name"` → `glazed:"name"` (breaking struct tag rename)

Per-command specifics:
- **list.go**: `settings.NewGlazedParameterLayers()` → `settings.NewGlazedSection()`
- **validate.go**: same `settings.NewGlazedParameterLayers()` → `settings.NewGlazedSection()`
- **list.go**, **token.go**, **validate.go**: `glayers.ParsedLayers` in `RunIntoGlazeProcessor` → `values.Values`
- `glzcli.NewCommandSettingsLayer()` → `glzcli.NewCommandSettingsSection()` (in all command constructors)
- vaultlayer call: `vaultlayer.GetVaultSettings(parsed)` signature must match new `*values.Values`

### `cmd/vault-envrc-generator/main.go`
- `layers.ParsedLayers` → `values.Values`
- `parsedLayers.InitializeStruct(cli.CommandSettingsSlug, commandSettings)` → `parsedLayers.DecodeSectionInto(cli.CommandSettingsSlug, commandSettings)`
- `middlewares.ParseFromCobraCommand(cmd, ...)` → `sources.FromCobra(cmd, ...)`
- `middlewares.GatherArguments(args, ...)` → `sources.FromArgs(args, ...)`
- `middlewares.GatherFlagsFromViper(...)` → `sources.GatherFlagsFromViper(...)` (deprecated but still in `sources`; or replace with `sources.FromFiles` + `sources.FromEnv`)
- `middlewares.SetFromDefaults(...)` → `sources.FromDefaults(...)`
- `parameters.WithParseStepSource("cobra")` → `fields.WithSource("cobra")`
- Import `"glazed/pkg/cmds/layers"` → `"glazed/pkg/cmds/values"`
- Import `"glazed/pkg/cmds/middlewares"` → `"glazed/pkg/cmds/sources"`
- Import `"glazed/pkg/cmds/parameters"` → `"glazed/pkg/cmds/fields"`
- `logging.AddLoggingLayerToRootCommand` → `logging.AddLoggingSectionToRootCommand`
- `logging.InitLoggerFromViper()` (deprecated) → `logging.SetupLoggingFromValues(...)` or keep with deprecation warning

### `cmd/examples/vault-glaze-example/main.go`
- `glayers.NewParameterLayer(slug, name, glayers.WithParameterDefinitions(...))` → `schema.NewSection(slug, name, schema.WithFields(...))`
- `glayers.NewParameterLayers(glayers.WithLayers(...))` → `schema.NewSchema(schema.WithSections(...))`
- `glayers.NewParsedLayers()` → `values.New()`
- `middlewares.ParseFromCobraCommand(nil, ...)` → `sources.FromCobra(nil, ...)`
- `middlewares.GatherArguments(os.Args[1:], ...)` → `sources.FromArgs(os.Args[1:], ...)`
- `middlewares.WrapWithWhitelistedLayers([...]string{...}, ...)` → `sources.WrapWithWhitelistedSections([...]string{...}, ...)`
- `middlewares.SetFromDefaults(...)` → `sources.FromDefaults(...)`
- `middlewares.GatherFlagsFromViper(...)` → `sources.GatherFlagsFromViper(...)` or replace
- `middlewares.ExecuteMiddlewares(pls, parsed, mw...)` → `sources.Execute(pls, parsed, mw...)`  
  (note signature: `sources.Execute(schema_ *schema.Schema, parsedValues *values.Values, middlewares ...Middleware)`)
- `parsed.InitializeStruct(glayers.DefaultSlug, &s)` → `parsed.DecodeSectionInto(schema.DefaultSlug, &s)`
- All imports updated accordingly

## Status

Current status: **active**

## Tasks

See [tasks.md](./tasks.md) for the current task list.

## Changelog

See [changelog.md](./changelog.md) for recent changes and decisions.

## Reference

- Migration guide: `glazed/pkg/doc/tutorials/migrating-to-facade-packages.md`
