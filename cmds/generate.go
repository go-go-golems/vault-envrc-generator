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

	"github.com/go-go-golems/vault-envrc-generator/pkg/envrc"
	"github.com/go-go-golems/vault-envrc-generator/pkg/output"
	"github.com/go-go-golems/vault-envrc-generator/pkg/vault"
	"github.com/go-go-golems/vault-envrc-generator/pkg/vaultlayer"
)

type GenerateCommand struct{ *gcmds.CommandDescription }

type GenerateSettings struct {
	Path          string   `glazed:"path"`
	TemplateFile  string   `glazed:"template"`
	Prefix        string   `glazed:"prefix"`
	ExcludeKeys   []string `glazed:"exclude"`
	IncludeKeys   []string `glazed:"include"`
	TransformKeys bool     `glazed:"transform-keys"`
	DryRun        bool     `glazed:"dry-run"`
	Format        string   `glazed:"format"`
	Output        string   `glazed:"output"`
	SortKeys      bool     `glazed:"sort-keys"`
}

func NewGenerateCommand() (*GenerateCommand, error) {
	section, err := glzcli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}
	cd := gcmds.NewCommandDescription(
		"generate",
		gcmds.WithShort("Generate .envrc/json/yaml from a single Vault path"),
		gcmds.WithFlags(
			fields.New("path", fields.TypeString, fields.WithRequired(true), fields.WithShortFlag("p"), fields.WithHelp("Vault path")),
			fields.New("template", fields.TypeString, fields.WithHelp("Custom template file")),
			fields.New("prefix", fields.TypeString, fields.WithHelp("Prefix to add to keys")),
			fields.New("exclude", fields.TypeStringList, fields.WithHelp("Keys to exclude")),
			fields.New("include", fields.TypeStringList, fields.WithHelp("Keys to include (overrides exclude)")),
			fields.New("transform-keys", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Transform keys to UPPER and '-' to '_'")),
			fields.New("dry-run", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Print to stdout instead of writing")),
			fields.New("format", fields.TypeChoice, fields.WithChoices("envrc", "json", "yaml"), fields.WithDefault("envrc"), fields.WithHelp("Output format")),
			fields.New("output", fields.TypeString, fields.WithDefault("-"), fields.WithHelp("Output path or '-' for stdout")),
			fields.New("sort-keys", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Sort keys in JSON/YAML")),
		),
		gcmds.WithSections(section),
	)
	_, err = vaultlayer.AddVaultSectionToCommand(cd)
	if err != nil {
		return nil, err
	}
	return &GenerateCommand{cd}, nil
}

func (c *GenerateCommand) Run(ctx context.Context, parsed *values.Values) error {
	s := &GenerateSettings{}
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

	secrets, err := client.GetSecrets(s.Path)
	if err != nil {
		return fmt.Errorf("failed to retrieve secrets: %w", err)
	}

	gen := envrc.NewGenerator(&envrc.Options{
		Prefix:        s.Prefix,
		ExcludeKeys:   s.ExcludeKeys,
		IncludeKeys:   s.IncludeKeys,
		TransformKeys: s.TransformKeys,
		Format:        s.Format,
		TemplateFile:  s.TemplateFile,
		Verbose:       false,
		SortKeys:      s.SortKeys,
	})
	content, err := gen.Generate(secrets)
	if err != nil {
		return err
	}

	if s.DryRun || s.Output == "-" {
		fmt.Print(content)
		if s.Format == "envrc" {
			fmt.Print("\n")
		}
		return nil
	}
	return output.Write(s.Output, []byte(content), output.WriteOptions{Format: s.Format, SortKeys: s.SortKeys})
}

var _ gcmds.BareCommand = &GenerateCommand{}
