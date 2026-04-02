package diffenv

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/go-go-golems/vault-envrc-generator/pkg/batch"
	"github.com/go-go-golems/vault-envrc-generator/pkg/seed"
	"github.com/go-go-golems/vault-envrc-generator/pkg/vault"
	"gopkg.in/yaml.v3"
)

// Options controls how the diff is computed
type Options struct {
	SeedPath     string
	BatchPath    string
	BasePath     string
	IncludeExtra bool
}

// Result captures a simple env vs vault diff
type Result struct {
	Matches      []Entry
	Changed      []ChangedEntry
	MissingInEnv []Entry
	ExtraInEnv   []Entry
}

type Entry struct {
	Name  string
	Value string
	Path  string
}

type ChangedEntry struct {
	Name  string
	Vault string
	Env   string
	Path  string
}

// Compute builds the expected env mapping from the provided seed or batch config, and compares it to the current environment.
func Compute(client *vault.Client, opts Options) (*Result, error) {
	expected := map[string]string{}
	expPaths := map[string]string{}

	if strings.TrimSpace(opts.SeedPath) != "" {
		exp, paths, err := expectedFromSeed(client, opts.SeedPath, opts.BasePath)
		if err != nil {
			return nil, err
		}
		for k, v := range exp {
			expected[k] = v
		}
		for k, p := range paths {
			expPaths[k] = p
		}
	}
	if strings.TrimSpace(opts.BatchPath) != "" {
		exp, paths, err := expectedFromBatch(client, opts.BatchPath, opts.BasePath)
		if err != nil {
			return nil, err
		}
		for k, v := range exp {
			expected[k] = v
		}
		for k, p := range paths {
			expPaths[k] = p
		}
	}

	// Capture current environment
	actual := map[string]string{}
	for _, kv := range os.Environ() {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 {
			actual[parts[0]] = parts[1]
		}
	}

	res := &Result{}
	for name, vVal := range expected {
		if eVal, ok := actual[name]; ok {
			if eVal == vVal {
				res.Matches = append(res.Matches, Entry{Name: name, Value: vVal, Path: expPaths[name]})
			} else {
				res.Changed = append(res.Changed, ChangedEntry{Name: name, Vault: vVal, Env: eVal, Path: expPaths[name]})
			}
		} else {
			res.MissingInEnv = append(res.MissingInEnv, Entry{Name: name, Value: vVal, Path: expPaths[name]})
		}
	}
	if opts.IncludeExtra {
		for name, eVal := range actual {
			if _, ok := expected[name]; !ok {
				res.ExtraInEnv = append(res.ExtraInEnv, Entry{Name: name, Value: eVal})
			}
		}
	}

	sort.Slice(res.Matches, func(i, j int) bool { return res.Matches[i].Name < res.Matches[j].Name })
	sort.Slice(res.MissingInEnv, func(i, j int) bool { return res.MissingInEnv[i].Name < res.MissingInEnv[j].Name })
	sort.Slice(res.ExtraInEnv, func(i, j int) bool { return res.ExtraInEnv[i].Name < res.ExtraInEnv[j].Name })
	sort.Slice(res.Changed, func(i, j int) bool { return res.Changed[i].Name < res.Changed[j].Name })
	return res, nil
}

func expectedFromSeed(client *vault.Client, path string, baseOverride string) (map[string]string, map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read seed config: %w", err)
	}
	var spec seed.Spec
	if err := yamlUnmarshal(data, &spec); err != nil {
		return nil, nil, fmt.Errorf("failed to parse seed YAML: %w", err)
	}

	tctx, err := vault.BuildTemplateContext(client)
	if err != nil {
		return nil, nil, err
	}
	base := strings.TrimSuffix(spec.BasePath, "/")
	if strings.TrimSpace(baseOverride) != "" {
		base = strings.TrimSuffix(baseOverride, "/")
	}
	if base != "" {
		if rbp, err := vault.RenderTemplateString(base, tctx); err == nil {
			base = strings.TrimSuffix(rbp, "/")
		}
	}

	expected := map[string]string{}
	paths := map[string]string{}
	for _, set := range spec.Sets {
		joined := vault.JoinBaseAndPath(base, set.Path)
		rendered, err := vault.RenderTemplateString(joined, tctx)
		if err != nil {
			return nil, nil, err
		}
		secrets, err := client.GetSecrets(rendered)
		if err != nil {
			continue
		}
		for vaultKey, envVar := range set.Env {
			if val, ok := secrets[vaultKey]; ok {
				expected[envVar] = convertToString(val)
				paths[envVar] = rendered
			}
		}
	}
	return expected, paths, nil
}

