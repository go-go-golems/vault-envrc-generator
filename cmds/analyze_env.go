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
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"gopkg.in/yaml.v3"

	"github.com/go-go-golems/vault-envrc-generator/pkg/analyze"
)

// AnalyzeEnvCommand exposes the analyze-env workflow over Glazed.
type AnalyzeEnvCommand struct {
	*gcmds.CommandDescription
	glazedSection *settings.GlazedSection
}

// AnalyzeEnvSettings captures command line flags.
type AnalyzeEnvSettings struct {
	Config        string `glazed:"config"`
	EnvSource     string `glazed:"env-source"`
	EnvrcPath     string `glazed:"envrc"`
	DotenvPath    string `glazed:"dotenv"`
	EmptyEnv      bool   `glazed:"empty-env"`
	IncludeValues bool   `glazed:"include-values"`
	Censor        string `glazed:"censor"`
	ReportPath    string `glazed:"report"`
	Format        string `glazed:"format"`
	Strict        bool   `glazed:"strict"`
	ConfirmExec   bool   `glazed:"confirm-exec"`
}

// NewAnalyzeEnvCommand wires the analyze-env command into the CLI suite.
func NewAnalyzeEnvCommand() (*AnalyzeEnvCommand, error) {
	commandSection, err := glzcli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}

	glazedSection, err := settings.NewGlazedSection()
	if err != nil {
		return nil, err
	}

	cd := gcmds.NewCommandDescription(
		"analyze-env",
		gcmds.WithShort("Compare shell environment against a seed configuration"),
		gcmds.WithFlags(
			fields.New("config", fields.TypeString, fields.WithRequired(true), fields.WithHelp("Seed YAML configuration")),
			fields.New("env-source", fields.TypeChoice, fields.WithChoices("current", "envrc", "direnv", "file"), fields.WithDefault("current"), fields.WithHelp("Where to read environment variables from")),
			fields.New("envrc", fields.TypeString, fields.WithDefault(".envrc"), fields.WithHelp("Path to .envrc when using env-source=envrc")),
			fields.New("dotenv", fields.TypeString, fields.WithHelp("Path to .env or dotenv-style file when env-source=file")),
			fields.New("empty-env", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Source .envrc within an empty environment")),
			fields.New("confirm-exec", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Acknowledge that sourcing .envrc may execute code")),
			fields.New("include-values", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Include variable values in the report")),
			fields.New("censor", fields.TypeString, fields.WithDefault("***"), fields.WithHelp("Mask used when printing values")),
			fields.New("format", fields.TypeChoice, fields.WithChoices("table", "yaml", "json"), fields.WithDefault("table"), fields.WithHelp("Preferred console output format")),
			fields.New("report", fields.TypeString, fields.WithHelp("Optional path to write the full YAML or JSON report")),
			fields.New("strict", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Return non-zero exit code when issues are detected")),
		),
		gcmds.WithSections(glazedSection, commandSection),
	)

	return &AnalyzeEnvCommand{CommandDescription: cd, glazedSection: glazedSection}, nil
}

func (c *AnalyzeEnvCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	parsedValues *values.Values,
	processor middlewares.Processor,
) error {
	s := &AnalyzeEnvSettings{}
	if err := parsedValues.DecodeSectionInto(schema.DefaultSlug, s); err != nil {
		return err
	}

	if err := c.overrideGlazedFormat(parsedValues, s.Format); err != nil {
		return err
	}

	opts := analyze.Options{
		SeedPath:    s.Config,
		EnvSource:   analyze.EnvSource(s.EnvSource),
		EnvrcPath:   s.EnvrcPath,
		DotenvPath:  s.DotenvPath,
		EmptyEnv:    s.EmptyEnv,
		ConfirmExec: s.ConfirmExec,
	}
	if s.EnvSource == string(analyze.EnvSourceDirenv) {
		if s.EnvrcPath != "" {
			opts.WorkingDir = filepath.Dir(s.EnvrcPath)
		}
	}

	result, err := c.runAnalysis(ctx, opts, s.Strict)
	var strictErr *analyze.StrictError
	if errors.As(err, &strictErr) {
		// Continue execution to emit output; return error after output is emitted.
	} else if err != nil {
		return err
	}

	masked := analyze.CloneWithMask(result, s.IncludeValues, s.Censor)

	if err := analyze.EmitRows(ctx, masked, s.IncludeValues, s.Censor, processor); err != nil {
		return err
	}

	if s.ReportPath != "" {
		if err := writeReport(s.ReportPath, masked); err != nil {
			return err
		}
	}

	if strictErr != nil {
		return strictErr
	}
	return nil
}

func (c *AnalyzeEnvCommand) overrideGlazedFormat(parsedValues *values.Values, format string) error {
	if format == "" {
		return nil
	}
	sectionVals := parsedValues.GetOrCreate(c.glazedSection)
	defs := c.glazedSection.GetDefinitions()
	pd, ok := defs.Get("output")
	if !ok {
		return fmt.Errorf("glazed section has no 'output' field definition")
	}
	return sectionVals.Fields.UpdateValue("output", pd, format)
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
