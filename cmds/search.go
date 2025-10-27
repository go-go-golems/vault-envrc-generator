package cmds

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	glzcli "github.com/go-go-golems/glazed/pkg/cli"
	gcmds "github.com/go-go-golems/glazed/pkg/cmds"
	glayers "github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"
	"github.com/go-go-golems/glazed/pkg/types"

	"github.com/go-go-golems/vault-envrc-generator/pkg/listing"
	"github.com/go-go-golems/vault-envrc-generator/pkg/vault"
	"github.com/go-go-golems/vault-envrc-generator/pkg/vaultlayer"
)

type SearchCommand struct{ *gcmds.CommandDescription }

type SearchSettings struct {
	Path          string   `glazed.parameter:"path"`
	KeyContains   []string `glazed.parameter:"key-contains"`
	KeyRegexp     []string `glazed.parameter:"key-regexp"`
	ValueContains []string `glazed.parameter:"value-contains"`
	ValueRegexp   []string `glazed.parameter:"value-regexp"`
	IgnoreCase    bool     `glazed.parameter:"ignore-case"`
	Depth         int      `glazed.parameter:"depth"`
	Reveal        bool     `glazed.parameter:"reveal-values"`
	CensorPrefix  int      `glazed.parameter:"censor-prefix"`
	CensorSuffix  int      `glazed.parameter:"censor-suffix"`
	IncludeAudit  bool     `glazed.parameter:"include-audit"`
	AuditLimit    int      `glazed.parameter:"audit-limit"`
}

func NewSearchCommand() (*SearchCommand, error) {
	glazedLayers, err := settings.NewGlazedParameterLayers()
	if err != nil {
		return nil, err
	}
	commandLayer, err := glzcli.NewCommandSettingsLayer()
	if err != nil {
		return nil, err
	}

	cd := gcmds.NewCommandDescription(
		"search",
		gcmds.WithShort("Search Vault secrets for matching keys or values"),
		gcmds.WithFlags(
			parameters.NewParameterDefinition("path", parameters.ParameterTypeString, parameters.WithRequired(true), parameters.WithShortFlag("p"), parameters.WithHelp("Root Vault path to search")),
			parameters.NewParameterDefinition("key-contains", parameters.ParameterTypeStringList, parameters.WithHelp("Substring filters to match against keys")),
			parameters.NewParameterDefinition("key-regexp", parameters.ParameterTypeStringList, parameters.WithHelp("Regular expression filters to match against keys")),
			parameters.NewParameterDefinition("value-contains", parameters.ParameterTypeStringList, parameters.WithHelp("Substring filters to match against values")),
			parameters.NewParameterDefinition("value-regexp", parameters.ParameterTypeStringList, parameters.WithHelp("Regular expression filters to match against values")),
			parameters.NewParameterDefinition("ignore-case", parameters.ParameterTypeBool, parameters.WithDefault(false), parameters.WithHelp("Perform case-insensitive matching")),
			parameters.NewParameterDefinition("depth", parameters.ParameterTypeInteger, parameters.WithDefault(0), parameters.WithHelp("Maximum recursion depth (0 = unlimited)")),
			parameters.NewParameterDefinition("reveal-values", parameters.ParameterTypeBool, parameters.WithDefault(false), parameters.WithHelp("Show full values instead of censored previews")),
			parameters.NewParameterDefinition("censor-prefix", parameters.ParameterTypeInteger, parameters.WithDefault(2), parameters.WithHelp("Visible characters at the start of censored values")),
			parameters.NewParameterDefinition("censor-suffix", parameters.ParameterTypeInteger, parameters.WithDefault(2), parameters.WithHelp("Visible characters at the end of censored values")),
			parameters.NewParameterDefinition("include-audit", parameters.ParameterTypeBool, parameters.WithDefault(false), parameters.WithHelp("Include KV metadata (audit trail) for matches")),
			parameters.NewParameterDefinition("audit-limit", parameters.ParameterTypeInteger, parameters.WithDefault(5), parameters.WithHelp("Maximum number of recent versions to display when including audit metadata (0 = all)")),
		),
		gcmds.WithLayersList(glazedLayers, commandLayer),
	)

	_, err = vaultlayer.AddVaultLayerToCommand(cd)
	if err != nil {
		return nil, err
	}

	return &SearchCommand{cd}, nil
}

