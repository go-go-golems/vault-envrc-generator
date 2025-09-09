package seed

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-go-golems/vault-envrc-generator/pkg/vault"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

type Options struct{ DryRun bool }

func Run(client *vault.Client, spec *Spec, opts Options) error {
	// Build template context from token for rendering templated paths
	tctx, err := vault.BuildTemplateContext(client)
	if err != nil {
		return fmt.Errorf("failed to build template context: %w", err)
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
			Strs("missing_env", missingEnv).
			Msg("seed: resolved set")

		if len(data) == 0 {
			log.Debug().Str("path", renderedTarget).Msg("seed: skipping (no data)")
			continue
		}

		if opts.DryRun {
			log.Debug().Str("path", renderedTarget).Interface("keys", keysOf(data)).Msg("seed: dry-run put")
			continue
		}

		if err := client.PutSecrets(renderedTarget, data); err != nil {
			return fmt.Errorf("failed to write %s: %w", renderedTarget, err)
		}
		log.Info().Str("path", renderedTarget).Int("keys", len(data)).Msg("seed: wrote secrets")
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
