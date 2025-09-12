package seed

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-go-golems/vault-envrc-generator/pkg/vault"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

type Options struct {
	DryRun            bool
	ForceOverwrite    bool
	AllowCommands     bool
	ExtraTemplateData map[string]interface{}
	OnlyNew           bool
}

type userDecision int

const (
	decNo userDecision = iota
	decYes
	decAllYes
	decAllNo
)

func Run(client *vault.Client, spec *Spec, opts Options) error {
	// Build template context from token for rendering templated paths
	tctx, err := vault.BuildTemplateContext(client)
	if err != nil {
		return fmt.Errorf("failed to build template context: %w", err)
	}
	// Initialize Extra and merge in extra template data
	if tctx.Extra == nil {
		tctx.Extra = map[string]interface{}{}
	}
	for k, v := range opts.ExtraTemplateData {
		tctx.Extra[k] = v
	}
	// Default commonly used flags
	if _, ok := tctx.Extra["skip_generate"]; !ok {
		tctx.Extra["skip_generate"] = false
	}

	// Resolve base path: YAML has priority; render templates
	base := strings.TrimSuffix(spec.BasePath, "/")
	if base == "" {
		base = strings.TrimSuffix(viper.GetString("seed.base_path"), "/")
	}
	if base != "" {
		if bp, err := vault.RenderTemplateString(base, tctx); err == nil {
			base = strings.TrimSuffix(bp, "/")
		} else {
			return fmt.Errorf("failed to render base_path template: %w", err)
		}
	}

	log.Info().Str("base_path", base).Int("sets_total", len(spec.Sets)).Msg("seed: start")

	// Interactive state flags
	overwriteAllAllow := false
	overwriteAllSkip := false
	commandAllRun := false
	commandAllSkip := false

	for i, set := range spec.Sets {
		// Determine target path: join with base if relative; error if relative without base
		target := set.Path
		if !vault.IsAbsoluteVaultPath(target) && base == "" {
			return fmt.Errorf("set %d: relative path '%s' without base_path", i+1, target)
		}
		target = vault.JoinBaseAndPath(base, target)
		renderedTarget, err := vault.RenderTemplateString(target, tctx)
		if err != nil {
			return fmt.Errorf("set %d: failed to render path '%s': %w", i+1, target, err)
		}

		data := map[string]interface{}{}
		missingEnv := []string{}

		// Process static data
		for k, v := range set.Data {
			data[k] = v
		}

		// Process environment variables
		for k, envName := range set.Env {
			if val, ok := os.LookupEnv(envName); ok {
				data[k] = val
			} else {
				missingEnv = append(missingEnv, envName)
			}
		}

		// Process regular files
		for k, filePath := range set.Files {
			fp := expandPath(filePath)
			if content, err := os.ReadFile(fp); err == nil {
				data[k] = string(content)
			} else {
				return fmt.Errorf("set %d: failed reading %s: %w", i+1, fp, err)
			}
		}

		// Process JSON files with transforms
		for vaultKey, jsonTransform := range set.JsonFiles {
			jsonData, err := processJsonFile(jsonTransform.File, jsonTransform.Transforms)
			if err != nil {
				return fmt.Errorf("set %d: failed processing JSON file: %w", i+1, err)
			}
			// If no transforms specified, use the entire JSON data as the vault key
			if len(jsonTransform.Transforms) == 0 {
				// Read entire file content as string
				fp := expandPath(jsonTransform.File)
				if content, err := os.ReadFile(fp); err == nil {
					data[vaultKey] = string(content)
				} else {
					return fmt.Errorf("set %d: failed reading JSON file %s: %w", i+1, fp, err)
				}
			} else {
				// Apply transforms and add each transformed value
				for transformedKey, value := range jsonData {
					data[transformedKey] = convertToString(value)
				}
			}
		}

		// Process YAML files with transforms
		for vaultKey, yamlTransform := range set.YamlFiles {
			yamlData, err := processYamlFile(yamlTransform.File, yamlTransform.Transforms)
			if err != nil {
				return fmt.Errorf("set %d: failed processing YAML file: %w", i+1, err)
			}
			// If no transforms specified, use the entire YAML data as the vault key
			if len(yamlTransform.Transforms) == 0 {
				// Read entire file content as string
				fp := expandPath(yamlTransform.File)
				if content, err := os.ReadFile(fp); err == nil {
					data[vaultKey] = string(content)
				} else {
					return fmt.Errorf("set %d: failed reading YAML file %s: %w", i+1, fp, err)
				}
			} else {
				// Apply transforms and add each transformed value
				for transformedKey, value := range yamlData {
					data[transformedKey] = convertToString(value)
				}
			}
		}

		// Prepare context with current data for command templates
		if tctx.Data == nil {
			tctx.Data = map[string]interface{}{}
		}
		for dk, dv := range data {
			tctx.Data[dk] = dv
		}

		// Run setup_commands: populate tctx.Data but do not persist into data map
		if len(set.SetupCommands) > 0 {
			executed := map[string]bool{}
			keys := make([]string, 0, len(set.SetupCommands))
			for k := range set.SetupCommands {
				keys = append(keys, k)
			}
			// Attempt multiple passes to satisfy dependencies
			for pass := 0; pass < len(keys); pass++ {
				progress := false
				for _, k := range keys {
					if executed[k] {
						continue
					}
					command := set.SetupCommands[k]
					cmdStr, err := vault.RenderTemplateString(command, tctx)
					if err != nil {
						// Defer if missing .Data dependency
						if isMissingDataKeyError(err) {
							continue
						}
						return fmt.Errorf("set %d: failed to render setup command '%s': %w", i+1, k, err)
					}
					if strings.TrimSpace(cmdStr) == "" {
						executed[k] = true
						progress = true
						continue
					}
					if opts.DryRun {
						log.Debug().Str("key", k).Str("path", renderedTarget).Msg("seed: dry-run setup command")
						executed[k] = true
						progress = true
						continue
					}
					if !opts.AllowCommands {
						if commandAllSkip {
							executed[k] = true
							progress = true
							continue
						}
						if !commandAllRun {
							escapedRenderedTarget := strings.ReplaceAll(renderedTarget, `\`, `\\`)
							escapedRenderedTarget = strings.ReplaceAll(escapedRenderedTarget, "'", "\\'")
							prompt := fmt.Sprintf("Run setup command '%s' at '%s'? [y/N/a/s]: %s ", k, escapedRenderedTarget, cmdStr)
							dec := askForDecision(prompt)
							switch dec {
							case decYes:
								// proceed
							case decAllYes:
								commandAllRun = true
							case decAllNo:
								commandAllSkip = true
								fallthrough
							case decNo:
								executed[k] = true
								progress = true
								continue
							}
						}
					}
					out, err := runShellCommand(cmdStr)
					if err != nil {
						return fmt.Errorf("set %d: setup command '%s' failed: %w", i+1, k, err)
					}
					val := strings.TrimSpace(out)
					if val != "" {
						tctx.Data[k] = val
					}
					executed[k] = true
					progress = true
				}
				if !progress {
					break
				}
			}
			// Verify all executed
			for _, k := range keys {
				if !executed[k] {
					return fmt.Errorf("set %d: unsatisfied setup command dependencies; could not render '%s'", i+1, k)
				}
			}
		}

		// Process commands (execute and capture stdout; command string can be templated)
		if len(set.Commands) > 0 {
			// deterministic execution order to allow dependencies
			keys := make([]string, 0, len(set.Commands))
			for k := range set.Commands {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				command := set.Commands[k]
				cmdStr, err := vault.RenderTemplateString(command, tctx)
				if err != nil {
					return fmt.Errorf("set %d: failed to render command for key '%s': %w", i+1, k, err)
				}
				if strings.TrimSpace(cmdStr) == "" {
					log.Debug().Str("key", k).Str("path", renderedTarget).Msg("seed: skipping empty command after template rendering")
					continue
				}
				if opts.DryRun {
					data[k] = fmt.Sprintf("<command: %s>", cmdStr)
					continue
				}
				if !opts.AllowCommands {
					if commandAllSkip {
						continue
					}
					if !commandAllRun {
						prompt := fmt.Sprintf("Run command for key '%s' at path '%s'? [y/N/a/s]: %s ", k, renderedTarget, cmdStr)
						dec := askForDecision(prompt)
						switch dec {
						case decYes:
							// proceed
						case decAllYes:
							commandAllRun = true
						case decAllNo:
							commandAllSkip = true
							continue
						case decNo:
							continue
						}
					}
				}
				out, err := runShellCommand(cmdStr)
				if err != nil {
					return fmt.Errorf("set %d: command for key '%s' failed: %w", i+1, k, err)
				}
				val := strings.TrimSpace(out)
				if val == "" {
					// do not overwrite existing data with empty output
					continue
				}
				data[k] = val
				// make it available to subsequent command templates
				tctx.Data[k] = val
			}
		}

		log.Debug().
			Int("index", i+1).
			Str("set_name", set.Name).
			Str("set_path", set.Path).
			Str("target", renderedTarget).
			Int("static_keys", len(set.Data)).
			Int("env_keys_present", len(set.Env)-len(missingEnv)).
			Int("env_keys_missing", len(missingEnv)).
			Int("file_keys", len(set.Files)).
			Int("json_file_keys", len(set.JsonFiles)).
			Int("yaml_file_keys", len(set.YamlFiles)).
			Msg("seed: resolved set")

		if len(data) == 0 {
			log.Debug().Str("path", renderedTarget).Msg("seed: skipping (no data)")
			continue
		}

		if opts.DryRun {
			log.Debug().Str("path", renderedTarget).Interface("keys", keysOf(data)).Msg("seed: dry-run put")
			continue
		}

		// If only-new is enabled, drop keys already present at target path
		if opts.OnlyNew {
			existing, err := client.GetSecrets(renderedTarget)
			if err != nil {
				// ignore not-found; treat as empty
				if !isNotFoundError(err) {
					return fmt.Errorf("failed to check existing secrets at %s: %w", renderedTarget, err)
				}
			} else if len(existing) > 0 {
				for key := range data {
					if _, ok := existing[key]; ok {
						delete(data, key)
					}
				}
			}
		}

		// Handle overwrite confirmations when existing values are present
		if !opts.ForceOverwrite && !opts.OnlyNew {
			existing, err := client.GetSecrets(renderedTarget)
			if err != nil {
				// if not found, skip prompting; otherwise propagate error
				if !isNotFoundError(err) {
					return fmt.Errorf("failed to check existing secrets at %s: %w", renderedTarget, err)
				}
			} else if len(existing) > 0 {
				for key := range data {
					if _, ok := existing[key]; ok {
						if overwriteAllSkip {
							delete(data, key)
							continue
						}
						if !overwriteAllAllow {
							prompt := fmt.Sprintf("Key '%s' exists at '%s'. Overwrite? [y/N/a/s]: ", key, renderedTarget)
							dec := askForDecision(prompt)
							switch dec {
							case decYes:
								// keep value in data
							case decAllYes:
								overwriteAllAllow = true
							case decAllNo:
								overwriteAllSkip = true
								fallthrough
							case decNo:
								delete(data, key)
								continue
							}
						}
					}
				}
			}
		}

		if len(data) == 0 {
			log.Debug().Str("path", renderedTarget).Msg("seed: skipping write (no data after confirmations)")
			continue
		}

		if err := client.PutSecrets(renderedTarget, data); err != nil {
			return fmt.Errorf("failed to write %s: %w", renderedTarget, err)
		}
		log.Info().Str("path", renderedTarget).Int("keys", len(data)).Msg("seed: wrote secrets")

		// Run cleanup_commands after writing (no persistence)
		if len(set.CleanupCommands) > 0 {
			for _, cmdT := range set.CleanupCommands {
				cmdStr, err := vault.RenderTemplateString(cmdT, tctx)
				if err != nil {
					return fmt.Errorf("set %d: failed to render cleanup command: %w", i+1, err)
				}
				if strings.TrimSpace(cmdStr) == "" {
					continue
				}
				if opts.DryRun {
					log.Debug().Str("path", renderedTarget).Msg("seed: dry-run cleanup command")
					continue
				}
				if !opts.AllowCommands {
					if commandAllSkip {
						continue
					}
					if !commandAllRun {
						prompt := fmt.Sprintf("Run cleanup at '%s'? [y/N/a/s]: %s ", renderedTarget, cmdStr)
						dec := askForDecision(prompt)
						switch dec {
						case decYes:
							// proceed
						case decAllYes:
							commandAllRun = true
						case decAllNo:
							commandAllSkip = true
							continue
						case decNo:
							continue
						}
					}
				}
				if _, err := runShellCommand(cmdStr); err != nil {
					return fmt.Errorf("set %d: cleanup command failed: %w", i+1, err)
				}
			}
		}
	}
	log.Info().Msg("seed: completed")
	return nil
}

func keysOf(m map[string]interface{}) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

// askForDecision prompts the user for y/N/a/s and returns a decision
func askForDecision(prompt string) userDecision {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	answer := strings.TrimSpace(strings.ToLower(line))
	switch answer {
	case "y", "yes":
		return decYes
	case "a", "all":
		return decAllYes
	case "s", "skip", "sa", "skipall", "skip-all":
		return decAllNo
	default:
		return decNo
	}
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "no secret found at path") || strings.Contains(s, "no secret found at KV v2 path")
}

func isMissingDataKeyError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "map has no entry for key") || strings.Contains(s, "can't evaluate field")
}

func runShellCommand(command string) (string, error) {
	cmd := exec.Command("/bin/sh", "-c", command)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// processJsonFile reads a JSON file and applies key transforms
func processJsonFile(filePath string, transforms map[string]string) (map[string]interface{}, error) {
	fp := expandPath(filePath)
	content, err := os.ReadFile(fp)
	if err != nil {
		return nil, fmt.Errorf("failed to read JSON file %s: %w", fp, err)
	}

	var jsonData interface{}
	if err := json.Unmarshal(content, &jsonData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON file %s: %w", fp, err)
	}

	result := make(map[string]interface{})
	for vaultKey, jsonPath := range transforms {
		value, err := extractValueByPath(jsonData, jsonPath)
		if err != nil {
			return nil, fmt.Errorf("failed to extract path '%s' from JSON file %s: %w", jsonPath, fp, err)
		}
		result[vaultKey] = value
	}

	return result, nil
}

// processYamlFile reads a YAML file and applies key transforms
func processYamlFile(filePath string, transforms map[string]string) (map[string]interface{}, error) {
	fp := expandPath(filePath)
	content, err := os.ReadFile(fp)
	if err != nil {
		return nil, fmt.Errorf("failed to read YAML file %s: %w", fp, err)
	}

	var yamlData interface{}
	if err := yaml.Unmarshal(content, &yamlData); err != nil {
		return nil, fmt.Errorf("failed to parse YAML file %s: %w", fp, err)
	}

	result := make(map[string]interface{})
	for vaultKey, yamlPath := range transforms {
		value, err := extractValueByPath(yamlData, yamlPath)
		if err != nil {
			return nil, fmt.Errorf("failed to extract path '%s' from YAML file %s: %w", yamlPath, fp, err)
		}
		result[vaultKey] = value
	}

	return result, nil
}

// expandPath expands ~ to home directory
func expandPath(filePath string) string {
	if strings.HasPrefix(filePath, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(filePath, "~"))
		}
	}
	return filePath
}

// extractValueByPath extracts a value from nested data using dot notation path
// Examples: "database.host", "auth.oauth.client_id", "servers.0.name"
func extractValueByPath(data interface{}, path string) (interface{}, error) {
	if path == "" {
		return data, nil
	}

	parts := strings.Split(path, ".")
	current := data

	for i, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			var ok bool
			current, ok = v[part]
			if !ok {
				return nil, fmt.Errorf("key '%s' not found at path segment %d", part, i+1)
			}
		case map[interface{}]interface{}:
			// YAML often uses interface{} keys
			var ok bool
			current, ok = v[part]
			if !ok {
				return nil, fmt.Errorf("key '%s' not found at path segment %d", part, i+1)
			}
		case []interface{}:
			// Handle array indexing
			idx, err := parseArrayIndex(part)
			if err != nil {
				return nil, fmt.Errorf("invalid array index '%s' at path segment %d: %w", part, i+1, err)
			}
			if idx < 0 || idx >= len(v) {
				return nil, fmt.Errorf("array index %d out of bounds (length %d) at path segment %d", idx, len(v), i+1)
			}
			current = v[idx]
		default:
			return nil, fmt.Errorf("cannot navigate into %T at path segment %d", current, i+1)
		}
	}

	return current, nil
}

// parseArrayIndex parses array index from string
func parseArrayIndex(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("empty array index")
	}

	var idx int
	_, err := fmt.Sscanf(s, "%d", &idx)
	return idx, err
}

// convertToString converts various types to string for Vault storage
func convertToString(value interface{}) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case string:
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", v)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v)
	case float32, float64:
		return fmt.Sprintf("%g", v)
	default:
		// For complex types (arrays, objects), marshal to JSON
		if data, err := json.Marshal(value); err == nil {
			return string(data)
		}
		return fmt.Sprintf("%v", value)
	}
}
