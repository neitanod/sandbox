package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/sys/unix"
)

// isTTY checks if stdin is a terminal
func isTTY() bool {
	_, err := unix.IoctlGetTermios(int(os.Stdin.Fd()), unix.TCGETS)
	return err == nil
}

// containerExists checks if a Docker container with the given name exists
func containerExists(name string) bool {
	err := exec.Command("docker", "inspect", "--type=container", name).Run()
	return err == nil
}

// containerState returns the state of a Docker container (running, exited, etc.)
func containerState(name string) string {
	out, err := exec.Command("docker", "inspect",
		"--format", "{{.State.Status}}", name).Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

// imageExists checks if a Docker image exists
func imageExists(name string) bool {
	err := exec.Command("docker", "inspect", "--type=image", name).Run()
	return err == nil
}

// createContainer creates a new Docker container with the given config
func createContainer(containerName, imageName string, cfg *SandboxConfig) {
	args := []string{"create",
		"--name", containerName,
		"-it",
	}

	// Labels
	args = append(args, LabelFlags(cfg)...)

	// Mounts
	for _, m := range cfg.Mounts {
		args = append(args, "-v", fmt.Sprintf("%s:%s", m.Host, m.Container))
	}

	// Ports
	for _, p := range cfg.Ports {
		args = append(args, "-p", fmt.Sprintf("%d:%d", p.Host, p.Container))
	}

	// Workdir
	if cfg.Workdir != "" {
		args = append(args, "-w", cfg.Workdir)
	}

	// Network
	if cfg.Network != nil && !*cfg.Network {
		args = append(args, "--network", "none")
	}

	// Security
	args = append(args, SecurityFlags(cfg.Security)...)

	// Image
	args = append(args, imageName)

	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fatal("docker create failed: %v", err)
	}
}

// dockerStart starts a stopped Docker container
func dockerStart(name string) {
	cmd := exec.Command("docker", "start", name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fatal("docker start failed: %v", err)
	}
}

// dockerExec executes a command in a running container
func dockerExec(containerName string, cfg *SandboxConfig, userCmd []string) {
	args := []string{"exec"}
	if isTTY() {
		args = append(args, "-it")
	} else {
		args = append(args, "-i")
	}
	if cfg != nil && cfg.Workdir != "" {
		args = append(args, "-w", cfg.Workdir)
	}
	args = append(args, containerName)
	args = append(args, wrapShellCmd(userCmd)...)

	cmd := exec.Command("docker", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		// Don't fatal on exec errors, the command might have a non-zero exit
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fatal("docker exec failed: %v", err)
	}
}

// wrapShellCmd wraps user commands in sh -c when they contain shell syntax
// or are a single string that needs shell interpretation.
func wrapShellCmd(userCmd []string) []string {
	if len(userCmd) == 0 {
		return userCmd
	}
	joined := strings.Join(userCmd, " ")
	// If the command contains shell operators, wrap in sh -c.
	// Also wrap if it's a single arg containing whitespace (e.g. "php -v"),
	// which would otherwise be treated as a literal executable name.
	if needsShell(joined) || (len(userCmd) == 1 && strings.ContainsAny(userCmd[0], " \t")) {
		return []string{"sh", "-c", joined}
	}
	return userCmd
}

func needsShell(cmd string) bool {
	for _, ch := range []string{";", "&&", "||", "|", ">", "<", "$", "`", "(", ")"} {
		if strings.Contains(cmd, ch) {
			return true
		}
	}
	return false
}

// dockerExecInteractive opens an interactive shell in the container
func dockerExecInteractive(containerName string, cfg *SandboxConfig) {
	shell := "/bin/sh"
	dockerExec(containerName, cfg, []string{shell})
}

// runEphemeral runs a disposable container with --rm
func runEphemeral(name string, cfg *SandboxConfig, userCmd []string) {
	imageName := buildImageIfNeeded(name, cfg)

	args := []string{"run", "--rm"}
	if isTTY() {
		args = append(args, "-it")
	} else {
		args = append(args, "-i")
	}

	// Labels (for consistency)
	args = append(args, LabelFlags(cfg)...)

	// Mounts
	for _, m := range cfg.Mounts {
		args = append(args, "-v", fmt.Sprintf("%s:%s", m.Host, m.Container))
	}

	// Ports
	for _, p := range cfg.Ports {
		args = append(args, "-p", fmt.Sprintf("%d:%d", p.Host, p.Container))
	}

	// Workdir
	if cfg.Workdir != "" {
		args = append(args, "-w", cfg.Workdir)
	}

	// Network
	if cfg.Network != nil && !*cfg.Network {
		args = append(args, "--network", "none")
	}

	// Security
	args = append(args, SecurityFlags(cfg.Security)...)

	// Image
	args = append(args, imageName)

	// User command or shell
	if len(userCmd) > 0 {
		args = append(args, wrapShellCmd(userCmd)...)
	}

	cmd := exec.Command("docker", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fatal("docker run failed: %v", err)
	}
}

// dockerStop stops a running container
func dockerStop(name string) {
	cmd := exec.Command("docker", "stop", name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fatal("docker stop failed: %v", err)
	}
}

// dockerRmForce force removes a container
func dockerRmForce(name string) {
	cmd := exec.Command("docker", "rm", "-f", name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fatal("docker rm failed: %v", err)
	}
}

// dockerRmi removes a Docker image
func dockerRmi(name string) {
	cmd := exec.Command("docker", "rmi", name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		// Non-fatal: image might be in use
		fmt.Fprintf(os.Stderr, "warning: could not remove image %s: %v\n", name, err)
	}
}
