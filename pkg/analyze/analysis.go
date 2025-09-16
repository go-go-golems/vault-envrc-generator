package analyze

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-go-golems/vault-envrc-generator/pkg/seed"
	"gopkg.in/yaml.v3"
)

// EnvSource represents the environment capture strategy requested by the user.
type EnvSource string

const (
	EnvSourceCurrent EnvSource = "current"
	EnvSourceEnvrc   EnvSource = "envrc"
	EnvSourceDirenv  EnvSource = "direnv"
	EnvSourceFile    EnvSource = "file"
)

// Options bundles configuration for running an analysis session.
type Options struct {
	SeedPath    string
	EnvSource   EnvSource
	EnvrcPath   string
	DotenvPath  string
	EmptyEnv    bool
	ConfirmExec bool
	WorkingDir  string
}

// Result captures the structured report produced by Analyze.
type Result struct {
	EnvSource        string            `json:"env_source" yaml:"env_source"`
	EnvrcPath        string            `json:"envrc_path,omitempty" yaml:"envrc_path,omitempty"`
	DotenvPath       string            `json:"dotenv_path,omitempty" yaml:"dotenv_path,omitempty"`
	SeedConfig       string            `json:"seed_config" yaml:"seed_config"`
	BasePath         string            `json:"base_path,omitempty" yaml:"base_path,omitempty"`
	Summary          Summary           `json:"summary" yaml:"summary"`
	Details          Details           `json:"details" yaml:"details"`
	NonEnvSeedInputs []NonEnvSeedInput `json:"non_env_seed_inputs" yaml:"non_env_seed_inputs"`
}

// Summary aggregates counters for the different result categories.
type Summary struct {
	MappedPresent   int `json:"mapped_present" yaml:"mapped_present"`
	MappedMissing   int `json:"mapped_missing" yaml:"mapped_missing"`
	UnmappedPresent int `json:"unmapped_present" yaml:"unmapped_present"`
}

// Details decomposes the analysis per category.
type Details struct {
	MappedPresent   []MappedPresentEntry   `json:"mapped_present" yaml:"mapped_present"`
	MappedMissing   []MappedMissingEntry   `json:"mapped_missing" yaml:"mapped_missing"`
	UnmappedPresent []UnmappedPresentEntry `json:"unmapped_present" yaml:"unmapped_present"`
}

// MappedPresentEntry represents an env var present both in the environment and in the seed mapping.
type MappedPresentEntry struct {
	EnvVar    string `json:"env_var" yaml:"env_var"`
	Value     string `json:"value" yaml:"value"`
	SetName   string `json:"mapped_set" yaml:"mapped_set"`
	VaultPath string `json:"vault_path" yaml:"vault_path"`
	VaultKey  string `json:"vault_key" yaml:"vault_key"`
}

// MappedMissingEntry represents a seed mapping for which no environment value was found.
type MappedMissingEntry struct {
	EnvVar    string `json:"env_var" yaml:"env_var"`
	SetName   string `json:"mapped_set" yaml:"mapped_set"`
	VaultPath string `json:"vault_path" yaml:"vault_path"`
	VaultKey  string `json:"vault_key" yaml:"vault_key"`
}

// UnmappedPresentEntry tracks environment variables that are not covered by the seed configuration.
type UnmappedPresentEntry struct {
	EnvVar string `json:"env_var" yaml:"env_var"`
	Value  string `json:"value" yaml:"value"`
}

// NonEnvSeedInput documents configuration sections that are not sourced from the environment.
type NonEnvSeedInput struct {
	SetName string   `json:"set" yaml:"set"`
	Type    string   `json:"type" yaml:"type"`
	Keys    []string `json:"keys" yaml:"keys"`
}

