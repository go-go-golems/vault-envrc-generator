package analyze

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-go-golems/vault-envrc-generator/pkg/seed"
)

// Suggestion links an unmapped env var to a potential seed configuration target.
type Suggestion struct {
	Reason             string `json:"reason" yaml:"reason"`
	CandidateSet       string `json:"candidate_set,omitempty" yaml:"candidate_set,omitempty"`
	CandidateVaultPath string `json:"candidate_vault_path,omitempty" yaml:"candidate_vault_path,omitempty"`
	CandidateVaultKey  string `json:"candidate_vault_key,omitempty" yaml:"candidate_vault_key,omitempty"`
}

type prefixRule struct {
	Prefix string
	Reason string
	Set    string
	Path   string
}

type aliasRule struct {
	EnvVar string
	Reason string
	Set    string
	Path   string
	Key    string
}

var prefixRules = []prefixRule{
	{Prefix: "MENTO_SERVICE_DB_", Reason: "Mentor service database prefix", Set: "postgres-main", Path: "resources/postgres/main"},
	{Prefix: "REDIS_", Reason: "Redis-style prefix", Set: "redis", Path: "resources/redis/main"},
	{Prefix: "ELASTICSEARCH_", Reason: "Elasticsearch-style prefix", Set: "elasticsearch", Path: "resources/elasticsearch/main"},
	{Prefix: "GOOGLE_", Reason: "Google cloud prefix", Set: "google", Path: "resources/google"},
}

var aliasRules = []aliasRule{
	{EnvVar: "OPENAI_API_KEY", Reason: "Known OpenAI API key", Set: "ai-openai", Path: "resources/ai/openai", Key: "api_key"},
	{EnvVar: "ANTHROPIC_API_KEY", Reason: "Known Anthropic API key", Set: "ai-anthropic", Path: "resources/ai/anthropic", Key: "api_key"},
	{EnvVar: "TF_VAR_DO_TOKEN", Reason: "DigitalOcean Terraform token", Set: "digitalocean", Path: "resources/deploy/digitalocean", Key: "access_token"},
}

func suggestForEnvVar(envVar string, spec *seed.Spec, mappings map[string]mappingEntry) []Suggestion {
	suggestions := make([]Suggestion, 0)
	seen := map[string]struct{}{}

	// Alias matches first.
	for _, rule := range aliasRules {
		if strings.EqualFold(envVar, rule.EnvVar) {
			s := buildSuggestion(rule.Reason, rule.Set, rule.Path, rule.Key, spec)
			key := suggestionID(s)
			if _, ok := seen[key]; !ok {
				suggestions = append(suggestions, s)
				seen[key] = struct{}{}
			}
		}
	}

	// Prefix heuristics.
	for _, rule := range prefixRules {
		if strings.HasPrefix(strings.ToUpper(envVar), strings.ToUpper(rule.Prefix)) {
			keyCandidate := deriveKeyFromSuffix(envVar, rule.Prefix)
			s := buildSuggestion(rule.Reason, rule.Set, rule.Path, keyCandidate, spec)
			key := suggestionID(s)
			if _, ok := seen[key]; !ok {
				suggestions = append(suggestions, s)
				seen[key] = struct{}{}
			}
		}
	}

	// Proximity to existing seed sets.
	suggestions = append(suggestions, proximitySuggestions(envVar, spec, seen)...)

	sort.SliceStable(suggestions, func(i, j int) bool {
		if suggestions[i].CandidateVaultPath == suggestions[j].CandidateVaultPath {
			return suggestions[i].CandidateVaultKey < suggestions[j].CandidateVaultKey
		}
		return suggestions[i].CandidateVaultPath < suggestions[j].CandidateVaultPath
	})
	return suggestions
}

func suggestionID(s Suggestion) string {
	return strings.Join([]string{s.CandidateSet, s.CandidateVaultPath, s.CandidateVaultKey}, "|")
}

func buildSuggestion(reason, setName, vaultPath, vaultKey string, spec *seed.Spec) Suggestion {
	resolvedSet := resolveSetName(setName, vaultPath, spec)
	return Suggestion{
		Reason:             reason,
		CandidateSet:       resolvedSet,
		CandidateVaultPath: vaultPath,
		CandidateVaultKey:  vaultKey,
	}
}

func resolveSetName(candidate string, vaultPath string, spec *seed.Spec) string {
	if spec == nil {
		return candidate
	}
	normalizedCandidate := strings.TrimSpace(candidate)
	for _, set := range spec.Sets {
		if strings.EqualFold(set.Path, vaultPath) && set.Name != "" {
			return set.Name
		}
		// When only leaf matches, prefer actual set name.
		if strings.EqualFold(filepath.Base(set.Path), filepath.Base(vaultPath)) && set.Name != "" {
			return set.Name
		}
	}
	return normalizedCandidate
}

func deriveKeyFromSuffix(envVar, prefix string) string {
	suffix := strings.TrimPrefix(envVar, prefix)
	// If prefix case differs, fall back to uppercase variant.
	if suffix == envVar {
		suffix = strings.TrimPrefix(strings.ToUpper(envVar), strings.ToUpper(prefix))
	}
	suffix = strings.TrimPrefix(suffix, "_")
	suffix = strings.ReplaceAll(suffix, "__", "_")
	suffix = strings.ToLower(suffix)
	return strings.ReplaceAll(suffix, "_", "-")
}

func proximitySuggestions(envVar string, spec *seed.Spec, seen map[string]struct{}) []Suggestion {
	suggestions := []Suggestion{}
	token := strings.ToUpper(envVar)
	for _, set := range spec.Sets {
		if set.Name == "" {
			continue
		}
		normalized := strings.ToUpper(strings.ReplaceAll(set.Name, "-", "_"))
		if strings.Contains(token, normalized) {
			keyCandidate := deriveKeyFromSuffix(envVar, normalized+"_")
			s := Suggestion{
				Reason:             "Matches seed set name",
				CandidateSet:       set.Name,
				CandidateVaultPath: set.Path,
				CandidateVaultKey:  keyCandidate,
			}
			key := suggestionID(s)
			if _, ok := seen[key]; !ok {
				suggestions = append(suggestions, s)
				seen[key] = struct{}{}
			}
		}
	}

	return suggestions
}

func summarizeSuggestions(suggestions []Suggestion) string {
	if len(suggestions) == 0 {
		return ""
	}
	parts := make([]string, 0, len(suggestions))
	for _, s := range suggestions {
		key := s.CandidateVaultKey
		if key == "" {
			key = "<key>"
		}
		var builder strings.Builder
		if s.CandidateSet != "" {
			builder.WriteString(s.CandidateSet)
		}
		if s.CandidateVaultPath != "" {
			if builder.Len() > 0 {
				builder.WriteString(" ")
			}
			builder.WriteString("[")
			builder.WriteString(s.CandidateVaultPath)
			builder.WriteString("]")
		}
		builder.WriteString(":" + key)
		if s.Reason != "" {
			builder.WriteString(" (" + s.Reason + ")")
		}
		parts = append(parts, builder.String())
	}
	return strings.Join(parts, "; ")
}
