package seed

type Spec struct {
	BasePath string `yaml:"base_path"`
	Sets     []Set  `yaml:"sets"`
}

type Set struct {
	Name            string                       `yaml:"name,omitempty"`
	Path            string                       `yaml:"path"`
	Data            map[string]string            `yaml:"data"`
	Env             map[string]string            `yaml:"env"`
	Files           map[string]string            `yaml:"files"`
	Commands        map[string]string            `yaml:"commands"`
	SetupCommands   map[string]string            `yaml:"setup_commands"`
	CleanupCommands []string                     `yaml:"cleanup_commands"`
	JsonFiles       map[string]JsonFileTransform `yaml:"json_files"`
	YamlFiles       map[string]YamlFileTransform `yaml:"yaml_files"`
}

// JsonFileTransform defines how to extract and transform data from JSON files
type JsonFileTransform struct {
	File       string            `yaml:"file"`
	Transforms map[string]string `yaml:"transforms"`
}

// YamlFileTransform defines how to extract and transform data from YAML files
type YamlFileTransform struct {
	File       string            `yaml:"file"`
	Transforms map[string]string `yaml:"transforms"`
}