// Analyze loads the seed specification, captures the environment, and classifies the variables.
func Analyze(ctx context.Context, opts Options) (*Result, map[string]string, error) {
	if opts.SeedPath == "" {
		return nil, nil, errors.New("seed specification path is required")
	}

	spec, err := loadSeedSpec(opts.SeedPath)
	if err != nil {
		return nil, nil, err
	}

	mappings, nonEnvInputs := extractMappings(spec)
	envValues, err := captureEnvironment(ctx, opts)
	if err != nil {
		return nil, nil, err
	}

	// Classify results
	mappedPresent := make([]MappedPresentEntry, 0)
	mappedMissing := make([]MappedMissingEntry, 0)
	unmappedPresent := make([]UnmappedPresentEntry, 0)

	// Determine which mapped vars are missing.
	for envVar, mapping := range mappings {
		if val, ok := envValues[envVar]; ok {
			mappedPresent = append(mappedPresent, MappedPresentEntry{
				EnvVar:    envVar,
				Value:     val,
				SetName:   mapping.SetName,
				VaultPath: mapping.VaultPath,
				VaultKey:  mapping.VaultKey,
			})
		} else {
			mappedMissing = append(mappedMissing, MappedMissingEntry{
				EnvVar:    envVar,
				SetName:   mapping.SetName,
				VaultPath: mapping.VaultPath,
				VaultKey:  mapping.VaultKey,
			})
		}
	}

	// Any environment var that isn't mapped is considered unmapped (unless ignored).
	seenMapped := make(map[string]struct{}, len(mappings))
	for envVar := range mappings {
		seenMapped[envVar] = struct{}{}
	}

	for envVar, value := range envValues {
		if _, ok := seenMapped[envVar]; ok {
			continue
		}
		if shouldIgnoreUnmapped(envVar) {
			continue
		}
		unmappedPresent = append(unmappedPresent, UnmappedPresentEntry{
			EnvVar: envVar,
			Value:  value,
		})
	}

	sort.Slice(mappedPresent, func(i, j int) bool {
		return mappedPresent[i].EnvVar < mappedPresent[j].EnvVar
	})
	sort.Slice(mappedMissing, func(i, j int) bool {
		return mappedMissing[i].EnvVar < mappedMissing[j].EnvVar
	})
	sort.Slice(unmappedPresent, func(i, j int) bool {
		return unmappedPresent[i].EnvVar < unmappedPresent[j].EnvVar
	})
	sort.Slice(nonEnvInputs, func(i, j int) bool {
		if nonEnvInputs[i].SetName == nonEnvInputs[j].SetName {
			if nonEnvInputs[i].Type == nonEnvInputs[j].Type {
				return strings.Join(nonEnvInputs[i].Keys, ",") < strings.Join(nonEnvInputs[j].Keys, ",")
			}
			return nonEnvInputs[i].Type < nonEnvInputs[j].Type
		}
		return nonEnvInputs[i].SetName < nonEnvInputs[j].SetName
	})

	result := &Result{
		EnvSource:  string(opts.EnvSource),
		EnvrcPath:  opts.EnvrcPath,
		DotenvPath: opts.DotenvPath,
		SeedConfig: opts.SeedPath,
		BasePath:   strings.TrimSpace(spec.BasePath),
		Summary: Summary{
			MappedPresent:   len(mappedPresent),
			MappedMissing:   len(mappedMissing),
			UnmappedPresent: len(unmappedPresent),
		},
		Details: Details{
			MappedPresent:   mappedPresent,
			MappedMissing:   mappedMissing,
			UnmappedPresent: unmappedPresent,
		},
		NonEnvSeedInputs: nonEnvInputs,
	}

	return result, envValues, nil
}

// mappingEntry is used internally to link environment variables to seed configuration entries.
type mappingEntry struct {
	SetName   string
	VaultPath string
	VaultKey  string
}

func loadSeedSpec(path string) (*seed.Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read seed config %s: %w", path, err)
	}
	var spec seed.Spec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse seed YAML %s: %w", path, err)
	}
	return &spec, nil
}

func extractMappings(spec *seed.Spec) (map[string]mappingEntry, []NonEnvSeedInput) {
	mappings := make(map[string]mappingEntry)
	inputs := make([]NonEnvSeedInput, 0)

	for _, set := range spec.Sets {
		setName := set.Name
		if setName == "" {
			// Derive a stable name from path when missing
			setName = filepath.Base(set.Path)
		}

		for vaultKey, envVar := range set.Env {
			mappings[envVar] = mappingEntry{
				SetName:   setName,
				VaultPath: set.Path,
				VaultKey:  vaultKey,
			}
		}

		collectNonEnv := func(kind string, keys []string) {
			if len(keys) == 0 {
				return
			}
			sort.Strings(keys)
			inputs = append(inputs, NonEnvSeedInput{
				SetName: setName,
				Type:    kind,
				Keys:    keys,
			})
		}

		if len(set.Data) > 0 {
			keys := make([]string, 0, len(set.Data))
			for k := range set.Data {
				keys = append(keys, k)
			}
			collectNonEnv("data", keys)
		}
		if len(set.Files) > 0 {
			keys := make([]string, 0, len(set.Files))
			for k := range set.Files {
				keys = append(keys, k)
			}
			collectNonEnv("files", keys)
		}
		if len(set.Commands) > 0 {
			keys := make([]string, 0, len(set.Commands))
			for k := range set.Commands {
				keys = append(keys, k)
			}
			collectNonEnv("commands", keys)
		}
		if len(set.SetupCommands) > 0 {
			labels := make([]string, 0, len(set.SetupCommands))
			for idx, sc := range set.SetupCommands {
				label := strings.TrimSpace(sc.Name)
				if label == "" {
					label = strings.TrimSpace(sc.OutputKey)
				}
				if label == "" {
					label = fmt.Sprintf("setup[%d]", idx+1)
				}
				labels = append(labels, label)
			}
			collectNonEnv("setup_commands", labels)
		}
		if len(set.JsonFiles) > 0 {
			keys := make([]string, 0, len(set.JsonFiles))
			for k := range set.JsonFiles {
				keys = append(keys, k)
			}
			collectNonEnv("json_files", keys)
		}
		if len(set.YamlFiles) > 0 {
			keys := make([]string, 0, len(set.YamlFiles))
			for k := range set.YamlFiles {
				keys = append(keys, k)
			}
			collectNonEnv("yaml_files", keys)
		}
	}

	return mappings, inputs
}

