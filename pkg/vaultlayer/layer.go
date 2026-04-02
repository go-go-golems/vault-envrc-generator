package vaultlayer

import (
	"fmt"

	glzcms "github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
)

const VaultLayerSlug = "vault"

type VaultSettings struct {
	VaultAddr        string `glazed:"vault-addr"`
	VaultToken       string `glazed:"vault-token"`
	VaultTokenSource string `glazed:"vault-token-source"`
	VaultTokenFile   string `glazed:"vault-token-file"`
}

// NewVaultSection defines a reusable section for Vault configuration.
func NewVaultSection() (schema.Section, error) {
	return schema.NewSection(
		VaultLayerSlug,
		"Vault connection settings",
		schema.WithFields(
			fields.New(
				"vault-addr",
				fields.TypeString,
				fields.WithHelp("Vault server address"),
				fields.WithDefault("http://127.0.0.1:8200"),
			),
			fields.New(
				"vault-token",
				fields.TypeString,
				fields.WithHelp("Vault token (optional)"),
				fields.WithDefault(""),
			),
			fields.New(
				"vault-token-source",
				fields.TypeChoice,
				fields.WithHelp("Token source: auto|env|file|lookup"),
				fields.WithDefault("auto"),
				fields.WithChoices("auto", "env", "file", "lookup"),
			),
			fields.New(
				"vault-token-file",
				fields.TypeString,
				fields.WithHelp("Path to token file (default ~/.vault-token)"),
				fields.WithDefault(""),
			),
		),
	)
}

// AddVaultSectionToCommand attaches the vault section to a Glazed command description.
func AddVaultSectionToCommand(c glzcms.Command) (glzcms.Command, error) {
	s, err := NewVaultSection()
	if err != nil {
		return nil, err
	}
	c.Description().Schema.Set(VaultLayerSlug, s)
	return c, nil
}

// GetVaultSettings returns parsed vault settings from the Values.
func GetVaultSettings(parsed *values.Values) (*VaultSettings, error) {
	var s VaultSettings
	if err := parsed.DecodeSectionInto(VaultLayerSlug, &s); err != nil {
		return nil, fmt.Errorf("failed to parse vault settings: %w", err)
	}
	return &s, nil
}
