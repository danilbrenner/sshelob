package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct{}

func Load(path string) (*Config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
