package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func cmdEdit(name string) {
	path := DefaultConfigPath(name)
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		fatal("config not found for %q. Create it with: sandbox %s --build", name, name)
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vi"
	}

	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fatal("editor failed: %v", err)
	}
}

func cmdConfig(args []string) {
	// args[0] = name, args[1:] = key, value, etc.
	name := args[0]
	rest := args[1:]

	// Load existing config or create new one
	cfg, err := LoadConfig(name, "")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg = NewDefaultConfig()
		} else {
			fatal("cannot load config for %q: %v", name, err)
		}
	}

	// No key: show full config
	if len(rest) == 0 {
		data, _ := json.MarshalIndent(cfg, "", "  ")
		fmt.Println(string(data))
		return
	}

	key := rest[0]

	// Key but no value: show current value
	if len(rest) == 1 {
		showConfigValue(cfg, key)
		return
	}

	// Array operations with + or -
	if len(rest) >= 3 && (rest[1] == "+" || rest[1] == "-") {
		op := rest[1]
		value := strings.Join(rest[2:], " ")
		arrayOp(cfg, key, op, value)
		if err := SaveConfig(name, cfg); err != nil {
			fatal("cannot save config: %v", err)
		}
		return
	}

	// Simple set
	value := strings.Join(rest[1:], " ")
	setConfigValue(cfg, key, value)
	if err := SaveConfig(name, cfg); err != nil {
		fatal("cannot save config: %v", err)
	}
}

func showConfigValue(cfg *SandboxConfig, key string) {
	switch key {
	case "description":
		fmt.Println(cfg.Description)
	case "image":
		fmt.Println(cfg.Image)
	case "workdir":
		fmt.Println(cfg.Workdir)
	case "network":
		if cfg.Network != nil {
			fmt.Println(*cfg.Network)
		} else {
			fmt.Println("true")
		}
	case "setup":
		for _, s := range cfg.Setup {
			fmt.Println(s)
		}
	case "mounts":
		for _, m := range cfg.Mounts {
			fmt.Printf("%s:%s\n", m.Host, m.Container)
		}
	case "ports":
		for _, p := range cfg.Ports {
			fmt.Printf("%d:%d\n", p.Host, p.Container)
		}
	case "security.no_root":
		fmt.Println(cfg.Security.NoRoot)
	case "security.drop_caps":
		fmt.Println(cfg.Security.DropCaps)
	case "security.read_only_rootfs":
		fmt.Println(cfg.Security.ReadOnlyRootfs)
	case "security.seccomp_default":
		fmt.Println(cfg.Security.SeccompDefault)
	default:
		fatal("unknown config key: %s", key)
	}
}

func setConfigValue(cfg *SandboxConfig, key, value string) {
	switch key {
	case "description":
		cfg.Description = value
	case "image":
		cfg.Image = value
	case "workdir":
		cfg.Workdir = value
	case "network":
		b := parseBool(value)
		cfg.Network = &b
	case "security.no_root":
		cfg.Security.NoRoot = parseBool(value)
	case "security.drop_caps":
		cfg.Security.DropCaps = parseBool(value)
	case "security.read_only_rootfs":
		cfg.Security.ReadOnlyRootfs = parseBool(value)
	case "security.seccomp_default":
		cfg.Security.SeccompDefault = parseBool(value)
	default:
		fatal("unknown config key: %s (use + or - for array fields like mounts, ports, setup)", key)
	}
	fmt.Printf("%s = %s\n", key, value)
}

func arrayOp(cfg *SandboxConfig, key, op, value string) {
	switch key {
	case "mounts":
		mount := parseMount(value)
		if op == "+" {
			cfg.Mounts = append(cfg.Mounts, mount)
			fmt.Printf("mounts += %s:%s\n", mount.Host, mount.Container)
		} else {
			cfg.Mounts = removeMountFunc(cfg.Mounts, mount)
			fmt.Printf("mounts -= %s:%s\n", mount.Host, mount.Container)
		}
	case "ports":
		port := parsePort(value)
		if op == "+" {
			cfg.Ports = append(cfg.Ports, port)
			fmt.Printf("ports += %d:%d\n", port.Host, port.Container)
		} else {
			cfg.Ports = removePortFunc(cfg.Ports, port)
			fmt.Printf("ports -= %d:%d\n", port.Host, port.Container)
		}
	case "setup":
		if op == "+" {
			cfg.Setup = append(cfg.Setup, value)
			fmt.Printf("setup += %q\n", value)
		} else {
			cfg.Setup = removeStringFunc(cfg.Setup, value)
			fmt.Printf("setup -= %q\n", value)
		}
	default:
		fatal("key %q does not support + or - operations (only mounts, ports, setup)", key)
	}
}

func parseMount(s string) MountConfig {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		fatal("invalid mount format %q (expected host_path:container_path)", s)
	}
	return MountConfig{Host: parts[0], Container: parts[1]}
}

func parsePort(s string) PortConfig {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		fatal("invalid port format %q (expected host_port:container_port)", s)
	}
	host, err := strconv.Atoi(parts[0])
	if err != nil {
		fatal("invalid host port %q: %v", parts[0], err)
	}
	container, err := strconv.Atoi(parts[1])
	if err != nil {
		fatal("invalid container port %q: %v", parts[1], err)
	}
	return PortConfig{Host: host, Container: container}
}

func parseBool(s string) bool {
	switch strings.ToLower(s) {
	case "true", "1", "yes":
		return true
	case "false", "0", "no":
		return false
	default:
		fatal("invalid boolean value %q (expected true/false)", s)
		return false
	}
}

func removeMountFunc(mounts []MountConfig, target MountConfig) []MountConfig {
	var result []MountConfig
	for _, m := range mounts {
		if m.Host != target.Host || m.Container != target.Container {
			result = append(result, m)
		}
	}
	return result
}

func removePortFunc(ports []PortConfig, target PortConfig) []PortConfig {
	var result []PortConfig
	for _, p := range ports {
		if p.Host != target.Host || p.Container != target.Container {
			result = append(result, p)
		}
	}
	return result
}

func removeStringFunc(items []string, target string) []string {
	var result []string
	for _, s := range items {
		if s != target {
			result = append(result, s)
		}
	}
	return result
}