func expectedFromBatch(client *vault.Client, path string, baseOverride string) (map[string]string, map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read batch config: %w", err)
	}
	var cfg batch.Config
	if err := yamlUnmarshal(data, &cfg); err != nil {
		return nil, nil, fmt.Errorf("failed to parse batch YAML: %w", err)
	}

	tctx, err := vault.BuildTemplateContext(client)
	if err != nil {
		return nil, nil, err
	}
	base := strings.TrimSuffix(cfg.BasePath, "/")
	if strings.TrimSpace(baseOverride) != "" {
		base = strings.TrimSuffix(baseOverride, "/")
	}
	if base != "" {
		if rbp, err := vault.RenderTemplateString(base, tctx); err == nil {
			base = strings.TrimSuffix(rbp, "/")
		}
	}

	expected := map[string]string{}
	paths := map[string]string{}
	for _, job := range cfg.Jobs {
		effBase := base
		if strings.TrimSpace(job.BasePath) != "" {
			effBase = strings.TrimSuffix(job.BasePath, "/")
			if rbp, err := vault.RenderTemplateString(effBase, tctx); err == nil {
				effBase = strings.TrimSuffix(rbp, "/")
			}
		}
		for _, sec := range job.Sections {
			joined := vault.JoinBaseAndPath(effBase, sec.Path)
			rendered, err := vault.RenderTemplateString(joined, tctx)
			if err != nil {
				return nil, nil, err
			}
			secrets, err := client.GetSecrets(rendered)
			if err != nil {
				continue
			}

			// Select and transform per env_map or section/job settings
			selected := map[string]interface{}{}
			if len(sec.EnvMap) > 0 {
				for envVar, srcKey := range sec.EnvMap {
					if v, ok := secrets[srcKey]; ok {
						selected[envVar] = v
					}
				}
				// env_map disables transform/prefix/include/exclude
				for envVar, val := range selected {
					expected[envVar] = convertToString(val)
					paths[envVar] = rendered
				}
				continue
			}

			// Start from all secrets
			for k, v := range secrets {
				selected[k] = v
			}

			// Determine effective options
			prefix := job.Prefix
			if sec.Prefix != "" {
				prefix = sec.Prefix
			}
			include := job.IncludeKeys
			if len(sec.IncludeKeys) > 0 {
				include = sec.IncludeKeys
			}
			exclude := job.ExcludeKeys
			if len(sec.ExcludeKeys) > 0 {
				exclude = sec.ExcludeKeys
			}
			transform := false
			if sec.Transform != nil {
				transform = *sec.Transform
			} else if job.Transform != nil {
				transform = *job.Transform
			}

			// Filter include/exclude
			filtered := map[string]interface{}{}
			for k, v := range selected {
				if len(include) > 0 && !matchesAny(k, include) {
					continue
				}
				if len(exclude) > 0 && matchesAny(k, exclude) {
					continue
				}
				filtered[k] = v
			}

			// Transform keys
			named := map[string]string{}
			for k, v := range filtered {
				name := k
				if transform {
					name = strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
				}
				if prefix != "" {
					name = prefix + name
				}
				named[name] = convertToString(v)
			}
			for envVar, val := range named {
				expected[envVar] = val
				paths[envVar] = rendered
			}
		}
	}
	return expected, paths, nil
}

func matchesAny(key string, patterns []string) bool {
	for _, p := range patterns {
		re := strings.ReplaceAll(p, "*", ".*")
		matched, _ := regexp.MatchString("^"+re+"$", key)
		if matched {
			return true
		}
		if key == p {
			return true
		}
	}
	return false
}

// convertToString mirrors seed's conversion for consistency
func convertToString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case bool:
		if t {
			return "true"
		}
		return "false"
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", t)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", t)
	case float32, float64:
		return fmt.Sprintf("%g", t)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// yamlUnmarshal is a small shim to avoid importing yaml twice in callers
var yamlUnmarshal = func(b []byte, out interface{}) error {
	return yaml.Unmarshal(b, out)
}
