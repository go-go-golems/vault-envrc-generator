package cmds

import (
	"context"
	"fmt"
	"time"

	glzcli "github.com/go-go-golems/glazed/pkg/cli"
	gcmds "github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"

	"github.com/go-go-golems/vault-envrc-generator/pkg/diffenv"
	"github.com/go-go-golems/vault-envrc-generator/pkg/vault"
	"github.com/go-go-golems/vault-envrc-generator/pkg/vaultlayer"
)

type DiffEnvCommand struct{ *gcmds.CommandDescription }

type DiffEnvSettings struct {
	SeedPath  string `glazed:"seed"`
	BatchPath string `glazed:"batch"`
	BasePath  string `glazed:"base-path"`
	ShowExtra bool   `glazed:"show-extra"`
	Reveal    bool   `glazed:"reveal-values"`
	CensorPre int    `glazed:"censor-prefix"`
	CensorSuf int    `glazed:"censor-suffix"`
}

func NewDiffEnvCommand() (*DiffEnvCommand, error) {
	section, err := glzcli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}

	cd := gcmds.NewCommandDescription(
		"diff-env",
		gcmds.WithShort("Diff current environment against Vault (seed or batch mapping)"),
		gcmds.WithFlags(
			fields.New("seed", fields.TypeString, fields.WithHelp("Seed YAML file to derive env mapping")),
			fields.New("batch", fields.TypeString, fields.WithHelp("Batch YAML file to derive env mapping")),
			fields.New("base-path", fields.TypeString, fields.WithHelp("Override base_path for template rendering")),
			fields.New("show-extra", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Also list env vars not in mapping")),
			fields.New("reveal-values", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Reveal real values instead of censored")),
			fields.New("censor-prefix", fields.TypeInteger, fields.WithDefault(2), fields.WithHelp("Visible characters at start of value when censored")),
			fields.New("censor-suffix", fields.TypeInteger, fields.WithDefault(2), fields.WithHelp("Visible characters at end of value when censored")),
		),
		gcmds.WithSections(section),
	)
	_, err = vaultlayer.AddVaultSectionToCommand(cd)
	if err != nil {
		return nil, err
	}
	return &DiffEnvCommand{cd}, nil
}

func (c *DiffEnvCommand) Run(ctx context.Context, parsed *values.Values) error {
	s := &DiffEnvSettings{}
	if err := parsed.DecodeSectionInto(schema.DefaultSlug, s); err != nil {
		return err
	}
	if s.SeedPath == "" && s.BatchPath == "" {
		return fmt.Errorf("one of --seed or --batch must be provided")
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

	res, err := diffenv.Compute(client, diffenv.Options{SeedPath: s.SeedPath, BatchPath: s.BatchPath, BasePath: s.BasePath, IncludeExtra: s.ShowExtra})
	if err != nil {
		return err
	}

	// Print compact report
	fmt.Printf("Matches: %d, Changed: %d, Missing: %d, Extra: %d\n", len(res.Matches), len(res.Changed), len(res.MissingInEnv), len(res.ExtraInEnv))
	for _, e := range res.Changed {
		envVal := e.Env
		vaultVal := e.Vault
		if !s.Reveal {
			envVal = censorString(envVal, s.CensorPre, s.CensorSuf)
			vaultVal = censorString(vaultVal, s.CensorPre, s.CensorSuf)
		}
		fmt.Printf("~ %s\n  env=%q\n  vault=%q\n  path=%s\n", e.Name, envVal, vaultVal, e.Path)
	}
	for _, e := range res.MissingInEnv {
		v := e.Value
		if !s.Reveal {
			v = censorString(v, s.CensorPre, s.CensorSuf)
		}
		fmt.Printf("- %s\n  vault=%q\n  path=%s\n", e.Name, v, e.Path)
	}
	for _, e := range res.ExtraInEnv {
		v := e.Value
		if !s.Reveal {
			v = censorString(v, s.CensorPre, s.CensorSuf)
		}
		fmt.Printf("+ %s\n  env=%q\n", e.Name, v)
	}
	return nil
}

var _ gcmds.BareCommand = &DiffEnvCommand{}
