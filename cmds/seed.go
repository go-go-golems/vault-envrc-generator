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
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"

	"github.com/go-go-golems/vault-envrc-generator/pkg/cmdutil"
	"github.com/go-go-golems/vault-envrc-generator/pkg/seed"
	"github.com/go-go-golems/vault-envrc-generator/pkg/vault"
	"github.com/go-go-golems/vault-envrc-generator/pkg/vaultlayer"
)

type SeedCommand struct{ *gcmds.CommandDescription }

type SeedSettings struct {
	Config    string   `glazed:"config"`
	DryRun    bool     `glazed:"dry-run"`
	Diff      bool     `glazed:"diff"`
	Confirm   bool     `glazed:"confirm"`
	BasePath  string   `glazed:"base-path"`
	Sets      []string `glazed:"sets"`
	Force     bool     `glazed:"force"`
	AllowCmd  bool     `glazed:"allow-commands"`
	ExtraKV   []string `glazed:"extra"`
	ExtraFile string   `glazed:"extra-file"`
}

func NewSeedCommand() (*SeedCommand, error) {
	section, err := glzcli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}

	cd := gcmds.NewCommandDescription(
		"seed",
		gcmds.WithShort("Seed Vault from env/files via YAML spec"),
		gcmds.WithFlags(
			fields.New("config", fields.TypeString, fields.WithRequired(true), fields.WithHelp("Seed YAML file"), fields.WithShortFlag("c")),
			fields.New("dry-run", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Preview without writing to Vault")),
			fields.New("diff", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Show a diff vs existing secrets (implied by --dry-run)")),
			fields.New("confirm", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Ask for confirmation after showing diff before applying")),
			fields.New("base-path", fields.TypeString, fields.WithHelp("Override base_path (supports Go templates)")),
			fields.New("sets", fields.TypeStringList, fields.WithHelp("Only seed sets whose path matches any of these; default all")),
			fields.New("force", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Overwrite existing keys without prompting")),
			fields.New("allow-commands", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Run commands in spec without confirmation")),
			fields.New("extra", fields.TypeStringList, fields.WithHelp("Additional template data key=value pairs")),
			fields.New("extra-file", fields.TypeString, fields.WithHelp("YAML or JSON file with additional template data")),
		),
		gcmds.WithSections(section),
	)
	_, err = vaultlayer.AddVaultSectionToCommand(cd)
	if err != nil {
		return nil, err
	}
	return &SeedCommand{cd}, nil
}

func (c *SeedCommand) Run(ctx context.Context, parsed *values.Values) error {
	s := &SeedSettings{}
	if err := parsed.DecodeSectionInto(schema.DefaultSlug, s); err != nil {
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
	spec.Sets = cmdutil.FilterItems(spec.Sets, s.Sets, func(st seed.Set) string { return st.Path }, func(st seed.Set) string { return st.Name })

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

	return seed.Run(client, &spec, seed.Options{
		DryRun:            s.DryRun,
		ForceOverwrite:    s.Force,
		AllowCommands:     s.AllowCmd,
		ExtraTemplateData: extra,
		ShowDiff:          s.Diff || s.DryRun,
		ConfirmApply:      s.Confirm,
	})
}

var _ gcmds.BareCommand = &SeedCommand{}
