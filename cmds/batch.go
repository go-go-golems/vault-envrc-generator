package cmds

import (
	"context"
	"fmt"
	"os"
	"time"

	glzcli "github.com/go-go-golems/glazed/pkg/cli"
	gcmds "github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"gopkg.in/yaml.v3"

	"github.com/go-go-golems/vault-envrc-generator/pkg/batch"
	"github.com/go-go-golems/vault-envrc-generator/pkg/vault"
	"github.com/go-go-golems/vault-envrc-generator/pkg/vaultlayer"
)

type BatchCommand struct{ *gcmds.CommandDescription }

type BatchSettings struct {
	Config          string   `glazed:"config"`
	OutputOverride  string   `glazed:"output"`
	Format          string   `glazed:"format"`
	ContinueOnError bool     `glazed:"continue-on-error"`
	DryRun          bool     `glazed:"dry-run"`
	SortKeys        bool     `glazed:"sort-keys"`
	BasePath        string   `glazed:"base-path"`
	Jobs            []string `glazed:"jobs"`
	Sections        []string `glazed:"sections"`
	ForceOverwrite  bool     `glazed:"force-overwrite"`
	SkipUnreadable  bool     `glazed:"skip-unreadable"`
}

func NewBatchCommand() (*BatchCommand, error) {
	section, err := glzcli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}

	cd := gcmds.NewCommandDescription(
		"batch",
		gcmds.WithShort("Process multiple Vault paths from a YAML file"),
		gcmds.WithFlags(
			fields.New("config", fields.TypeString, fields.WithRequired(true), fields.WithHelp("Batch YAML file"), fields.WithShortFlag("c")),
			fields.New("base-path", fields.TypeString, fields.WithHelp("Base Vault path to prepend to relative section paths")),
			fields.New("output", fields.TypeString, fields.WithHelp("Override output for all jobs; '-' for stdout")),
			fields.New("format", fields.TypeString, fields.WithHelp("envrc|json|yaml")),
			fields.New("continue-on-error", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Continue processing on errors")),
			fields.New("dry-run", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Preview outputs without writing files")),
			fields.New("sort-keys", fields.TypeBool, fields.WithDefault(true), fields.WithHelp("Sort JSON/YAML keys for deterministic output")),
			fields.New("jobs", fields.TypeStringList, fields.WithHelp("Only process jobs with these names; default all")),
			fields.New("sections", fields.TypeStringList, fields.WithHelp("Only process sections with these names; default all")),
			fields.New("force-overwrite", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Overwrite .envrc without prompting")),
			fields.New("skip-unreadable", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Skip sections that cannot be read; warn instead of failing")),
		),
		gcmds.WithSections(section),
	)
	// attach vault section
	_, err = vaultlayer.AddVaultSectionToCommand(cd)
	if err != nil {
		return nil, err
	}
	return &BatchCommand{cd}, nil
}

func (c *BatchCommand) Run(ctx context.Context, parsed *values.Values) error {
	s := &BatchSettings{}
	if err := parsed.DecodeSectionInto(schema.DefaultSlug, s); err != nil {
		return err
	}
	vs, err := vaultlayer.GetVaultSettings(parsed)
	if err != nil {
		return err
	}

	ctx2, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	token, err := vault.ResolveToken(ctx2, vs.VaultToken, vault.TokenSource(vs.VaultTokenSource), vs.VaultTokenFile, false)
	if err != nil {
		return fmt.Errorf("failed to resolve Vault token: %w", err)
	}
	client, err := vault.NewClient(vs.VaultAddr, token)
	if err != nil {
		return fmt.Errorf("failed to create Vault client: %w", err)
	}

	cfg, err := loadBatchConfig(s.Config)
	if err != nil {
		return err
	}
	// Apply job/section filtering if requested (ignore empty selectors)
	if len(s.Jobs) > 0 {
		allowedJobs := map[string]struct{}{}
		for _, name := range s.Jobs {
			if name == "" {
				continue
			}
			allowedJobs[name] = struct{}{}
		}
		if len(allowedJobs) > 0 {
			filteredJobs := make([]batch.Job, 0, len(cfg.Jobs))
			for _, job := range cfg.Jobs {
				if _, ok := allowedJobs[job.Name]; ok {
					filteredJobs = append(filteredJobs, job)
				}
			}
			cfg.Jobs = filteredJobs
		}
	}
	if len(s.Sections) > 0 {
		allowedSecs := map[string]struct{}{}
		for _, name := range s.Sections {
			if name == "" {
				continue
			}
			allowedSecs[name] = struct{}{}
		}
		if len(allowedSecs) > 0 {
			for ji, job := range cfg.Jobs {
				if len(job.Sections) == 0 {
					continue
				}
				newSecs := make([]batch.Section, 0, len(job.Sections))
				for _, sec := range job.Sections {
					if _, ok := allowedSecs[sec.Name]; ok {
						newSecs = append(newSecs, sec)
					}
				}
				cfg.Jobs[ji].Sections = newSecs
			}
		}
	}
	proc := batch.Processor{Client: client}
	return proc.Process(cfg, batch.ProcessorOptions{
		BasePath:               s.BasePath,
		OutputOverride:         s.OutputOverride,
		FormatOverride:         s.Format,
		ContinueOnError:        s.ContinueOnError,
		DryRun:                 s.DryRun,
		SortKeys:               s.SortKeys,
		ForceOverwrite:         s.ForceOverwrite,
		SkipUnreadableSections: s.SkipUnreadable,
	})
}

var _ gcmds.BareCommand = &BatchCommand{}

func loadBatchConfig(filename string) (*batch.Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	var config batch.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML config: %w", err)
	}
	return &config, nil
}
