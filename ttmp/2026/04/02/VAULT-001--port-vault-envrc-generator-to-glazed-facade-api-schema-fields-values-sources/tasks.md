# Tasks

## TODO

- [ ] Add tasks here

- [x] Step 0: Update go.mod to point to Glazed with schema/fields/values/sources; run go mod tidy
- [x] Step 1a: pkg/vaultlayer/layer.go – update imports (layers→schema+values, parameters→fields)
- [x] Step 1b: pkg/vaultlayer/layer.go – rename NewVaultLayer()→NewVaultSection(), return schema.Section
- [x] Step 1c: pkg/vaultlayer/layer.go – rename AddVaultLayerToCommand→AddVaultSectionToCommand, use Schema.Set
- [x] Step 1d: pkg/vaultlayer/layer.go – update GetVaultSettings to accept *values.Values and call DecodeSectionInto
- [x] Step 1e: pkg/vaultlayer/layer.go – change VaultSettings struct tags glazed.parameter: → glazed:
- [x] Step 2a: pkg/glazed/middleware.go – update imports (middlewares→sources, layers→schema+values, parameters→fields)
- [x] Step 2b: pkg/glazed/middleware.go – change UpdateFromVault signature: sources.Middleware, fields.ParseOption
- [x] Step 2c: pkg/glazed/middleware.go – update HandlerFunc closure params: schema_ *schema.Schema, parsedValues *values.Values
- [x] Step 2d: pkg/glazed/middleware.go – fix inner loop: schema_.ForEachE, s.GetDefinitions(), parsedValues.GetOrCreate(s), *fields.Definition
- [x] Step 3a: cmds/generate.go – imports, WithSections, fields.New+TypeX, *values.Values, DecodeSectionInto, struct tags
- [x] Step 3b: cmds/batch.go – imports, WithSections, fields.New+TypeX, *values.Values, DecodeSectionInto, struct tags
- [x] Step 3c: cmds/seed.go – imports, WithSections, fields.New+TypeX, *values.Values, DecodeSectionInto, struct tags
- [x] Step 3d: cmds/tree.go – imports, WithSections, fields.New+TypeX, *values.Values, DecodeSectionInto, struct tags
- [x] Step 3e: cmds/rmtree.go – imports, WithSections, fields.New+TypeX, *values.Values, DecodeSectionInto, struct tags
- [x] Step 3f: cmds/interactive.go – imports, WithSections, fields.New+TypeX, *values.Values, DecodeSectionInto, struct tags
- [x] Step 3g: cmds/token.go – imports, WithSections, fields.New+TypeX, *values.Values, DecodeSectionInto, struct tags
- [x] Step 3h: cmds/list.go – settings.NewGlazedSection(), WithSections, *values.Values RunIntoGlazeProcessor, fields.*, struct tags
- [x] Step 3i: cmds/validate.go – settings.NewGlazedSection(), WithSections, *values.Values RunIntoGlazeProcessor, fields.*, struct tags
- [x] Step 4: cmd/vault-envrc-generator/main.go – update getMiddlewares signature, sources.FromCobra/FromArgs/GatherFlagsFromViper/FromDefaults, fields.WithSource
- [x] Step 4b: cmd/vault-envrc-generator/main.go – replace logging.InitLoggerFromViper with logging.SetupLoggingFromValues
- [x] Step 5: cmd/examples/vault-glaze-example/main.go – full port: schema.NewSection, schema.NewSchema, values.New, sources.*, fields.*, DecodeSectionInto
- [x] Step 6: Validate – go build ./...; go test ./...; golangci-lint run; rg for remaining glazed.parameter: tags and layers. usages
