package cmds

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	glzcli "github.com/go-go-golems/glazed/pkg/cli"
	gcmds "github.com/go-go-golems/glazed/pkg/cmds"
	glayers "github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"

	"github.com/go-go-golems/vault-envrc-generator/pkg/seed"
	"github.com/go-go-golems/vault-envrc-generator/pkg/vault"
	"github.com/go-go-golems/vault-envrc-generator/pkg/vaultlayer"
)

type SeedCommand struct{ *gcmds.CommandDescription }

type SeedSettings struct {
	Config    string   `glazed.parameter:"config"`
	DryRun    bool     `glazed.parameter:"dry-run"`
	BasePath  string   `glazed.parameter:"base-path"`
	Sets      []string `glazed.parameter:"sets"`
	Force     bool     `glazed.parameter:"force"`
	AllowCmd  bool     `glazed.parameter:"allow-commands"`
	OnlyNew   bool     `glazed.parameter:"only-new"`
	ExtraKV   []string `glazed.parameter:"extra"`
	ExtraFile string   `glazed.parameter:"extra-file"`
}

func NewSeedCommand() (*SeedCommand, error) {
	layer, err := glzcli.NewCommandSettingsLayer()
	if err != nil {
		return nil, err
	}

	cd := gcmds.NewCommandDescription(
		"seed",
		gcmds.WithShort("Seed Vault from env/files via YAML spec"),
		gcmds.WithFlags(
			parameters.NewParameterDefinition("config", parameters.ParameterTypeString, parameters.WithRequired(true), parameters.WithHelp("Seed YAML file"), parameters.WithShortFlag("c")),
			parameters.NewParameterDefinition("dry-run", parameters.ParameterTypeBool, parameters.WithDefault(false), parameters.WithHelp("Preview without writing to Vault")),
			parameters.NewParameterDefinition("base-path", parameters.ParameterTypeString, parameters.WithHelp("Override base_path (supports Go templates)")),
			parameters.NewParameterDefinition("sets", parameters.ParameterTypeStringList, parameters.WithHelp("Only seed sets whose path matches any of these; default all")),
			parameters.NewParameterDefinition("force", parameters.ParameterTypeBool, parameters.WithDefault(false), parameters.WithHelp("Overwrite existing keys without prompting")),
			parameters.NewParameterDefinition("allow-commands", parameters.ParameterTypeBool, parameters.WithDefault(false), parameters.WithHelp("Run commands in spec without confirmation")),
			parameters.NewParameterDefinition("only-new", parameters.ParameterTypeBool, parameters.WithDefault(false), parameters.WithHelp("Only write keys that don't already exist at the target path")),
			parameters.NewParameterDefinition("extra", parameters.ParameterTypeStringList, parameters.WithHelp("Additional template data key=value pairs")),
			parameters.NewParameterDefinition("extra-file", parameters.ParameterTypeString, parameters.WithHelp("YAML or JSON file with additional template data")),
		),
		gcmds.WithLayersList(layer),
	)
	_, err = vaultlayer.AddVaultLayerToCommand(cd)
	if err != nil {
		return nil, err
	}
	return &SeedCommand{cd}, nil
}

func (c *SeedCommand) Run(ctx context.Context, parsed *glayers.ParsedLayers) error {
	s := &SeedSettings{}
	if err := parsed.InitializeStruct(glayers.DefaultSlug, s); err != nil {
		return err
	}
	// Allow overriding base_path via CLI flag
	if s.BasePath != "" {
		viper.Set("seed.base_path", s.BasePath)
	}
	vs, err := vaultlayer.GetVaultSettings(parsed)
	if err != nil {
		return err
	}

	ctx2, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	token, err := vault.ResolveToken(ctx2, vs.VaultToken, vault.TokenSource(vs.VaultTokenSource), vs.VaultTokenFile, false)
	if err != nil {
		return fmt.Errorf("failed to resolve Vault token: %w", err)
	}
	client, err := vault.NewClient(vs.VaultAddr, token)
	if err != nil {
		return fmt.Errorf("failed to create Vault client: %w", err)
	}

	b, err := os.ReadFile(s.Config)
	if err != nil {
		return fmt.Errorf("failed to read seed config: %w", err)
	}
	var spec seed.Spec
	if err := yaml.Unmarshal(b, &spec); err != nil {
		return fmt.Errorf("failed to parse seed YAML: %w", err)
	}

	// CLI flag should override YAML base_path when provided
	if s.BasePath != "" {
		spec.BasePath = s.BasePath
	}

	// Filter sets if requested (match by name or path) and ignoring empty selectors
	if len(s.Sets) > 0 {
		allowed := map[string]struct{}{}
		for _, p := range s.Sets {
			if p == "" {
				continue
			}
			allowed[p] = struct{}{}
		}
		if len(allowed) > 0 {
			filtered := make([]seed.Set, 0, len(spec.Sets))
			for _, st := range spec.Sets {
				if _, ok := allowed[st.Path]; ok {
					filtered = append(filtered, st)
					continue
				}
				if st.Name != "" {
					if _, ok := allowed[st.Name]; ok {
						filtered = append(filtered, st)
					}
				}
			}
			spec.Sets = filtered
		}
	}

	// Build extra template data from flags
	extra := map[string]interface{}{}
	for _, kv := range s.ExtraKV {
		if kv == "" {
			continue
		}
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 {
			extra[parts[0]] = parts[1]
		}
	}
	if s.ExtraFile != "" {
		content, err := os.ReadFile(s.ExtraFile)
		if err != nil {
			return fmt.Errorf("failed to read extra-file: %w", err)
		}
		// Try YAML first, then JSON
		var obj map[string]interface{}
		if err := yaml.Unmarshal(content, &obj); err != nil {
			var objJSON map[string]interface{}
			if err2 := json.Unmarshal(content, &objJSON); err2 == nil {
				for k, v := range objJSON {
					extra[k] = v
				}
			} else {
				return fmt.Errorf("failed to parse extra-file as YAML or JSON: %v / %v", err, err2)
			}
		} else {
			for k, v := range obj {
				extra[k] = v
			}
		}
	}

	return seed.Run(client, &spec, seed.Options{DryRun: s.DryRun, ForceOverwrite: s.Force, AllowCommands: s.AllowCmd, ExtraTemplateData: extra, OnlyNew: s.OnlyNew})
}

var _ gcmds.BareCommand = &SeedCommand{}
