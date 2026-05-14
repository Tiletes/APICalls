package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds the full application configuration.
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Session  SessionConfig  `yaml:"session"`
	Log      LogConfig      `yaml:"log"`
}

type ServerConfig struct {
	Port    int    `yaml:"port"`
	Host    string `yaml:"host"`
	TLSCert string `yaml:"tls_cert"`
	TLSKey  string `yaml:"tls_key"`
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
}

type SessionConfig struct {
	Secret string `yaml:"secret"`
	Name   string `yaml:"name"`
	MaxAge int    `yaml:"max_age"`
}

type LogConfig struct {
	Path     string `yaml:"path"`
	TechPath string `yaml:"tech_path"`
}

// Load reads the YAML config file and returns a Config.
func Load(path string) (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port:    8443,
			Host:    "0.0.0.0",
			TLSCert: "certs/server.crt",
			TLSKey:  "certs/server.key",
		},
		Database: DatabaseConfig{Path: "data/apicalls.db"},
		Session: SessionConfig{
			Secret: "change-me-to-a-random-secret-key-32bytes",
			Name:   "apicalls_session",
			MaxAge: 86400,
		},
		Log: LogConfig{
			Path:     "logs/apicalls.log",
			TechPath: "logs/technical.log",
		},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
