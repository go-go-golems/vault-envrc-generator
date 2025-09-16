package glazed

import (
	"context"
	"fmt"
	"strings"
	"time"

	glayers "github.com/go-go-golems/glazed/pkg/cmds/layers"
	gmiddlewares "github.com/go-go-golems/glazed/pkg/cmds/middlewares"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"

	"github.com/go-go-golems/vault-envrc-generator/pkg/vault"
	"github.com/go-go-golems/vault-envrc-generator/pkg/vaultlayer"
)

// UpdateFromVault loads key-value pairs from a Vault secret path and updates
// matching parameters across all layers. A parameter is updated when a secret
// with the same name exists at the given path.
//
// Typical usage:
//
//	middlewares.ExecuteMiddlewares(layers, parsed,
//	    glazed.UpdateFromVault("kv/my-app/config",
//	        parameters.WithParseStepSource("vault"),
//	    ),
//	    middlewares.SetFromDefaults(parameters.WithParseStepSource("defaults")),
//	)
//
// Notes:
//   - Vault connection settings are read from the `vault` layer (see vaultlayer.NewVaultLayer).
//   - The secret path supports Go template expressions using the current token context,
//     via {{ .Token.* }} values (see vault.BuildTemplateContext).
func UpdateFromVault(path string, options ...parameters.ParseStepOption) gmiddlewares.Middleware {
	return func(next gmiddlewares.HandlerFunc) gmiddlewares.HandlerFunc {
		return func(layers *glayers.ParameterLayers, parsed *glayers.ParsedLayers) error {
			// Run the rest of the chain first; then apply Vault values.
			if err := next(layers, parsed); err != nil {
				return err
			}

			// Resolve Vault settings from the parsed layers
			vs, err := vaultlayer.GetVaultSettings(parsed)
			if err != nil {
				return fmt.Errorf("failed to parse vault settings: %w", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			token, err := vault.ResolveToken(ctx, vs.VaultToken, vault.TokenSource(vs.VaultTokenSource), vs.VaultTokenFile, false)
			if err != nil {
				return fmt.Errorf("failed to resolve Vault token: %w", err)
			}

			client, err := vault.NewClient(vs.VaultAddr, token)
			if err != nil {
				return fmt.Errorf("failed to create Vault client: %w", err)
			}

			// Support templated paths using the token context
			effectivePath := strings.TrimSpace(path)
			if effectivePath == "" {
				return fmt.Errorf("vault path is empty")
			}

			if strings.Contains(effectivePath, "{{") {
				tctx, err := vault.BuildTemplateContext(client)
				if err != nil {
					return fmt.Errorf("failed to build Vault template context: %w", err)
				}
				if rp, err := vault.RenderTemplateString(effectivePath, tctx); err == nil {
					effectivePath = rp
				} else {
					return fmt.Errorf("failed to render templated Vault path: %w", err)
				}
			}

			secrets, err := client.GetSecrets(effectivePath)
			if err != nil {
				return fmt.Errorf("failed to retrieve secrets from %s: %w", effectivePath, err)
			}

			// Update matching parameters across all layers
			err = layers.ForEachE(func(_ string, l glayers.ParameterLayer) error {
				parsedLayer := parsed.GetOrCreate(l)
				pds := l.GetParameterDefinitions()
				return pds.ForEachE(func(pd *parameters.ParameterDefinition) error {
					if v, ok := secrets[pd.Name]; ok {
						if err := parsedLayer.Parameters.UpdateValue(pd.Name, pd, v, options...); err != nil {
							return err
						}
					}
					return nil
				})
			})
			if err != nil {
				return err
			}

			return nil
		}
	}
}