func (c *SearchCommand) RunIntoGlazeProcessor(ctx context.Context, parsed *glayers.ParsedLayers, gp middlewares.Processor) error {
	s := &SearchSettings{}
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

	keyMatchers, err := buildPatternMatchers(s.KeyContains, s.KeyRegexp, s.IgnoreCase)
	if err != nil {
		return err
	}

	valueMatchers, err := buildPatternMatchers(s.ValueContains, s.ValueRegexp, s.IgnoreCase)
	if err != nil {
		return err
	}

	if len(keyMatchers) == 0 && len(valueMatchers) == 0 {
		return fmt.Errorf("provide at least one key or value matcher (--key-contains/--key-regexp/--value-contains/--value-regexp)")
	}

	entries, warns := listing.Walk(client, s.Path, s.Depth)
	for _, w := range warns {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", w.Error())
		row := types.NewRow(
			types.MRP("type", "warning"),
			types.MRP("path", extractPathFromError(w)),
			types.MRP("error", w.Error()),
		)
		if err := gp.AddRow(ctx, row); err != nil {
			return err
		}
	}

	metadataCache := map[string]*vault.SecretMetadata{}
	metadataErrCache := map[string]error{}

	for _, entry := range entries {
		if strings.HasSuffix(entry, "/") {
			continue
		}

		data, err := client.GetSecrets(entry)
		if err != nil {
			msg := fmt.Sprintf("failed to read %s: %v", entry, err)
			fmt.Fprintln(os.Stderr, msg)
			row := types.NewRow(
				types.MRP("type", "error"),
				types.MRP("path", entry),
				types.MRP("error", msg),
			)
			if err := gp.AddRow(ctx, row); err != nil {
				return err
			}
			continue
		}

		var meta *vault.SecretMetadata
		var metaErr error
		if s.IncludeAudit {
			if cached, ok := metadataCache[entry]; ok {
				meta = cached
			} else if cachedErr, ok := metadataErrCache[entry]; ok {
				metaErr = cachedErr
			} else {
				meta, metaErr = client.GetSecretMetadata(entry)
				if metaErr != nil {
					metadataErrCache[entry] = metaErr
				} else {
					metadataCache[entry] = meta
				}
			}
		}

		keys := make([]string, 0, len(data))
		for k := range data {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			v := data[k]
			matchTypes := collectMatchTypes(k, v, keyMatchers, valueMatchers)
			if len(matchTypes) == 0 {
				continue
			}

			valueStr := fmt.Sprintf("%v", v)
			displayValue := valueStr
			if !s.Reveal {
				displayValue = censorString(valueStr, s.CensorPrefix, s.CensorSuffix)
			}

			params := []types.MapRowPair{
				types.MRP("path", entry),
				types.MRP("key", k),
				types.MRP("match", strings.Join(matchTypes, ",")),
				types.MRP("value", displayValue),
				types.MRP("value_length", len(valueStr)),
			}

			if s.IncludeAudit {
				if metaErr != nil {
					params = append(params, types.MRP("audit_error", metaErr.Error()))
				} else if meta != nil {
					params = append(params, types.MRP("current_version", meta.CurrentVersion))
					if versions := summarizeVersions(meta, s.AuditLimit); len(versions) > 0 {
						params = append(params, types.MRP("audit_versions", versions))
					}
				}
			}

			row := types.NewRow(params...)
			if err := gp.AddRow(ctx, row); err != nil {
				return err
			}
		}
	}

	return nil
}

func collectMatchTypes(key string, value interface{}, keyMatchers []matcherFunc, valueMatchers []matcherFunc) []string {
	var matchTypes []string

	if len(keyMatchers) > 0 && matchesAny(keyMatchers, key) {
		matchTypes = append(matchTypes, "key")
	}

	if len(valueMatchers) > 0 {
		valStr := fmt.Sprintf("%v", value)
		if matchesAny(valueMatchers, valStr) {
			matchTypes = append(matchTypes, "value")
		}
	}

	return matchTypes
}

func buildPatternMatchers(contains []string, regexps []string, ignoreCase bool) ([]matcherFunc, error) {
	matchers := make([]matcherFunc, 0, len(contains)+len(regexps))
	for _, c := range contains {
		matchers = append(matchers, buildContainsMatcher(c, ignoreCase))
	}
	regexMatchers, err := buildRegexMatchers(regexps, ignoreCase)
	if err != nil {
		return nil, err
	}
	matchers = append(matchers, regexMatchers...)
	return matchers, nil
}

func buildContainsMatcher(pattern string, ignoreCase bool) matcherFunc {
	if ignoreCase {
		lpat := strings.ToLower(pattern)
		return func(s string) bool {
			return strings.Contains(strings.ToLower(s), lpat)
		}
	}
	return func(s string) bool {
		return strings.Contains(s, pattern)
	}
}

func buildRegexMatchers(patterns []string, ignoreCase bool) ([]matcherFunc, error) {
	matchers := make([]matcherFunc, 0, len(patterns))
	for _, p := range patterns {
		m, err := buildRegexMatcher(p, ignoreCase)
		if err != nil {
			return nil, err
		}
		matchers = append(matchers, m)
	}
	return matchers, nil
}

func buildRegexMatcher(pattern string, ignoreCase bool) (matcherFunc, error) {
	pat := pattern
	if ignoreCase && !strings.HasPrefix(pattern, "(?i)") {
		pat = "(?i)" + pattern
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		return nil, fmt.Errorf("invalid regular expression %q: %w", pattern, err)
	}
	return func(s string) bool {
		return re.MatchString(s)
	}, nil
}

func matchesAny(matchers []matcherFunc, candidate string) bool {
	for _, m := range matchers {
		if m(candidate) {
			return true
		}
	}
	return false
}

func summarizeVersions(meta *vault.SecretMetadata, limit int) []string {
	if meta == nil || len(meta.Versions) == 0 {
		return nil
	}
	versions := make([]int, 0, len(meta.Versions))
	for v := range meta.Versions {
		versions = append(versions, v)
	}
	sort.Ints(versions)
	if limit > 0 && len(versions) > limit {
		versions = versions[len(versions)-limit:]
	}

	out := make([]string, 0, len(versions))
	for _, v := range versions {
		vm := meta.Versions[v]
		parts := []string{fmt.Sprintf("v%d", v)}
		if vm.CreatedTime != nil {
			parts = append(parts, "created="+vm.CreatedTime.UTC().Format(time.RFC3339))
		}
		if vm.DeletionTime != nil && !vm.DeletionTime.IsZero() {
			parts = append(parts, "deleted="+vm.DeletionTime.UTC().Format(time.RFC3339))
		}
		if vm.Destroyed {
			parts = append(parts, "destroyed=true")
		}
		out = append(out, strings.Join(parts, " "))
	}
	return out
}

func extractPathFromError(err error) string {
	msg := err.Error()
	parts := strings.SplitN(msg, ": ", 2)
	if len(parts) == 2 {
		return parts[0]
	}
	return ""
}

type matcherFunc func(string) bool

var _ gcmds.GlazeCommand = &SearchCommand{}
