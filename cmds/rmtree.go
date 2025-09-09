package cmds

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	glzcli "github.com/go-go-golems/glazed/pkg/cli"
	gcmds "github.com/go-go-golems/glazed/pkg/cmds"
	glayers "github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"gopkg.in/yaml.v3"

	"github.com/go-go-golems/vault-envrc-generator/pkg/listing"
	"github.com/go-go-golems/vault-envrc-generator/pkg/vault"
	"github.com/go-go-golems/vault-envrc-generator/pkg/vaultlayer"
)

func askForConfirmation(prompt string) bool {
	fmt.Print(prompt)
	var line string
	_, _ = fmt.Scanln(&line)
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	default:
		return false
	}
}

type RmTreeCommand struct{ *gcmds.CommandDescription }

type RmTreeSettings struct {
	Path  string `glazed.parameter:"path"`
	Depth int    `glazed.parameter:"depth"`
	Yes   bool   `glazed.parameter:"yes"`
}

func NewRmTreeCommand() (*RmTreeCommand, error) {
	layer, err := glzcli.NewCommandSettingsLayer()
	if err != nil {
		return nil, err
	}
	cd := gcmds.NewCommandDescription(
		"rm-tree",
		gcmds.WithShort("Print a Vault tree and delete all leaves under it after confirmation"),
		gcmds.WithFlags(
			parameters.NewParameterDefinition("path", parameters.ParameterTypeString, parameters.WithRequired(true), parameters.WithShortFlag("p"), parameters.WithHelp("Root Vault path to delete")),
			parameters.NewParameterDefinition("depth", parameters.ParameterTypeInteger, parameters.WithDefault(0), parameters.WithHelp("Max depth to scan before delete (0 = unlimited)")),
			parameters.NewParameterDefinition("yes", parameters.ParameterTypeBool, parameters.WithDefault(false), parameters.WithHelp("Skip confirmation prompt and delete immediately")),
		),
		gcmds.WithLayersList(layer),
	)
	_, err = vaultlayer.AddVaultLayerToCommand(cd)
	if err != nil {
		return nil, err
	}
	return &RmTreeCommand{cd}, nil
}

func (c *RmTreeCommand) Run(ctx context.Context, parsed *glayers.ParsedLayers) error {
	s := &RmTreeSettings{}
	if err := parsed.InitializeStruct(glayers.DefaultSlug, s); err != nil {
		return err
	}
	vs, err := vaultlayer.GetVaultSettings(parsed)
	if err != nil {
		return err
	}

	ctx2, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	token, err := vault.ResolveToken(ctx2, vs.VaultToken, vault.TokenSource(vs.VaultTokenSource), vs.VaultTokenFile, false)
	if err != nil {
		return fmt.Errorf("failed to resolve Vault token: %w", err)
	}
	client, err := vault.NewClient(vs.VaultAddr, token)
	if err != nil {
		return fmt.Errorf("failed to create Vault client: %w", err)
	}

	// Print the tree (list of paths)
	keys, errs := listing.Walk(client, s.Path, s.Depth)
	out := map[string]interface{}{"paths": keys}
	enc := yaml.NewEncoder(os.Stdout)
	enc.SetIndent(2)
	_ = enc.Encode(out)
	_ = enc.Close()
	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "warning: %v\n", e)
	}
	if len(keys) == 0 {
		fmt.Fprintln(os.Stderr, "nothing to delete")
		return nil
	}

	if !s.Yes {
		if !askForConfirmation(fmt.Sprintf("Delete %d secrets under '%s'? [y/N]: ", len(keys), s.Path)) {
			fmt.Fprintln(os.Stderr, "aborted")
			return nil
		}
	}

	// Delete leaf secrets (skip directories)
	deleted := 0
	for _, p := range keys {
		if strings.HasSuffix(p, "/") {
			continue
		}
		if err := client.DeleteSecret(p); err != nil {
			fmt.Fprintf(os.Stderr, "failed to delete %s: %v\n", p, err)
		} else {
			deleted++
		}
	}
	fmt.Fprintf(os.Stdout, "deleted %d secrets\n", deleted)
	return nil
}

var _ gcmds.BareCommand = &RmTreeCommand{}
