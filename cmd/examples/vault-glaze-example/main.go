package main

import (
	"context"
	"fmt"
	"os"

	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	glayers "github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/middlewares"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"

	vglazed "github.com/go-go-golems/vault-envrc-generator/pkg/glazed"
	"github.com/go-go-golems/vault-envrc-generator/pkg/vaultlayer"
)

type ExampleSettings struct {
	// This will be set from Vault if a key with the same name exists
	ApiKey string `glazed.parameter:"api-key"`
}

func main() {
	// Define layers: command settings + vault + default app layer
	cs, err := cli.NewCommandSettingsLayer()
	if err != nil {
		panic(err)
	}
	appLayer, err := glayers.NewParameterLayer(
		glayers.DefaultSlug,
		"Example App Settings",
		glayers.WithParameterDefinitions(
			parameters.NewParameterDefinition("api-key", parameters.ParameterTypeString, parameters.WithHelp("API key (populated from Vault)")),
		),
	)
	if err != nil {
		panic(err)
	}
	vaultLayer, err := vaultlayer.NewVaultLayer()
	if err != nil {
		panic(err)
	}
	pls := glayers.NewParameterLayers(glayers.WithLayers(cs, vaultLayer, appLayer))
	parsed := glayers.NewParsedLayers()

	// Simple middleware chain demonstrating vault bootstrap and UpdateFromVault
	mw := []middlewares.Middleware{
		// Basic CLI collection (flags/args) – first so they get highest precedence later
		middlewares.ParseFromCobraCommand(nil, parameters.WithParseStepSource("flags")),
		middlewares.GatherArguments(os.Args[1:], parameters.WithParseStepSource("args")),

		// 1) Bootstrap ONLY the vault layer
		middlewares.WrapWithWhitelistedLayers(
			[]string{vaultlayer.VaultLayerSlug},
			middlewares.SetFromDefaults(parameters.WithParseStepSource("defaults")),
			middlewares.GatherFlagsFromViper(parameters.WithParseStepSource("viper")),
		),

		// 2) Load values from Vault into parameters across layers
		vglazed.UpdateFromVault("kv/example/app", parameters.WithParseStepSource("vault")),

		// 3) Normal middlewares for all layers
		middlewares.GatherFlagsFromViper(parameters.WithParseStepSource("viper")),
		middlewares.SetFromDefaults(parameters.WithParseStepSource("defaults")),
	}

	if err := middlewares.ExecuteMiddlewares(pls, parsed, mw...); err != nil {
		panic(err)
	}

	// Show result
	var s ExampleSettings
	if err := parsed.InitializeStruct(glayers.DefaultSlug, &s); err != nil {
		panic(err)
	}
	if s.ApiKey != "" {
		fmt.Println("api-key retrieved successfully")
	} else {
		fmt.Println("api-key not set")
	}

	// Prevent unused imports if running as a simple example
	_ = context.Background()
	_ = cmds.NewCommandDescription
}
