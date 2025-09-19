package cmds

import (
	"context"
	"strings"

	glzcli "github.com/go-go-golems/glazed/pkg/cli"
	gcmds "github.com/go-go-golems/glazed/pkg/cmds"
	glayers "github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"

	"github.com/go-go-golems/vault-envrc-generator/pkg/vaultlayer"
	"github.com/go-go-golems/vault-envrc-generator/pkg/webserver"
)

type ServeCommand struct{ *gcmds.CommandDescription }

type ServeSettings struct {
	Port        string `glazed.parameter:"port"`
	Host        string `glazed.parameter:"host"`
	CorsOrigins string `glazed.parameter:"cors-origins"`
	DevMode     bool   `glazed.parameter:"dev-mode"`
}

func NewServeCommand() (*ServeCommand, error) {
	commandLayer, err := glzcli.NewCommandSettingsLayer()
	if err != nil {
		return nil, err
	}
	cd := gcmds.NewCommandDescription(
		"serve",
		gcmds.WithShort("Start web server for Vault tree exploration"),
		gcmds.WithFlags(
			parameters.NewParameterDefinition("port", parameters.ParameterTypeString,
				parameters.WithDefault("8080"), parameters.WithHelp("Server port")),
			parameters.NewParameterDefinition("host", parameters.ParameterTypeString,
				parameters.WithDefault("127.0.0.1"), parameters.WithHelp("Server host")),
			parameters.NewParameterDefinition("cors-origins", parameters.ParameterTypeString,
				parameters.WithDefault("*"), parameters.WithHelp("CORS allowed origins")),
			parameters.NewParameterDefinition("dev-mode", parameters.ParameterTypeBool,
				parameters.WithDefault(false), parameters.WithHelp("Enable development mode")),
		),
		gcmds.WithLayersList(commandLayer),
	)
	_, err = vaultlayer.AddVaultLayerToCommand(cd)
	if err != nil {
		return nil, err
	}
	return &ServeCommand{cd}, nil
}

func (c *ServeCommand) Run(ctx context.Context, parsed *glayers.ParsedLayers) error {
	s := &ServeSettings{}
	if err := parsed.InitializeStruct(glayers.DefaultSlug, s); err != nil {
		return err
	}

	// Parse vault settings to ensure environment is wired (even if unused at Stage 0)
	_, err := vaultlayer.GetVaultSettings(parsed)
	if err != nil {
		return err
	}

	server := webserver.New(&webserver.Config{
		Host:        s.Host,
		Port:        s.Port,
		DevMode:     s.DevMode,
	})

	// CORS is not yet enforced at Stage 0; placeholder parses the setting
	_ = strings.Split(s.CorsOrigins, ",")

	return server.Start(ctx)
}


