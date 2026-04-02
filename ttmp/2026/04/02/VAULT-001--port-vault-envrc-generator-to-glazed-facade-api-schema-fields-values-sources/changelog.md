# Changelog

## 2026-04-02

- Initial workspace created


## 2026-04-02

Complete migration: all 13 source files ported from layers/parameters/middlewares to schema/fields/values/sources. go build ./... clean, golangci-lint 0 issues. Two commits: 9cc50ea (full port + linter fixes). Also added local clay module to go.work (already uses new API). Deprecated clay.InitViper→clay.InitGlazed; sources.GatherFlagsFromViper→sources.FromEnv; CobraParserConfig{AppName} replaces custom getMiddlewares.

### Related Files

- /home/manuel/workspaces/2025-09-10/add-vault-middleware-to-glazed/vault-envrc-generator/cmd/vault-envrc-generator/main.go — Migrated: clay.InitGlazed
- /home/manuel/workspaces/2025-09-10/add-vault-middleware-to-glazed/vault-envrc-generator/pkg/glazed/middleware.go — Migrated: UpdateFromVault uses sources.Middleware
- /home/manuel/workspaces/2025-09-10/add-vault-middleware-to-glazed/vault-envrc-generator/pkg/vaultlayer/layer.go — Migrated: NewVaultLayer→NewVaultSection

