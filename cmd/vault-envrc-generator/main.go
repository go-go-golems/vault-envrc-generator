package main

import (
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/logging"
	"github.com/go-go-golems/glazed/pkg/cmds/middlewares"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/help"
	help_cmd "github.com/go-go-golems/glazed/pkg/help/cmd"
	"github.com/spf13/cobra"
	clay "github.com/go-go-golems/clay/pkg"

	appcmds "github.com/go-go-golems/vault-envrc-generator/cmds"
	appdoc "github.com/go-go-golems/vault-envrc-generator/pkg/doc"
)

var version = "dev"

func getMiddlewares(parsedLayers *layers.ParsedLayers, cmd *cobra.Command, args []string) ([]middlewares.Middleware, error) {
	commandSettings := &cli.CommandSettings{}
	err := parsedLayers.InitializeStruct(cli.CommandSettingsSlug, commandSettings)
	if err != nil {
		return nil, err
	}

	mw_ := []middlewares.Middleware{
		middlewares.ParseFromCobraCommand(cmd,
			parameters.WithParseStepSource("cobra"),
		),
		middlewares.GatherArguments(args,
			parameters.WithParseStepSource("arguments"),
		),
	}

	mw_ = append(mw_,
		middlewares.GatherFlagsFromViper(parameters.WithParseStepSource("viper")),
		middlewares.SetFromDefaults(parameters.WithParseStepSource("defaults")),
	)

	return mw_, nil
}

func main() {
	rootCmd := &cobra.Command{
		Use:     "vault-envrc-generator",
		Short:   "Generate envrc and seed Vault via glazed commands",
		Version: version,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			err := logging.InitLoggerFromViper()
			cobra.CheckErr(err)
		},
	}

	clay.InitViper("vault-envrc-generator", rootCmd)

	// Help system
	hs := help.NewHelpSystem()
	_ = appdoc.AddDocToHelpSystem(hs)
	help_cmd.SetupCobraRootCommand(hs, rootCmd)

	opts := []cli.CobraOption{
            cli.WithParserConfig(cli.CobraParserConfig{
                // ShortHelpLayers: []string{layers.DefaultSlug},
                MiddlewaresFunc: getMiddlewares,
            }),
	}

	// Register commands
	if bc, err := appcmds.NewBatchCommand(); err == nil {
		cmd, err := cli.BuildCobraCommand(bc, opts...)
		cobra.CheckErr(err)
		rootCmd.AddCommand(cmd)
	} else {
		cobra.CheckErr(err)
	}

	if sc, err := appcmds.NewSeedCommand(); err == nil {
		cmd, err := cli.BuildCobraCommand(sc, opts...)
		cobra.CheckErr(err)
		rootCmd.AddCommand(cmd)
	} else {
		cobra.CheckErr(err)
	}

	if gc, err := appcmds.NewGenerateCommand(); err == nil {
		cmd, err := cli.BuildCobraCommand(gc, opts...)
		cobra.CheckErr(err)
		rootCmd.AddCommand(cmd)
	} else {
		cobra.CheckErr(err)
	}

	if lc, err := appcmds.NewListCommand(); err == nil {
		cmd, err := cli.BuildCobraCommand(lc, opts...)
		cobra.CheckErr(err)
		rootCmd.AddCommand(cmd)
	} else {
		cobra.CheckErr(err)
	}

	if tc, err := appcmds.NewTokenCommand(); err == nil {
		cmd, err := cli.BuildCobraCommand(tc, opts...)
		cobra.CheckErr(err)
		rootCmd.AddCommand(cmd)
	} else {
		cobra.CheckErr(err)
	}

	if ic, err := appcmds.NewInteractiveCommand(); err == nil {
		cmd, err := cli.BuildCobraCommand(ic, opts...)
		cobra.CheckErr(err)
		rootCmd.AddCommand(cmd)
	} else {
		cobra.CheckErr(err)
	}

	cobra.CheckErr(rootCmd.Execute())
}
