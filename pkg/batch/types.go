package batch

// Config represents the configuration for batch processing
type Config struct {
	BasePath string `yaml:"base_path"`
	Jobs     []Job  `yaml:"jobs"`
}

// Section represents one logical section emitted by a job
type Section struct {
	Name        string            `yaml:"name,omitempty"`
	Description string            `yaml:"description,omitempty"`
	Required    bool              `yaml:"required,omitempty"`
	Path        string            `yaml:"path,omitempty"`
	Prefix      string            `yaml:"prefix,omitempty"`
	ExcludeKeys []string          `yaml:"exclude_keys,omitempty"`
	IncludeKeys []string          `yaml:"include_keys,omitempty"`
	Transform   *bool             `yaml:"transform_keys,omitempty"`
	Template    string            `yaml:"template,omitempty"`
	Variables   map[string]string `yaml:"variables,omitempty"`
	Format      string            `yaml:"format,omitempty"`
	Output      string            `yaml:"output,omitempty"`
	EnvMap      map[string]string `yaml:"env_map,omitempty"`
	Fixed       map[string]string `yaml:"fixed,omitempty"`
	Commands    map[string]string `yaml:"commands,omitempty"`
}

// Job represents a single job in batch processing
type Job struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Required    bool              `yaml:"required"`
	Path        string            `yaml:"path,omitempty"`
	Output      string            `yaml:"output"`
	Prefix      string            `yaml:"prefix,omitempty"`
	ExcludeKeys []string          `yaml:"exclude_keys,omitempty"`
	IncludeKeys []string          `yaml:"include_keys,omitempty"`
	Transform   *bool             `yaml:"transform_keys,omitempty"`
	Format      string            `yaml:"format,omitempty"`
	Template    string            `yaml:"template,omitempty"`
	Variables   map[string]string `yaml:"variables,omitempty"`
	Sections    []Section         `yaml:"sections,omitempty"`
	BasePath    string            `yaml:"base_path,omitempty"`
	Fixed       map[string]string `yaml:"fixed,omitempty"`
	EnvrcPrefix string            `yaml:"envrc_prefix,omitempty"`
	EnvrcSuffix string            `yaml:"envrc_suffix,omitempty"`
}
