package main

import (
	"fmt"
	"os"

	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"strings"

	"github.com/go-go-golems/glazed/pkg/cmds/sources"
	"github.com/go-go-golems/glazed/pkg/cmds/values"

	vglazed "github.com/go-go-golems/vault-envrc-generator/pkg/glazed"
	"github.com/go-go-golems/vault-envrc-generator/pkg/vaultlayer"
)

type ExampleSettings struct {
	// This will be set from Vault if a key with the same name exists
	ApiKey string `glazed:"api-key"`
}

func main() {
	// Define sections: command settings + vault + default app section
	cs, err := cli.NewCommandSettingsSection()
	if err != nil {
		panic(err)
	}
	appSection, err := schema.NewSection(
		schema.DefaultSlug,
		"Example App Settings",
		schema.WithFields(
			fields.New("api-key", fields.TypeString, fields.WithHelp("API key (populated from Vault)")),
		),
	)
	if err != nil {
		panic(err)
	}
	vaultSection, err := vaultlayer.NewVaultSection()
	if err != nil {
		panic(err)
	}
	pls := schema.NewSchema(schema.WithSections(cs, vaultSection, appSection))
	parsed := values.New()

	// Middleware chain demonstrating vault bootstrap and UpdateFromVault
	mw := []sources.Middleware{
		// Basic CLI collection (flags/args) – first so they get highest precedence later
		sources.FromCobra(nil, fields.WithSource("flags")),
		sources.FromArgs(os.Args[1:], fields.WithSource("args")),

		// 1) Bootstrap ONLY the vault section with defaults + env
		sources.WrapWithWhitelistedSections(
			[]string{vaultlayer.VaultLayerSlug},
			sources.FromDefaults(fields.WithSource("defaults")),
			sources.FromEnv(strings.ToUpper("vault_envrc_generator"), fields.WithSource("env")),
		),

		// 2) Load values from Vault into fields across sections
		vglazed.UpdateFromVault("kv/example/app", fields.WithSource("vault")),

		// 3) Normal sources for all sections
		sources.FromEnv(strings.ToUpper("vault_envrc_generator"), fields.WithSource("env")),
		sources.FromDefaults(fields.WithSource("defaults")),
	}

	if err := sources.Execute(pls, parsed, mw...); err != nil {
		panic(err)
	}

	// Show result
	var s ExampleSettings
	if err := parsed.DecodeSectionInto(schema.DefaultSlug, &s); err != nil {
		panic(err)
	}
	fmt.Printf("api-key: %s\n", s.ApiKey)
}
