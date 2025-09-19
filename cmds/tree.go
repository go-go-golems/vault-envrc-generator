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

	"github.com/go-go-golems/vault-envrc-generator/pkg/vault"
	"github.com/go-go-golems/vault-envrc-generator/pkg/vaultlayer"
)

type TreeCommand struct{ *gcmds.CommandDescription }

type TreeSettings struct {
	Path         string `glazed.parameter:"path"`
	Depth        int    `glazed.parameter:"depth"`
	Reveal       bool   `glazed.parameter:"reveal-values"`
	CensorPrefix int    `glazed.parameter:"censor-prefix"`
	CensorSuffix int    `glazed.parameter:"censor-suffix"`
}

func NewTreeCommand() (*TreeCommand, error) {
	layer, err := glzcli.NewCommandSettingsLayer()
	if err != nil {
		return nil, err
	}
	cd := gcmds.NewCommandDescription(
		"tree",
		gcmds.WithShort("Recursively print Vault tree as YAML (censored by default)"),
		gcmds.WithFlags(
			parameters.NewParameterDefinition("path", parameters.ParameterTypeString, parameters.WithRequired(true), parameters.WithShortFlag("p"), parameters.WithHelp("Root Vault path (prefix)")),
			parameters.NewParameterDefinition("depth", parameters.ParameterTypeInteger, parameters.WithDefault(0), parameters.WithHelp("Max depth (0 = unlimited)")),
			parameters.NewParameterDefinition("reveal-values", parameters.ParameterTypeBool, parameters.WithDefault(false), parameters.WithHelp("Reveal real values instead of censored")),
			parameters.NewParameterDefinition("censor-prefix", parameters.ParameterTypeInteger, parameters.WithDefault(2), parameters.WithHelp("Visible characters at start of value when censored")),
			parameters.NewParameterDefinition("censor-suffix", parameters.ParameterTypeInteger, parameters.WithDefault(2), parameters.WithHelp("Visible characters at end of value when censored")),
		),
		gcmds.WithLayersList(layer),
	)
	_, err = vaultlayer.AddVaultLayerToCommand(cd)
	if err != nil {
		return nil, err
	}
	return &TreeCommand{cd}, nil
}

func (c *TreeCommand) Run(ctx context.Context, parsed *glayers.ParsedLayers) error {
	s := &TreeSettings{}
	if err := parsed.InitializeStruct(glayers.DefaultSlug, s); err != nil {
		return err
	}
	vs, err := vaultlayer.GetVaultSettings(parsed)
	if err != nil {
		return err
	}

	ctx2, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	token, err := vault.ResolveToken(ctx2, vs.VaultToken, vault.TokenSource(vs.VaultTokenSource), vs.VaultTokenFile, false)
	if err != nil {
		return fmt.Errorf("failed to resolve Vault token: %w", err)
	}
	client, err := vault.NewClient(vs.VaultAddr, token)
	if err != nil {
		return fmt.Errorf("failed to create Vault client: %w", err)
	}

	root := vault.NormalizeListPath(s.Path)
	tree := map[string]interface{}{}

	var walk func(prefix string, depth int, node map[string]interface{}) error
	walk = func(prefix string, depth int, node map[string]interface{}) error {
		keys, err := client.ListSecrets(prefix)
		if err != nil {
			// Try leaf secret
			trimmed := strings.TrimSuffix(prefix, "/")
			if trimmed != "" {
				data, err2 := client.GetSecrets(trimmed)
				if err2 == nil {
					node["__secret__"] = materializeData(data, s.Reveal, s.CensorPrefix, s.CensorSuffix)
					return nil
				}
			}
			return err
		}

		// If no keys were returned, this might be a leaf secret; attempt to read it
		if len(keys) == 0 {
			trimmed := strings.TrimSuffix(prefix, "/")
			if trimmed != "" {
				if data, err2 := client.GetSecrets(trimmed); err2 == nil {
					node["__secret__"] = materializeData(data, s.Reveal, s.CensorPrefix, s.CensorSuffix)
					return nil
				}
			}
		}
		for _, k := range keys {
			if strings.HasSuffix(k, "/") {
				name := strings.TrimSuffix(k, "/")
				child := map[string]interface{}{}
				node[name] = child
				if depth != 1 {
					next := depth
					if next > 0 {
						next = depth - 1
					}
					if err := walk(prefix+k, next, child); err != nil {
						node[name+"__error__"] = err.Error()
					}
				}
			} else {
				// leaf entry
				leafPath := prefix + k
				data, err := client.GetSecrets(leafPath)
				if err != nil {
					node[k+"__error__"] = err.Error()
					continue
				}
				node[k] = materializeData(data, s.Reveal, s.CensorPrefix, s.CensorSuffix)
			}
		}
		return nil
	}

	if err := walk(root, s.Depth, tree); err != nil {
		return err
	}

	enc := yaml.NewEncoder(os.Stdout)
	enc.SetIndent(2)
	if err := enc.Encode(tree); err != nil {
		return err
	}
	_ = enc.Close()
	return nil
}

func materializeData(data map[string]interface{}, reveal bool, pre int, suf int) map[string]string {
	out := make(map[string]string, len(data))
	for k, v := range data {
		sval := fmt.Sprintf("%v", v)
		if reveal {
			out[k] = sval
		} else {
			out[k] = censorString(sval, pre, suf)
		}
	}
	return out
}

func censorString(s string, pre int, suf int) string {
	if pre < 0 {
		pre = 0
	}
	if suf < 0 {
		suf = 0
	}
	n := len(s)
	if n == 0 {
		return s
	}
	if pre+suf >= n {
		return strings.Repeat("*", n)
	}
	return s[:pre] + "..." + s[n-suf:]
}

var _ gcmds.BareCommand = &TreeCommand{}
