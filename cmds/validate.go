package cmds

import (
	"context"
	"fmt"
	"os"

	glzcli "github.com/go-go-golems/glazed/pkg/cli"
	gcmds "github.com/go-go-golems/glazed/pkg/cmds"
	glayers "github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"
	"gopkg.in/yaml.v3"

	"github.com/go-go-golems/vault-envrc-generator/pkg/batch"
	"github.com/go-go-golems/vault-envrc-generator/pkg/seed"
)

type ValidateCommand struct{ *gcmds.CommandDescription }

type ValidateSettings struct {
	SeedConfig  string `glazed.parameter:"seed-config"`
	BatchConfig string `glazed.parameter:"batch-config"`
}

func NewValidateCommand() (*ValidateCommand, error) {
	glazedLayers, err := settings.NewGlazedParameterLayers()
	if err != nil {
		return nil, err
	}
	commandLayer, err := glzcli.NewCommandSettingsLayer()
	if err != nil {
		return nil, err
	}
	cd := gcmds.NewCommandDescription(
		"validate",
		gcmds.WithShort("Validate seed env vars and cross-check with batch requirements"),
		gcmds.WithFlags(
			parameters.NewParameterDefinition("seed-config", parameters.ParameterTypeString, parameters.WithRequired(true), parameters.WithShortFlag("s"), parameters.WithHelp("Path to seed YAML config")),
			parameters.NewParameterDefinition("batch-config", parameters.ParameterTypeString, parameters.WithShortFlag("b"), parameters.WithHelp("Path to batch YAML config (optional)")),
		),
		gcmds.WithLayersList(glazedLayers, commandLayer),
	)
	return &ValidateCommand{cd}, nil
}

func (c *ValidateCommand) RunIntoGlazeProcessor(ctx context.Context, parsed *glayers.ParsedLayers, gp middlewares.Processor) error {
	s := &ValidateSettings{}
	if err := parsed.InitializeStruct(glayers.DefaultSlug, s); err != nil {
		return err
	}

	// Load seed spec
	seedData, err := os.ReadFile(s.SeedConfig)
	if err != nil {
		return fmt.Errorf("failed to read seed config: %w", err)
	}
	var spec seed.Spec
	if err := yaml.Unmarshal(seedData, &spec); err != nil {
		return fmt.Errorf("failed to parse seed YAML: %w", err)
	}

	// Build seed path -> keys set, and collect env var checks
	type void struct{}
	var exists void
	seedKeysByPath := map[string]map[string]void{}

	for _, st := range spec.Sets {
		if _, ok := seedKeysByPath[st.Path]; !ok {
			seedKeysByPath[st.Path] = map[string]void{}
		}
		// Data keys
		for k := range st.Data {
			seedKeysByPath[st.Path][k] = exists
		}
		// Env keys (also verify env var presence)
		for k, envName := range st.Env {
			seedKeysByPath[st.Path][k] = exists
			if _, ok := os.LookupEnv(envName); !ok {
				row := types.NewRow(
					types.MRP("type", "seed_env_missing"),
					types.MRP("set", st.Name),
					types.MRP("path", st.Path),
					types.MRP("key", k),
					types.MRP("env_var", envName),
				)
				if err := gp.AddRow(ctx, row); err != nil {
					return err
				}
			}
		}
		// Files keys
		for k := range st.Files {
			seedKeysByPath[st.Path][k] = exists
		}
	}

	// If no batch config provided, we're done after env validation
	if s.BatchConfig == "" {
		return nil
	}

	// Load batch config
	batchData, err := os.ReadFile(s.BatchConfig)
	if err != nil {
		return fmt.Errorf("failed to read batch config: %w", err)
	}
	var bcfg batch.Config
	if err := yaml.Unmarshal(batchData, &bcfg); err != nil {
		return fmt.Errorf("failed to parse batch YAML: %w", err)
	}

	// Build batch path -> required keys and env var mappings
	requiredKeysByPath := map[string]map[string]void{}
	envVarByPathAndKey := map[string]map[string][]string{}

	ensureMap := func(m map[string]map[string]void, k string) {
		if _, ok := m[k]; !ok {
			m[k] = map[string]void{}
		}
	}
	ensureEnvMap := func(m map[string]map[string][]string, k string) {
		if _, ok := m[k]; !ok {
			m[k] = map[string][]string{}
		}
	}

	for _, job := range bcfg.Jobs {
		if len(job.Sections) == 0 {
			// Legacy single-path job
			p := job.Path
			if p != "" {
				ensureMap(requiredKeysByPath, p)
			}
			for _, k := range job.IncludeKeys {
				requiredKeysByPath[p][k] = exists
			}
		} else {
			for _, sec := range job.Sections {
				p := sec.Path
				ensureMap(requiredKeysByPath, p)
				ensureEnvMap(envVarByPathAndKey, p)

				// include_keys
				for _, k := range sec.IncludeKeys {
					requiredKeysByPath[p][k] = exists
				}
				// env_map (target env var -> source key)
				for envName, srcKey := range sec.EnvMap {
					requiredKeysByPath[p][srcKey] = exists
					envVarByPathAndKey[p][srcKey] = append(envVarByPathAndKey[p][srcKey], envName)
				}
			}
		}
	}

	// Compare: batch-required keys missing in seed
	for p, req := range requiredKeysByPath {
		seedKeys := seedKeysByPath[p]
		for k := range req {
			if _, ok := seedKeys[k]; !ok {
				row := types.NewRow(
					types.MRP("type", "batch_missing_key"),
					types.MRP("path", p),
					types.MRP("key", k),
					types.MRP("env_vars", envVarByPathAndKey[p][k]),
				)
				if err := gp.AddRow(ctx, row); err != nil {
					return err
				}
			}
		}
	}

	// Compare: seed keys not referenced by batch
	for p, sk := range seedKeysByPath {
		req := requiredKeysByPath[p]
		for k := range sk {
			if _, ok := req[k]; !ok {
				row := types.NewRow(
					types.MRP("type", "seed_key_unused"),
					types.MRP("path", p),
					types.MRP("key", k),
				)
				if err := gp.AddRow(ctx, row); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

var _ gcmds.GlazeCommand = &ValidateCommand{}
