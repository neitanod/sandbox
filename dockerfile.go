package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// buildImageIfNeeded builds a custom Docker image if setup commands exist.
// Returns the image name to use (custom or base).
func buildImageIfNeeded(name string, cfg *SandboxConfig) string {
	if len(cfg.Setup) == 0 {
		return cfg.Image
	}

	customImage := CustomImageName(name)
	tmpDir := filepath.Join(os.TempDir(), "sandbox-"+name)

	// Create temp dir
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		fatal("cannot create temp dir %s: %v", tmpDir, err)
	}
	defer os.RemoveAll(tmpDir)

	// Generate Dockerfile
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("FROM %s\n", cfg.Image))
	for _, cmd := range cfg.Setup {
		sb.WriteString(fmt.Sprintf("RUN %s\n", cmd))
	}

	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(sb.String()), 0644); err != nil {
		fatal("cannot write Dockerfile: %v", err)
	}

	// Build image
	fmt.Printf("building image %s...\n", customImage)
	buildCmd := exec.Command("docker", "build", "-t", customImage, tmpDir)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		fatal("docker build failed: %v", err)
	}

	return customImage
}
