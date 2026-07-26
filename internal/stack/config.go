package stack

// DefaultConfigPath is the default location for the stack configuration file.
const DefaultConfigPath = "~/.config/overlord/stack.yaml"

// StackConfig represents the complete stack configuration for overlord start.
type StackConfig struct {
	Version  int                      `yaml:"version"`
	Services map[string]ServiceConfig `yaml:"services"`
}

// ServiceConfig represents configuration for a single service in the stack.
type ServiceConfig struct {
	Enabled  bool              `yaml:"enabled"`
	Settings map[string]string `yaml:"settings,omitempty"`
}
