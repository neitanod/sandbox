package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type MountConfig struct {
	Host      string `json:"host"`
	Container string `json:"container"`
}

type PortConfig struct {
	Host      int `json:"host"`
	Container int `json:"container"`
}

type SecurityConfig struct {
	NoRoot         bool `json:"no_root"`
	DropCaps       bool `json:"drop_caps"`
	ReadOnlyRootfs bool `json:"read_only_rootfs"`
	SeccompDefault bool `json:"seccomp_default"`
}

type SandboxConfig struct {
	Description string         `json:"description,omitempty"`
	Image       string         `json:"image,omitempty"`
	Setup       []string       `json:"setup,omitempty"`
	Mounts      []MountConfig  `json:"mounts"`
	Ports       []PortConfig   `json:"ports,omitempty"`
	Workdir     string         `json:"workdir,omitempty"`
	Network     *bool          `json:"network,omitempty"`
	Security    SecurityConfig `json:"security,omitempty"`
}

// ConfigDir returns the default config directory (~/.config/sandbox/)
func ConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		fatal("cannot determine home directory: %v", err)
	}
	return filepath.Join(home, ".config", "sandbox")
}

// DefaultConfigPath returns the path for a named config
func DefaultConfigPath(name string) string {
	return filepath.Join(ConfigDir(), name+".json")
}

// ContainerName returns the Docker container name for a sandbox
func ContainerName(name string) string {
	return "sandbox-" + name
}

// CustomImageName returns the Docker image name for a custom build
func CustomImageName(name string) string {
	return "sandbox-" + name + ":latest"
}

// SandboxNameFromContainer extracts the sandbox name from a container name
func SandboxNameFromContainer(containerName string) string {
	if len(containerName) > 8 && containerName[:8] == "sandbox-" {
		return containerName[8:]
	}
	return containerName
}

// LoadConfig loads a sandbox config from JSON file
func LoadConfig(name, configPath string) (*SandboxConfig, error) {
	path := configPath
	if path == "" {
		path = DefaultConfigPath(name)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg SandboxConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Apply defaults
	if cfg.Image == "" {
		cfg.Image = "alpine:latest"
	}
	if cfg.Network == nil {
		t := true
		cfg.Network = &t
	}
	if cfg.Workdir == "" && len(cfg.Mounts) > 0 {
		cfg.Workdir = cfg.Mounts[0].Container
	}

	return &cfg, nil
}

// SaveConfig writes a sandbox config to JSON file
func SaveConfig(name string, cfg *SandboxConfig) error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	path := DefaultConfigPath(name)
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

// NewDefaultConfig returns a config with default values
func NewDefaultConfig() *SandboxConfig {
	t := true
	return &SandboxConfig{
		Image:   "alpine:latest",
		Network: &t,
	}
}

// ListConfigNames returns the names of all configs in the config directory
func ListConfigNames() ([]string, error) {
	dir := ConfigDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if filepath.Ext(name) == ".json" {
			names = append(names, name[:len(name)-5])
		}
	}
	return names, nil
}
