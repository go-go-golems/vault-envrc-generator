package cmds

import (
	"context"
	"fmt"
	"time"

	glzcli "github.com/go-go-golems/glazed/pkg/cli"
	gcmds "github.com/go-go-golems/glazed/pkg/cmds"
	glayers "github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/types"

	"github.com/go-go-golems/vault-envrc-generator/pkg/vault"
	"github.com/go-go-golems/vault-envrc-generator/pkg/vaultlayer"
)

type TokenCommand struct{ *gcmds.CommandDescription }

type TokenSettings struct {
	Keys []string `glazed.parameter:"keys"`
}

func NewTokenCommand() (*TokenCommand, error) {
	cmdLayer, err := glzcli.NewCommandSettingsLayer()
	if err != nil {
		return nil, err
	}

	cd := gcmds.NewCommandDescription(
		"token",
		gcmds.WithShort("Show Vault token details (for templates)"),
		gcmds.WithFlags(
			parameters.NewParameterDefinition("keys", parameters.ParameterTypeStringList, parameters.WithHelp("Limit to these keys (default: all)")),
		),
		gcmds.WithLayersList(cmdLayer),
	)
	_, err = vaultlayer.AddVaultLayerToCommand(cd)
	if err != nil {
		return nil, err
	}
	return &TokenCommand{cd}, nil
}

// Glaze-style command producing structured rows
func (c *TokenCommand) RunIntoGlazeProcessor(ctx context.Context, parsed *glayers.ParsedLayers, gp middlewares.Processor) error {
	s := &TokenSettings{}
	if err := parsed.InitializeStruct(glayers.DefaultSlug, s); err != nil {
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

	tctx, err := vault.BuildTemplateContext(client)
	if err != nil {
		return fmt.Errorf("failed to lookup token: %w", err)
	}

	// Flatten fields for output
	add := func(key string, value interface{}) error {
		// filter if specified
		if len(s.Keys) > 0 {
			found := false
			for _, f := range s.Keys {
				if f == key {
					found = true
					break
				}
			}
			if !found {
				return nil
			}
		}
		row := types.NewRow(
			types.MRP("key", key),
			types.MRP("value", value),
		)
		return gp.AddRow(ctx, row)
	}

	// Top-level token fields
	_ = add("accessor", tctx.Token.Accessor)
	_ = add("creation_ttl", tctx.Token.CreationTTL)
	_ = add("display_name", tctx.Token.DisplayName)
	_ = add("entity_id", tctx.Token.EntityID)
	_ = add("expire_time", tctx.Token.ExpireTime)
	_ = add("id", tctx.Token.ID)
	_ = add("issue_time", tctx.Token.IssueTime)
	_ = add("path", tctx.Token.Path)
	_ = add("ttl", tctx.Token.TTL)
	_ = add("type", tctx.Token.Type)
	_ = add("oidc_user_id", tctx.Token.OIDCUserID)

	// policies
	_ = add("policies", tctx.Token.Policies)
	// meta map
	_ = add("meta", tctx.Token.Meta)

	return nil
}

var _ gcmds.GlazeCommand = &TokenCommand{}