// shouldIgnoreUnmapped filters out ubiquitous shell variables that would otherwise overwhelm the report.
func shouldIgnoreUnmapped(envVar string) bool {
	upper := strings.ToUpper(envVar)
	if _, ok := ignoredExact[upper]; ok {
		return true
	}
	for _, prefix := range ignoredPrefixes {
		if strings.HasPrefix(upper, prefix) {
			return true
		}
	}
	return false
}

var ignoredExact = map[string]struct{}{
	"PWD":              {},
	"OLDPWD":           {},
	"SHLVL":            {},
	"SHELL":            {},
	"PATH":             {},
	"HOME":             {},
	"USER":             {},
	"TERM":             {},
	"EDITOR":           {},
	"LANG":             {},
	"LOGNAME":          {},
	"TMPDIR":           {},
	"TMP":              {},
	"TEMP":             {},
	"PS1":              {},
	"PS2":              {},
	"PROMPT_COMMAND":   {},
	"GIT_AUTHOR_NAME":  {},
	"GIT_AUTHOR_EMAIL": {},
}

var ignoredPrefixes = []string{
	"XDG_",
	"LC_",
	"LESS",
	"LS_COLORS",
	"DIRENV",
	"_",
	"BASH_",
	"ZSH",
	"SSH_",
	"GPG_",
	"HIST",
	"SYSTEMD_",
}

// StrictError is returned when AnalyzeStrict detects violations and strict mode is enabled.
type StrictError struct {
	Missing  int
	Unmapped int
}

func (e *StrictError) Error() string {
	return fmt.Sprintf("strict mode failed: %d mapped variables missing, %d unmapped present", e.Missing, e.Unmapped)
}

// AnalyzeStrict wraps Analyze and enforces strict mode expectations.
func AnalyzeStrict(ctx context.Context, opts Options) (*Result, map[string]string, error) {
	result, env, err := Analyze(ctx, opts)
	if err != nil {
		return nil, nil, err
	}
	if result.Summary.MappedMissing > 0 || result.Summary.UnmappedPresent > 0 {
		return result, env, &StrictError{Missing: result.Summary.MappedMissing, Unmapped: result.Summary.UnmappedPresent}
	}
	return result, env, nil
}

// CloneWithMask produces a deep copy of the result with values hidden or masked according to the flags.
func CloneWithMask(result *Result, includeValues bool, censor string) *Result {
	if result == nil {
		return nil
	}

	clone := *result
	clone.Details.MappedPresent = make([]MappedPresentEntry, len(result.Details.MappedPresent))
	for i, entry := range result.Details.MappedPresent {
		clone.Details.MappedPresent[i] = entry
		if includeValues {
			if censor != "" {
				clone.Details.MappedPresent[i].Value = censor
			}
		} else {
			clone.Details.MappedPresent[i].Value = ""
		}
	}

	clone.Details.MappedMissing = append([]MappedMissingEntry(nil), result.Details.MappedMissing...)

	clone.Details.UnmappedPresent = make([]UnmappedPresentEntry, len(result.Details.UnmappedPresent))
	for i, entry := range result.Details.UnmappedPresent {
		clone.Details.UnmappedPresent[i] = entry
		if includeValues {
			if censor != "" {
				clone.Details.UnmappedPresent[i].Value = censor
			}
		} else {
			clone.Details.UnmappedPresent[i].Value = ""
		}
	}

	clone.NonEnvSeedInputs = append([]NonEnvSeedInput(nil), result.NonEnvSeedInputs...)
	return &clone
}
