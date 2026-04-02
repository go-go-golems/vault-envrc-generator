package glazed

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/sources"
	"github.com/go-go-golems/glazed/pkg/cmds/values"

	"github.com/go-go-golems/vault-envrc-generator/pkg/vault"
	"github.com/go-go-golems/vault-envrc-generator/pkg/vaultlayer"
)

// UpdateFromVault loads key-value pairs from a Vault secret path and updates
// matching fields across all sections. A field is updated when a secret
// with the same name exists at the given path.
//
// Typical usage:
//
//	sources.Execute(schema_, parsed,
//	    glazed.UpdateFromVault("kv/my-app/config",
//	        fields.WithSource("vault"),
//	    ),
//	    sources.FromDefaults(fields.WithSource("defaults")),
//	)
//
// Notes:
//   - Vault connection settings are read from the `vault` section (see vaultlayer.NewVaultSection).
//   - The secret path supports Go template expressions using the current token context,
//     via {{ .Token.* }} values (see vault.BuildTemplateContext).
func UpdateFromVault(path string, options ...fields.ParseOption) sources.Middleware {
	return func(next sources.HandlerFunc) sources.HandlerFunc {
		return func(schema_ *schema.Schema, parsedValues *values.Values) error {
			// Run the rest of the chain first; then apply Vault values.
			if err := next(schema_, parsedValues); err != nil {
				return err
			}

			// Resolve Vault settings from the parsed values
			vs, err := vaultlayer.GetVaultSettings(parsedValues)
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

			// Update matching fields across all sections
			err = schema_.ForEachE(func(_ string, s schema.Section) error {
				sectionVals := parsedValues.GetOrCreate(s)
				defs := s.GetDefinitions()
				return defs.ForEachE(func(pd *fields.Definition) error {
					if v, ok := secrets[pd.Name]; ok {
						if err := sectionVals.Fields.UpdateValue(pd.Name, pd, v, options...); err != nil {
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
