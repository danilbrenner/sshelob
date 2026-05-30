package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// TunnelType represents the SSH port-forwarding mode.
type TunnelType string

const (
	TunnelTypeLocal   TunnelType = "local"
	TunnelTypeRemote  TunnelType = "remote"
	TunnelTypeDynamic TunnelType = "dynamic"
)

// HealthCheckDef defines an optional TCP probe for a tunnel.
type HealthCheckDef struct {
	// Interval between probes, e.g. "10s".
	Interval string `yaml:"interval"`
	// Timeout for each probe attempt, e.g. "3s".
	Timeout string `yaml:"timeout"`
}

type ConnectionDef struct {
	Name          string `yaml:"name"`
	Host          string `yaml:"host"`
	User          string `yaml:"user"`
	Port          int    `yaml:"port"`
	KeyPath       string `yaml:"key_path"`
	UsePassphrase bool   `yaml:"use_passphrase"`
}

type TunnelDef struct {
	Type        TunnelType      `yaml:"type"`
	Name        string          `yaml:"name"`
	Connection  string          `yaml:"connection"`
	BindAddr    string          `yaml:"bind_addr"`
	DestAddr    string          `yaml:"dest_addr"`
	HealthCheck *HealthCheckDef `yaml:"health_check,omitempty"`
}

type Config struct {
	Connections []ConnectionDef `yaml:"connections"`
	Tunnels     []TunnelDef     `yaml:"tunnels"`
}

const fallbackConfigRelativePath = ".config/sshelob/config.yml"

// Load reads, parses, and validates the YAML config file.
// If path is empty, it falls back to ~/.config/sshelob/config.yml.
// If path is set but missing, it then tries the fallback path.
func Load(path string) (*Config, error) {

	if path == "" {
		var err error
		path, err = defaultConfigPath()
		if err != nil {
			return nil, fmt.Errorf("config: determine default path: %w", err)
		}
	}

	return loadFromPath(path)
}

func loadFromPath(path string) (*Config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: cannot read file %q: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return nil, fmt.Errorf("config: cannot parse YAML in %q: %w", path, err)
	}

	if err := validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// validate checks all required fields in every tunnel definition.
func validate(cfg *Config) error {
	if len(cfg.Connections) == 0 {
		return fmt.Errorf("config: connections: must define at least one connection")
	}
	if len(cfg.Tunnels) == 0 {
		return fmt.Errorf("config: tunnels: must define at least one tunnel")
	}

	connectionsByName := make(map[string]ConnectionDef, len(cfg.Connections))
	for index, connectionDef := range cfg.Connections {
		prefix := fmt.Sprintf("config: connections[%d]", index)

		if connectionDef.Name == "" {
			return fmt.Errorf("%s.name: required field is missing", prefix)
		}
		if connectionDef.Host == "" {
			return fmt.Errorf("%s.host: required field is missing", prefix)
		}
		if connectionDef.User == "" {
			return fmt.Errorf("%s.user: required field is missing", prefix)
		}
		if connectionDef.Port == 0 {
			return fmt.Errorf("%s.port: required field is missing or zero", prefix)
		}
		if connectionDef.KeyPath == "" {
			return fmt.Errorf("%s.key_path: required field is missing", prefix)
		}
		if _, exists := connectionsByName[connectionDef.Name]; exists {
			return fmt.Errorf("%s.name: duplicate connection name %q", prefix, connectionDef.Name)
		}

		connectionsByName[connectionDef.Name] = connectionDef
	}

	for index, tunnelDef := range cfg.Tunnels {
		prefix := fmt.Sprintf("config: tunnels[%d]", index)

		if tunnelDef.Name == "" {
			return fmt.Errorf("%s.name: required field is missing", prefix)
		}

		switch tunnelDef.Type {
		case TunnelTypeLocal, TunnelTypeRemote, TunnelTypeDynamic:
		case "":
			return fmt.Errorf("%s.type: required field is missing", prefix)
		default:
			return fmt.Errorf("%s.type: unknown tunnel type %q (must be local, remote, or dynamic)", prefix, tunnelDef.Type)
		}

		if tunnelDef.Connection == "" {
			return fmt.Errorf("%s.connection: required field is missing", prefix)
		}
		if _, exists := connectionsByName[tunnelDef.Connection]; !exists {
			return fmt.Errorf("%s.connection: unknown connection %q", prefix, tunnelDef.Connection)
		}
		if tunnelDef.BindAddr == "" {
			return fmt.Errorf("%s.bind_addr: required field is missing", prefix)
		}

		if tunnelDef.Type == TunnelTypeLocal || tunnelDef.Type == TunnelTypeRemote {
			if tunnelDef.DestAddr == "" {
				return fmt.Errorf("%s.dest_addr: required for tunnel type %q", prefix, tunnelDef.Type)
			}
		}
		if tunnelDef.Type == TunnelTypeDynamic && tunnelDef.DestAddr != "" {
			return fmt.Errorf("%s.dest_addr: must not be set for tunnel type %q", prefix, tunnelDef.Type)
		}
	}

	return nil
}

func defaultConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDir, fallbackConfigRelativePath), nil
}

func (c *Config) ConnectionByName(name string) (ConnectionDef, bool) {
	for _, connectionDef := range c.Connections {
		if connectionDef.Name == name {
			return connectionDef, true
		}
	}

	return ConnectionDef{}, false
}
