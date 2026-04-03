package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	LabelManaged     = "sandbox.managed"
	LabelCreatedAt   = "sandbox.created-at"
	LabelDescription = "sandbox.description"
)

// LabelFlags returns Docker CLI --label flags for container creation
func LabelFlags(cfg *SandboxConfig) []string {
	now := time.Now().Format("2006-01-02")
	desc := cfg.Description

	return []string{
		"--label", fmt.Sprintf("%s=true", LabelManaged),
		"--label", fmt.Sprintf("%s=%s", LabelCreatedAt, now),
		"--label", fmt.Sprintf("%s=%s", LabelDescription, desc),
	}
}

// ContainerLabels reads labels from an existing Docker container
type ContainerLabels struct {
	Managed     bool
	CreatedAt   string
	Description string
}

// ReadContainerLabels reads labels from a Docker container via docker inspect
func ReadContainerLabels(containerName string) (*ContainerLabels, error) {
	out, err := exec.Command("docker", "inspect",
		"--format", "{{json .Config.Labels}}", containerName).Output()
	if err != nil {
		return nil, fmt.Errorf("docker inspect: %w", err)
	}

	var labels map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(out))), &labels); err != nil {
		return nil, fmt.Errorf("parse labels: %w", err)
	}

	return &ContainerLabels{
		Managed:     labels[LabelManaged] == "true",
		CreatedAt:   labels[LabelCreatedAt],
		Description: labels[LabelDescription],
	}, nil
}

// ManagedContainerInfo holds info about a managed Docker container
type ManagedContainerInfo struct {
	ContainerName string
	SandboxName   string
	Image         string
	State         string
	Description   string
	CreatedAt     string
}

// ListManagedContainers returns all Docker containers with the sandbox.managed label
func ListManagedContainers() ([]ManagedContainerInfo, error) {
	out, err := exec.Command("docker", "ps", "-a",
		"--filter", fmt.Sprintf("label=%s=true", LabelManaged),
		"--format", "{{.Names}}\t{{.Image}}\t{{.State}}").Output()
	if err != nil {
		return nil, fmt.Errorf("docker ps: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, nil
	}

	var containers []ManagedContainerInfo
	for _, line := range lines {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}

		name := parts[0]
		info := ManagedContainerInfo{
			ContainerName: name,
			SandboxName:   SandboxNameFromContainer(name),
			Image:         parts[1],
			State:         parts[2],
		}

		// Read labels for description and created-at
		labels, err := ReadContainerLabels(name)
		if err == nil {
			info.Description = labels.Description
			info.CreatedAt = labels.CreatedAt
		}

		containers = append(containers, info)
	}

	return containers, nil
}
