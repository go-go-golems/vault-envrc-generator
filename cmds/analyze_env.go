package cmds

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	glzcli "github.com/go-go-golems/glazed/pkg/cli"
	gcmds "github.com/go-go-golems/glazed/pkg/cmds"
	glayers "github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"gopkg.in/yaml.v3"

	"github.com/go-go-golems/vault-envrc-generator/pkg/analyze"
)

// AnalyzeEnvCommand exposes the analyze-env workflow over Glazed.
type AnalyzeEnvCommand struct {
	*gcmds.CommandDescription
	glazedLayers *settings.GlazedParameterLayers
}

// AnalyzeEnvSettings captures command line flags.
type AnalyzeEnvSettings struct {
	Config        string `glazed.parameter:"config"`
	EnvSource     string `glazed.parameter:"env-source"`
	EnvrcPath     string `glazed.parameter:"envrc"`
	DotenvPath    string `glazed.parameter:"dotenv"`
	EmptyEnv      bool   `glazed.parameter:"empty-env"`
	IncludeValues bool   `glazed.parameter:"include-values"`
	Censor        string `glazed.parameter:"censor"`
	ReportPath    string `glazed.parameter:"report"`
	Format        string `glazed.parameter:"format"`
	Strict        bool   `glazed.parameter:"strict"`
	ConfirmExec   bool   `glazed.parameter:"confirm-exec"`
}

// NewAnalyzeEnvCommand wires the analyze-env command into the CLI suite.
func NewAnalyzeEnvCommand() (*AnalyzeEnvCommand, error) {
	commandLayer, err := glzcli.NewCommandSettingsLayer()
	if err != nil {
		return nil, err
	}

	glazedLayers, err := settings.NewGlazedParameterLayers()
	if err != nil {
		return nil, err
	}

	cd := gcmds.NewCommandDescription(
		"analyze-env",
		gcmds.WithShort("Compare shell environment against a seed configuration"),
		gcmds.WithFlags(
			parameters.NewParameterDefinition("config", parameters.ParameterTypeString, parameters.WithRequired(true), parameters.WithHelp("Seed YAML configuration")),
			parameters.NewParameterDefinition("env-source", parameters.ParameterTypeChoice, parameters.WithChoices("current", "envrc", "direnv", "file"), parameters.WithDefault("current"), parameters.WithHelp("Where to read environment variables from")),
			parameters.NewParameterDefinition("envrc", parameters.ParameterTypeString, parameters.WithDefault(".envrc"), parameters.WithHelp("Path to .envrc when using env-source=envrc")),
			parameters.NewParameterDefinition("dotenv", parameters.ParameterTypeString, parameters.WithHelp("Path to .env or dotenv-style file when env-source=file")),
			parameters.NewParameterDefinition("empty-env", parameters.ParameterTypeBool, parameters.WithDefault(false), parameters.WithHelp("Source .envrc within an empty environment")),
			parameters.NewParameterDefinition("confirm-exec", parameters.ParameterTypeBool, parameters.WithDefault(false), parameters.WithHelp("Acknowledge that sourcing .envrc may execute code")),
			parameters.NewParameterDefinition("include-values", parameters.ParameterTypeBool, parameters.WithDefault(false), parameters.WithHelp("Include variable values in the report")),
			parameters.NewParameterDefinition("censor", parameters.ParameterTypeString, parameters.WithDefault("***"), parameters.WithHelp("Mask used when printing values")),
			parameters.NewParameterDefinition("format", parameters.ParameterTypeChoice, parameters.WithChoices("table", "yaml", "json"), parameters.WithDefault("table"), parameters.WithHelp("Preferred console output format")),
			parameters.NewParameterDefinition("report", parameters.ParameterTypeString, parameters.WithHelp("Optional path to write the full YAML or JSON report")),
			parameters.NewParameterDefinition("strict", parameters.ParameterTypeBool, parameters.WithDefault(false), parameters.WithHelp("Return non-zero exit code when issues are detected")),
		),
		gcmds.WithLayersList(glazedLayers, commandLayer),
	)

	return &AnalyzeEnvCommand{CommandDescription: cd, glazedLayers: glazedLayers}, nil
}

func (c *AnalyzeEnvCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedLayers *glayers.ParsedLayers,
	processor middlewares.Processor,
) error {
	settings := &AnalyzeEnvSettings{}
	if err := parsedLayers.InitializeStruct(glayers.DefaultSlug, settings); err != nil {
		return err
	}

	if err := c.overrideGlazedFormat(parsedLayers, settings.Format); err != nil {
		return err
	}

	opts := analyze.Options{
		SeedPath:    settings.Config,
		EnvSource:   analyze.EnvSource(settings.EnvSource),
		EnvrcPath:   settings.EnvrcPath,
		DotenvPath:  settings.DotenvPath,
		EmptyEnv:    settings.EmptyEnv,
		ConfirmExec: settings.ConfirmExec,
	}
	if settings.EnvSource == string(analyze.EnvSourceDirenv) {
		if settings.EnvrcPath != "" {
			opts.WorkingDir = filepath.Dir(settings.EnvrcPath)
		}
	}

	result, err := c.runAnalysis(ctx, opts, settings.Strict)
	var strictErr *analyze.StrictError
	if errors.As(err, &strictErr) {
		// Continue execution to emit output; return error after output is emitted.
	} else if err != nil {
		return err
	}

	masked := analyze.CloneWithMask(result, settings.IncludeValues, settings.Censor)

	if err := analyze.EmitRows(ctx, masked, settings.IncludeValues, settings.Censor, processor); err != nil {
		return err
	}

	if settings.ReportPath != "" {
		if err := writeReport(settings.ReportPath, masked); err != nil {
			return err
		}
	}

	if strictErr != nil {
		return strictErr
	}
	return nil
}

func (c *AnalyzeEnvCommand) overrideGlazedFormat(parsedLayers *glayers.ParsedLayers, format string) error {
	if format == "" {
		return nil
	}
	slug := c.glazedLayers.OutputParameterLayer.GetSlug()
	layer, err := glayers.NewParsedLayer(
		c.glazedLayers.OutputParameterLayer,
		glayers.WithParsedParameterValue("output", format),
	)
	if err != nil {
		return err
	}
	if existing, ok := parsedLayers.Get(slug); ok {
		return existing.MergeParameters(layer)
	}
	parsedLayers.Set(slug, layer)
	return nil
}

func (c *AnalyzeEnvCommand) runAnalysis(ctx context.Context, opts analyze.Options, strict bool) (*analyze.Result, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if strict {
		result, _, err := analyze.AnalyzeStrict(ctx, opts)
		return result, err
	}
	result, _, err := analyze.Analyze(ctx, opts)
	return result, err
}

func writeReport(path string, result *analyze.Result) error {
	if result == nil {
		return nil
	}
	ext := strings.ToLower(filepath.Ext(path))
	var data []byte
	var err error

	switch ext {
	case ".json":
		data, err = json.MarshalIndent(result, "", "  ")
	default:
		data, err = yaml.Marshal(result)
	}
	if err != nil {
		return err
	}

	if path == "-" {
		fmt.Println(string(data))
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

var _ gcmds.GlazeCommand = &AnalyzeEnvCommand{}
